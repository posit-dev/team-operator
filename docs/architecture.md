# Team Operator Architecture

This document provides detailed architecture diagrams and explanations for the Team Operator and its managed products.

## Table of Contents

- [System Overview](#system-overview)
- [Database Architecture](#database-architecture)
- [Connect Architecture](#connect-architecture)
- [Workbench Architecture](#workbench-architecture)
- [Package Manager Architecture](#package-manager-architecture)
- [Flightdeck Architecture](#flightdeck-architecture)
- [Chronicle Architecture](#chronicle-architecture)

---

## System Overview

The Team Operator follows the Kubernetes operator pattern: a Site Custom Resource (CR) serves as the single source of truth, and controllers reconcile the desired state into running Kubernetes resources.

```
User creates Site CR
        ↓
Site Controller reconciles
        ↓
Product CRs created (Connect, Workbench, PackageManager, etc.)
        ↓
Product Controllers reconcile
        ↓
Kubernetes resources created (Deployments, Services, Ingress, etc.)
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Site CR** | The top-level resource that defines an entire Posit Team deployment |
| **Product CR** | Child resources (Connect, Workbench, PackageManager) created by the Site controller |
| **Controller** | Watches resources and reconciles them to the desired state |
| **Reconciliation** | The process of comparing desired state (CR spec) with actual state and making corrections |

---

## Database Architecture

Each Posit Team product requires database storage. The operator provisions separate databases with dedicated users and schemas.

```mermaid
flowchart TB
    subgraph db [Team Operator - Databases]
        subgraph pub[PublishDB - Connect]
            pub-user(Connect User)
            pub-main[Main Schema]
            pub-metrics[Instrumentation Schema]
        end
        pub-user-->pub-main
        pub-user-->pub-metrics

        subgraph pkg[PackageDB - Package Manager]
            pkg-user(Package Manager User)
            pkg-main[Main Schema]
            pkg-metrics[Metrics Schema]
        end
        pkg-user-->pkg-main
        pkg-user-->pkg-metrics

        subgraph dev[DevDB - Workbench]
            dev-user(Workbench User)
            dev-main[Public Schema]
        end
        dev-user-->dev-main
    end

    classDef userNode fill:#FAEEE9,stroke:#ab4d26
    class pub-user,pkg-user,dev-user userNode
```

### Component Descriptions

| Component | Description |
|-----------|-------------|
| **PublishDB** | PostgreSQL database for Connect. Stores published content metadata, user accounts, and access controls. |
| **Main Schema** | Primary data storage for the product (content, users, permissions) |
| **Instrumentation Schema** | Metrics and usage tracking data (Connect and Package Manager only) |
| **PackageDB** | PostgreSQL database for Package Manager. Stores package metadata, repository configurations, and sync state. |
| **Metrics Schema** | Analytics data for package downloads and repository usage |
| **DevDB** | PostgreSQL database for Workbench. Stores user sessions, project metadata, and launcher state. |
| **Public Schema** | Workbench uses a single schema for all data |

### Database User Isolation

Each product gets a dedicated database user with access only to its own schemas. This provides:
- **Security isolation**: Products cannot access each other's data
- **Resource tracking**: Database connections can be attributed to specific products
- **Independent credentials**: Rotating one product's credentials doesn't affect others

---

## Connect Architecture

Posit Connect is a publishing platform for data science content. The operator manages its deployment including off-host content execution.

```mermaid
flowchart TB
    subgraph external [External Configuration]
        manual(Manual Setup)
        license(License)
        clientsecret(Auth Client Secret)
        mainDbCon(Main DB Connection)
    end

    subgraph operator [Team Operator]
        site(Site Controller)
        dbcon(Database Controller)
        connect(Connect Controller)
    end

    subgraph k8s [Kubernetes Resources]
        subgraph storage [Storage]
            pv(PersistentVolume)
            pvc(PersistentVolumeClaim)
        end
        subgraph config [Configuration]
            cm(ConfigMaps)
            dbsecret(DB Password Secret)
            secretkey(Secret Key)
        end
        subgraph workload [Workload]
            pubdeploy(Connect Pod)
            ing(Ingress)
            svc(Service)
        end
    end

    %% External to Operator
    manual --> license
    manual --> clientsecret
    manual --> mainDbCon
    mainDbCon --> dbcon

    %% Operator flow
    site --> pv
    site --> connect
    site --> dbcon
    dbcon --> dbsecret

    %% Connect Controller creates resources
    connect --> pvc
    connect --> cm
    connect --> secretkey
    connect --> pubdeploy
    connect --> ing
    connect --> svc

    %% Resources flow to Pod
    pv --> pvc
    pvc --> pubdeploy
    cm --> pubdeploy
    dbsecret --> pubdeploy
    secretkey --> pubdeploy
    license --> pubdeploy
    clientsecret --> pubdeploy

    classDef external fill:#FAEEE9,stroke:#ab4d26
    classDef operator fill:#E3F2FD,stroke:#1976D2
    classDef k8s fill:#E8F5E9,stroke:#388E3C

    class manual,license,clientsecret,mainDbCon external
    class site,dbcon,connect operator
    class pv,pvc,cm,dbsecret,secretkey,pubdeploy,ing,svc k8s
```

### Component Descriptions

#### External Configuration (Coral)

| Component | Description |
|-----------|-------------|
| **Manual Setup** | One-time configuration performed by the administrator before deployment |
| **License** | Posit Connect license file or activation key, stored in a Kubernetes Secret or AWS Secrets Manager |
| **Auth Client Secret** | OIDC/SAML client credentials for SSO integration (client ID and secret from your IdP) |
| **Main DB Connection** | PostgreSQL connection string for the external database server |

#### Team Operator (Blue)

| Component | Description |
|-----------|-------------|
| **Site Controller** | Watches Site CRs and creates product-specific CRs (Connect, Workbench, etc.). Manages shared resources like PersistentVolumes. |
| **Database Controller** | Creates databases and schemas within the PostgreSQL server. Generates credentials and stores them in Secrets. |
| **Connect Controller** | Watches Connect CRs and creates all Kubernetes resources needed to run Connect. |

#### Kubernetes Resources (Green)

| Component | Description |
|-----------|-------------|
| **PersistentVolume (PV)** | Cluster-level storage resource representing physical storage (NFS, FSx, Azure NetApp) |
| **PersistentVolumeClaim (PVC)** | Namespace-scoped claim that binds to a PV. Mounted into the Connect pod for content storage. |
| **ConfigMaps** | Connect configuration files (`rstudio-connect.gcfg`) generated from the CR spec |
| **DB Password Secret** | Auto-generated database credentials created by the Database Controller |
| **Secret Key** | Encryption key for Connect's internal data encryption |
| **Connect Pod** | The main Connect server container running the publishing platform |
| **Ingress** | Routes external traffic to the Connect Service based on hostname |
| **Service** | Kubernetes Service providing stable networking for the Connect Pod |

### Off-Host Execution

When off-host execution is enabled, Connect runs content (Shiny apps, APIs, reports) in separate Kubernetes Jobs rather than in the main Connect pod. This provides:
- **Resource isolation**: Content processes don't compete with the Connect server
- **Scalability**: Content can scale independently
- **Security**: Content runs with minimal privileges

See the [Connect Configuration Guide](guides/connect-configuration.md) for details.

---

## Workbench Architecture

Posit Workbench provides IDE environments (RStudio, VS Code, Jupyter) for data scientists. The operator manages both the main server and user session pods.

```mermaid
flowchart TB
    subgraph external [External Configuration]
        manual(Manual Setup)
        license(License)
        clientsecret(Auth Client Secret)
        mainDbCon(Main DB Connection)
    end

    subgraph operator [Team Operator]
        site(Site Controller)
        dbcon(Database Controller)
        workbench(Workbench Controller)
    end

    subgraph k8s [Kubernetes Resources]
        subgraph storage [Storage]
            pv(PersistentVolume)
            pvc(PersistentVolumeClaim)
            homepvc(Home Directory PVC)
        end
        subgraph config [Configuration]
            cm(ConfigMaps)
            dbsecret(DB Password Secret)
            secretkey(Secret Key)
            jobtpl(Job Templates)
        end
        subgraph workload [Workload]
            wbdeploy(Workbench Pod)
            ing(Ingress)
            svc(Service)
        end
    end

    subgraph sessions [Session Infrastructure]
        launcher(Job Launcher)
        sessionpod1(Session Pod)
        sessionpod2(Session Pod)
    end

    %% External to Operator
    manual --> license
    manual --> clientsecret
    manual --> mainDbCon
    mainDbCon --> dbcon

    %% Operator flow
    site --> pv
    site --> workbench
    site --> dbcon
    dbcon --> dbsecret

    %% Workbench Controller creates resources
    workbench --> pvc
    workbench --> homepvc
    workbench --> cm
    workbench --> secretkey
    workbench --> jobtpl
    workbench --> wbdeploy
    workbench --> ing
    workbench --> svc

    %% Resources flow to Pod
    pv --> pvc
    pvc --> wbdeploy
    homepvc --> wbdeploy
    cm --> wbdeploy
    dbsecret --> wbdeploy
    secretkey --> wbdeploy
    license --> wbdeploy
    clientsecret --> wbdeploy
    jobtpl --> wbdeploy

    %% Session management
    wbdeploy --> launcher
    launcher --> sessionpod1
    launcher --> sessionpod2
    homepvc --> sessionpod1
    homepvc --> sessionpod2
    pvc --> sessionpod1
    pvc --> sessionpod2

    classDef external fill:#FAEEE9,stroke:#ab4d26
    classDef operator fill:#E3F2FD,stroke:#1976D2
    classDef k8s fill:#E8F5E9,stroke:#388E3C
    classDef session fill:#FFF3E0,stroke:#F57C00

    class manual,license,clientsecret,mainDbCon external
    class site,dbcon,workbench operator
    class pv,pvc,homepvc,cm,dbsecret,secretkey,jobtpl,wbdeploy,ing,svc k8s
    class launcher,sessionpod1,sessionpod2 session
```

### Component Descriptions

#### External Configuration (Coral)

Same as Connect - see [Connect Architecture](#component-descriptions) above.

#### Team Operator (Blue)

| Component | Description |
|-----------|-------------|
| **Site Controller** | Creates the Workbench CR and manages shared storage volumes |
| **Database Controller** | Provisions the Workbench database (DevDB) for session and project metadata |
| **Workbench Controller** | Creates all Kubernetes resources for Workbench including session templates |

#### Kubernetes Resources (Green)

| Component | Description |
|-----------|-------------|
| **PersistentVolume / PVC** | Shared project storage accessible by both the server and all session pods |
| **Home Directory PVC** | User home directories, mounted into session pods at `/home/{username}` |
| **ConfigMaps** | Workbench configuration files including `rserver.conf`, `launcher.conf`, and IDE settings |
| **Job Templates** | Kubernetes Job/Service templates used by the Launcher to create session pods |
| **Workbench Pod** | The main Workbench server handling authentication, the web UI, and session management |
| **Ingress / Service** | Network routing for external access to Workbench |

#### Session Infrastructure (Orange)

| Component | Description |
|-----------|-------------|
| **Job Launcher** | Component within Workbench that creates Kubernetes Jobs for user sessions |
| **Session Pod** | Individual IDE sessions (RStudio, VS Code, Jupyter) running as Kubernetes Jobs. Each user session gets its own pod with dedicated resources. |

### Session Lifecycle

1. User logs into Workbench and requests a new session
2. Job Launcher creates a Kubernetes Job using the configured template
3. Session Pod starts with the selected IDE and mounts user's home directory
4. User works in the session; all files are saved to persistent storage
5. When the session ends, the Job completes and the Pod is cleaned up
6. User's work persists in the Home Directory PVC for the next session

### Storage Architecture

Workbench requires careful storage planning:

| Storage | Purpose | Access Mode |
|---------|---------|-------------|
| **Home Directory PVC** | User home directories with personal files and settings | ReadWriteMany (multiple sessions) |
| **Shared Storage PVC** | Shared project data accessible by all users | ReadWriteMany |
| **Session Scratch** | Temporary storage for session runtime (optional) | ReadWriteOnce per session |

See the [Workbench Configuration Guide](guides/workbench-configuration.md) for details.

---

## Package Manager Architecture

Posit Package Manager provides a local repository for R and Python packages. It can mirror public repositories and host private packages.

```mermaid
flowchart TB
    subgraph external [External Configuration]
        manual(Manual Setup)
        license(License)
        clientsecret(Auth Client Secret)
        mainDbCon(Main DB Connection)
        sshkeys(Git SSH Keys)
    end

    subgraph cloudstorage [Cloud Storage]
        s3(S3 Bucket)
        azfiles(Azure Files)
    end

    subgraph operator [Team Operator]
        site(Site Controller)
        dbcon(Database Controller)
        pm(PackageManager Controller)
    end

    subgraph k8s [Kubernetes Resources]
        subgraph storage [Storage]
            pv(PersistentVolume)
            pvc(PersistentVolumeClaim)
        end
        subgraph config [Configuration]
            cm(ConfigMaps)
            dbsecret(DB Password Secret)
            secretkey(Secret Key)
            sshsecret(SSH Key Secret)
        end
        subgraph workload [Workload]
            pmdeploy(Package Manager Pod)
            ing(Ingress)
            svc(Service)
        end
    end

    %% External to Operator
    manual --> license
    manual --> clientsecret
    manual --> mainDbCon
    manual --> sshkeys
    mainDbCon --> dbcon
    sshkeys --> sshsecret

    %% Operator flow
    site --> pv
    site --> pm
    site --> dbcon
    dbcon --> dbsecret

    %% PackageManager Controller creates resources
    pm --> pvc
    pm --> cm
    pm --> secretkey
    pm --> pmdeploy
    pm --> ing
    pm --> svc

    %% Resources flow to Pod
    pv --> pvc
    pvc --> pmdeploy
    cm --> pmdeploy
    dbsecret --> pmdeploy
    secretkey --> pmdeploy
    license --> pmdeploy
    clientsecret --> pmdeploy
    sshsecret --> pmdeploy

    %% Cloud storage connections
    pmdeploy --> s3
    pmdeploy --> azfiles

    classDef external fill:#FAEEE9,stroke:#ab4d26
    classDef operator fill:#E3F2FD,stroke:#1976D2
    classDef k8s fill:#E8F5E9,stroke:#388E3C
    classDef cloud fill:#E1F5FE,stroke:#0288D1

    class manual,license,clientsecret,mainDbCon,sshkeys external
    class site,dbcon,pm operator
    class pv,pvc,cm,dbsecret,secretkey,sshsecret,pmdeploy,ing,svc k8s
    class s3,azfiles cloud
```

### Component Descriptions

#### External Configuration (Coral)

| Component | Description |
|-----------|-------------|
| **Manual Setup** | One-time configuration by the administrator |
| **License** | Posit Package Manager license |
| **Auth Client Secret** | OIDC/SAML credentials for SSO |
| **Main DB Connection** | PostgreSQL connection for package metadata |
| **Git SSH Keys** | SSH keys for accessing private Git repositories when building packages from source |

#### Cloud Storage (Light Blue)

| Component | Description |
|-----------|-------------|
| **S3 Bucket** | AWS S3 storage for package binaries (recommended for AWS deployments) |
| **Azure Files** | Azure file storage for package binaries (recommended for Azure deployments) |

Package Manager can use either cloud storage backend. The choice typically depends on your cloud provider:
- **AWS**: Use S3 for best performance and cost
- **Azure**: Use Azure Files with the CSI driver
- **On-premises**: Use the local PVC for package storage

#### Team Operator (Blue)

| Component | Description |
|-----------|-------------|
| **Site Controller** | Creates the PackageManager CR |
| **Database Controller** | Provisions the Package Manager database with main and metrics schemas |
| **PackageManager Controller** | Creates all Kubernetes resources for Package Manager |

#### Kubernetes Resources (Green)

| Component | Description |
|-----------|-------------|
| **PersistentVolume / PVC** | Local storage for temporary files and cache (when not using cloud storage) |
| **ConfigMaps** | Package Manager configuration (`rstudio-pm.gcfg`) |
| **SSH Key Secret** | Mounted SSH keys for Git authentication during package builds |
| **Package Manager Pod** | The main server handling package requests, sync operations, and builds |
| **Ingress / Service** | Network routing for package installation requests |

### Package Storage Options

| Option | Best For | Configuration |
|--------|----------|---------------|
| **S3** | AWS deployments, large repositories | `spec.packageManager.s3Bucket` |
| **Azure Files** | Azure deployments | `spec.packageManager.azureFiles` |
| **Local PVC** | Development, small deployments | Default when no cloud storage configured |

### Git Builder Integration

Package Manager can build R packages from Git repositories. This requires:

1. **SSH Keys**: Private keys with access to your Git repositories
2. **Known Hosts**: SSH host key verification (optional but recommended)
3. **Build Resources**: CPU/memory for compilation

See the [Package Manager Configuration Guide](guides/packagemanager-configuration.md) for details.

---

## Flightdeck Architecture

Flightdeck is the landing page and navigation hub for Posit Team deployments. It provides a simple dashboard for users to access the various products.

```mermaid
flowchart TB
    subgraph operator [Team Operator]
        site(Site Controller)
        flightdeck_ctrl(Flightdeck Controller)
    end

    subgraph k8s [Kubernetes Resources]
        subgraph config [Configuration]
            cm(ConfigMap)
        end
        subgraph workload [Workload]
            fddeploy(Flightdeck Pod)
            ing(Ingress)
            svc(Service)
        end
    end

    subgraph products [Product Endpoints]
        wb_ing(Workbench Ingress)
        conn_ing(Connect Ingress)
        pm_ing(Package Manager Ingress)
    end

    subgraph users [Users]
        browser(Web Browser)
    end

    %% Operator flow
    site --> flightdeck_ctrl
    flightdeck_ctrl --> cm
    flightdeck_ctrl --> fddeploy
    flightdeck_ctrl --> ing
    flightdeck_ctrl --> svc

    %% Config to Pod
    cm --> fddeploy

    %% User access
    browser --> ing
    ing --> svc
    svc --> fddeploy

    %% Navigation to products
    fddeploy -.-> wb_ing
    fddeploy -.-> conn_ing
    fddeploy -.-> pm_ing

    classDef operator fill:#E3F2FD,stroke:#1976D2
    classDef k8s fill:#E8F5E9,stroke:#388E3C
    classDef product fill:#FFF3E0,stroke:#F57C00
    classDef user fill:#F3E5F5,stroke:#7B1FA2

    class site,flightdeck_ctrl operator
    class cm,fddeploy,ing,svc k8s
    class wb_ing,conn_ing,pm_ing product
    class browser user
```

### Component Descriptions

#### Team Operator (Blue)

| Component | Description |
|-----------|-------------|
| **Site Controller** | Creates the Flightdeck CR when Flightdeck is enabled in the Site spec |
| **Flightdeck Controller** | Creates all Kubernetes resources needed to run the landing page |

#### Kubernetes Resources (Green)

| Component | Description |
|-----------|-------------|
| **ConfigMap** | Configuration for Flightdeck including enabled features and product URLs |
| **Flightdeck Pod** | Static web server serving the landing page HTML/CSS/JS |
| **Ingress** | Routes traffic from the base domain to Flightdeck |
| **Service** | Kubernetes Service for the Flightdeck Pod |

#### Product Endpoints (Orange)

| Component | Description |
|-----------|-------------|
| **Workbench Ingress** | Flightdeck links to `workbench.{domain}` |
| **Connect Ingress** | Flightdeck links to `connect.{domain}` |
| **Package Manager Ingress** | Flightdeck links to `packagemanager.{domain}` |

### Features

Flightdeck is intentionally simple:

- **No database**: Serves static content only
- **No authentication**: Relies on product-level authentication
- **Configurable layout**: Shows only enabled products
- **Optional Academy**: Can display a fourth card for Posit Academy

### Configuration Options

| Option | Description |
|--------|-------------|
| `spec.flightdeck.replicas` | Number of replicas (default: 1) |
| `spec.flightdeck.featureEnabler.showConfig` | Show configuration page link |
| `spec.flightdeck.featureEnabler.showAcademy` | Show Academy product card |

---

## Chronicle Architecture

Chronicle is the telemetry and usage tracking service for Posit Team. It collects metrics from Connect and Workbench via sidecar containers.

```mermaid
flowchart TB
    subgraph operator [Team Operator]
        site(Site Controller)
        chronicle_ctrl(Chronicle Controller)
        connect_ctrl(Connect Controller)
        workbench_ctrl(Workbench Controller)
    end

    subgraph k8s [Kubernetes Resources]
        subgraph config [Configuration]
            cm(ConfigMap)
            apikey(API Key Secret)
        end
        subgraph workload [Chronicle Service]
            chronicledeploy(Chronicle Pod)
            svc(Service)
        end
    end

    subgraph products [Product Pods with Sidecars]
        subgraph connectpod [Connect Pod]
            connect_main(Connect Container)
            connect_sidecar(Chronicle Sidecar)
        end
        subgraph workbenchpod [Workbench Pod]
            wb_main(Workbench Container)
            wb_sidecar(Chronicle Sidecar)
        end
    end

    subgraph storage [Telemetry Storage]
        s3(S3 Bucket)
        local(Local Volume)
    end

    %% Operator flow
    site --> chronicle_ctrl
    site --> connect_ctrl
    site --> workbench_ctrl
    chronicle_ctrl --> cm
    chronicle_ctrl --> apikey
    chronicle_ctrl --> chronicledeploy
    chronicle_ctrl --> svc

    %% Sidecar injection
    connect_ctrl --> connect_sidecar
    workbench_ctrl --> wb_sidecar

    %% API key distribution
    apikey --> connect_sidecar
    apikey --> wb_sidecar

    %% Metrics flow
    connect_main -.->|metrics| connect_sidecar
    wb_main -.->|metrics| wb_sidecar
    connect_sidecar -->|send| chronicledeploy
    wb_sidecar -->|send| chronicledeploy

    %% Storage
    chronicledeploy --> s3
    chronicledeploy --> local

    classDef operator fill:#E3F2FD,stroke:#1976D2
    classDef k8s fill:#E8F5E9,stroke:#388E3C
    classDef product fill:#FFF3E0,stroke:#F57C00
    classDef storage fill:#E1F5FE,stroke:#0288D1
    classDef sidecar fill:#FFEBEE,stroke:#C62828

    class site,chronicle_ctrl,connect_ctrl,workbench_ctrl operator
    class cm,apikey,chronicledeploy,svc k8s
    class connect_main,wb_main product
    class connect_sidecar,wb_sidecar sidecar
    class s3,local storage
```

### Component Descriptions

#### Team Operator (Blue)

| Component | Description |
|-----------|-------------|
| **Site Controller** | Creates the Chronicle CR when Chronicle is enabled |
| **Chronicle Controller** | Creates the Chronicle service and manages API keys |
| **Connect Controller** | Injects Chronicle sidecar into Connect pods when enabled |
| **Workbench Controller** | Injects Chronicle sidecar into Workbench pods when enabled |

#### Kubernetes Resources (Green)

| Component | Description |
|-----------|-------------|
| **ConfigMap** | Chronicle server configuration |
| **API Key Secret** | Shared secret for sidecar authentication to the Chronicle service |
| **Chronicle Pod** | Central telemetry aggregation service |
| **Service** | Internal endpoint for sidecars to send metrics |

#### Product Pods (Orange/Red)

| Component | Description |
|-----------|-------------|
| **Connect/Workbench Container** | Main product container that generates usage metrics |
| **Chronicle Sidecar** | Lightweight agent that collects metrics from the main container and forwards them to the Chronicle service |

#### Telemetry Storage (Light Blue)

| Component | Description |
|-----------|-------------|
| **S3 Bucket** | Cloud storage for telemetry data (recommended for production) |
| **Local Volume** | Local storage option for development or air-gapped environments |

### Data Flow

1. **Metrics Generation**: Connect and Workbench generate usage metrics (content views, session starts, etc.)
2. **Sidecar Collection**: Chronicle sidecars collect metrics from the product containers
3. **Aggregation**: Sidecars send data to the central Chronicle service
4. **Storage**: Chronicle persists data to S3 or local storage
5. **Analysis**: Data can be queried for usage reports and analytics

### Sidecar Injection

The Chronicle sidecar is automatically injected into product pods when:
- Chronicle is enabled in the Site spec (`spec.chronicle.enabled: true`)
- The product has Chronicle integration enabled

The sidecar:
- Runs as a secondary container in the same pod
- Shares the pod's network namespace (can reach localhost)
- Uses the API key secret for authentication
- Has minimal resource requirements (~50Mi memory)

### Configuration Options

| Option | Description |
|--------|-------------|
| `spec.chronicle.enabled` | Enable Chronicle telemetry collection |
| `spec.chronicle.image` | Chronicle agent container image |
| `spec.chronicle.s3Bucket` | S3 bucket for telemetry storage |
| `spec.chronicle.localStorage` | Use local volume instead of S3 |

---

## Related Documentation

- [Site Management Guide](guides/product-team-site-management.md) - Managing Site CRs
- [Connect Configuration](guides/connect-configuration.md) - Detailed Connect setup
- [Workbench Configuration](guides/workbench-configuration.md) - Detailed Workbench setup
- [Package Manager Configuration](guides/packagemanager-configuration.md) - Detailed Package Manager setup
- [API Reference](api-reference.md) - Complete CRD field reference
