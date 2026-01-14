# Team Operator

The Team Operator is a Kubernetes operator that manages the deployment and configuration of Posit Team products within a Kubernetes cluster.

## Overview

The Team Operator automates the deployment and lifecycle management of:
- **Workbench** - Interactive development environment
- **Connect** - Publishing and sharing platform
- **Package Manager** - Package repository management
- **Chronicle** - Telemetry and monitoring
- **Keycloak** - Authentication and identity management

## Architecture

The operator uses a hierarchical configuration model:

```
Site CRD (single source of truth)
    ├── Connect configuration
    ├── Workbench configuration
    ├── Package Manager configuration
    ├── Chronicle configuration
    └── Keycloak configuration
```

The Site controller watches for Site resources and reconciles product-specific Custom Resources for each enabled product.

### Overall System Architecture

```mermaid
flowchart TB
    subgraph user [User Interface]
        kubectl(kubectl / Helm)
    end

    subgraph crd [Custom Resources]
        site[Site CRD]
        connect_cr[Connect CR]
        workbench_cr[Workbench CR]
        pm_cr[PackageManager CR]
        chronicle_cr[Chronicle CR]
        keycloak_cr[Keycloak CR]
        flightdeck_cr[Flightdeck CR]
        pgdb_cr[PostgresDatabase CR]
    end

    subgraph controllers [Controllers]
        site_ctrl[Site Controller]
        connect_ctrl[Connect Controller]
        workbench_ctrl[Workbench Controller]
        pm_ctrl[PackageManager Controller]
        chronicle_ctrl[Chronicle Controller]
        db_ctrl[Database Controller]
        flightdeck_ctrl[Flightdeck Controller]
    end

    subgraph k8s [Kubernetes Resources]
        deployments[Deployments]
        services[Services]
        ingresses[Ingresses]
        configmaps[ConfigMaps]
        secrets[Secrets]
        pvcs[PVCs]
        rbac[RBAC]
    end

    %% User creates Site
    kubectl --> site

    %% Site controller creates product CRs
    site --> site_ctrl
    site_ctrl --> connect_cr
    site_ctrl --> workbench_cr
    site_ctrl --> pm_cr
    site_ctrl --> chronicle_cr
    site_ctrl --> keycloak_cr
    site_ctrl --> flightdeck_cr
    site_ctrl --> pgdb_cr

    %% Product controllers watch CRs
    connect_cr --> connect_ctrl
    workbench_cr --> workbench_ctrl
    pm_cr --> pm_ctrl
    chronicle_cr --> chronicle_ctrl
    pgdb_cr --> db_ctrl
    flightdeck_cr --> flightdeck_ctrl

    %% Controllers create K8s resources
    connect_ctrl --> k8s
    workbench_ctrl --> k8s
    pm_ctrl --> k8s
    chronicle_ctrl --> k8s
    db_ctrl --> k8s
    flightdeck_ctrl --> k8s

    classDef crdStyle fill:#E8F5E9,stroke:#388E3C
    classDef ctrlStyle fill:#E3F2FD,stroke:#1976D2
    classDef k8sStyle fill:#FFF3E0,stroke:#F57C00

    class site,connect_cr,workbench_cr,pm_cr,chronicle_cr,keycloak_cr,flightdeck_cr,pgdb_cr crdStyle
    class site_ctrl,connect_ctrl,workbench_ctrl,pm_ctrl,chronicle_ctrl,db_ctrl,flightdeck_ctrl ctrlStyle
    class deployments,services,ingresses,configmaps,secrets,pvcs,rbac k8sStyle
```

### Reconciliation Flow

```mermaid
sequenceDiagram
    participant User
    participant K8s as Kubernetes API
    participant SiteCtrl as Site Controller
    participant ProductCR as Product CRs
    participant ProductCtrl as Product Controllers
    participant Resources as K8s Resources

    User->>K8s: Create/Update Site CR
    K8s->>SiteCtrl: Watch event triggered

    rect rgb(227, 242, 253)
        Note over SiteCtrl: Site Reconciliation
        SiteCtrl->>SiteCtrl: Determine database URL
        SiteCtrl->>SiteCtrl: Provision volumes (if needed)
        SiteCtrl->>ProductCR: Create/Update Connect CR
        SiteCtrl->>ProductCR: Create/Update Workbench CR
        SiteCtrl->>ProductCR: Create/Update PackageManager CR
        SiteCtrl->>ProductCR: Create/Update Chronicle CR
        SiteCtrl->>ProductCR: Create/Update Flightdeck CR
        SiteCtrl->>ProductCR: Create/Update Keycloak CR
    end

    ProductCR->>ProductCtrl: Watch events triggered

    rect rgb(232, 245, 233)
        Note over ProductCtrl: Product Reconciliation
        ProductCtrl->>ProductCtrl: Ensure database exists
        ProductCtrl->>Resources: Create ConfigMaps
        ProductCtrl->>Resources: Create Secrets
        ProductCtrl->>Resources: Create PVCs
        ProductCtrl->>Resources: Create Deployment
        ProductCtrl->>Resources: Create Service
        ProductCtrl->>Resources: Create Ingress
        ProductCtrl->>Resources: Create RBAC (if off-host)
    end

    Resources-->>K8s: Resources created
    K8s-->>User: Site ready
```

### Workbench Architecture

```mermaid
flowchart TB
    subgraph external [External Access]
        user[User Browser]
        ingress[Ingress Controller]
    end

    subgraph workbench_pod [Workbench Pod]
        wb_server[Workbench Server]
        launcher[Job Launcher]
    end

    subgraph k8s_api [Kubernetes API]
        api[API Server]
    end

    subgraph sessions [Session Pods]
        session1[Session Pod 1<br/>RStudio/VS Code/Jupyter]
        session2[Session Pod 2<br/>RStudio/VS Code/Jupyter]
        session3[Session Pod N<br/>...]
    end

    subgraph storage [Shared Storage]
        home_pvc[Home Directory PVC<br/>ReadWriteMany]
        shared_pvc[Shared Storage PVC<br/>ReadWriteMany]
    end

    subgraph config [Configuration]
        cm[ConfigMaps]
        templates[Job Templates]
        session_cm[Session ConfigMap]
    end

    %% User flow
    user --> ingress
    ingress --> wb_server

    %% Launcher creates sessions
    wb_server --> launcher
    launcher --> api
    api --> session1
    api --> session2
    api --> session3

    %% Storage connections
    wb_server --> home_pvc
    session1 --> home_pvc
    session2 --> home_pvc
    session3 --> home_pvc
    session1 --> shared_pvc
    session2 --> shared_pvc
    session3 --> shared_pvc

    %% Configuration
    cm --> wb_server
    templates --> launcher
    session_cm --> session1
    session_cm --> session2
    session_cm --> session3

    classDef external fill:#FAEEE9,stroke:#ab4d26
    classDef workbench fill:#E3F2FD,stroke:#1976D2
    classDef session fill:#E8F5E9,stroke:#388E3C
    classDef storage fill:#FFF3E0,stroke:#F57C00
    classDef config fill:#F3E5F5,stroke:#7B1FA2

    class user,ingress external
    class wb_server,launcher workbench
    class session1,session2,session3 session
    class home_pvc,shared_pvc storage
    class cm,templates,session_cm config
```

### Component Relationships

```mermaid
flowchart LR
    subgraph products [Posit Team Products]
        flightdeck[Flightdeck<br/>Landing Page]
        workbench[Workbench<br/>Development]
        connect[Connect<br/>Publishing]
        pm[Package Manager<br/>Packages]
        chronicle[Chronicle<br/>Telemetry]
    end

    subgraph shared [Shared Infrastructure]
        keycloak[Keycloak<br/>Authentication]
        postgres[(PostgreSQL<br/>Database)]
        storage[(Shared Storage<br/>NFS/EFS/FSx)]
    end

    %% Landing page links to products
    flightdeck -.-> workbench
    flightdeck -.-> connect
    flightdeck -.-> pm

    %% Product interactions
    workbench -->|Publish content| connect
    workbench -->|Fetch packages| pm
    connect -->|Fetch packages| pm

    %% Shared infrastructure
    workbench --> keycloak
    connect --> keycloak
    pm --> keycloak

    workbench --> postgres
    connect --> postgres
    pm --> postgres
    chronicle --> postgres

    workbench --> storage
    connect --> storage

    %% Chronicle collects from products
    chronicle -.->|Collect metrics| workbench
    chronicle -.->|Collect metrics| connect
    chronicle -.->|Collect metrics| pm

    classDef product fill:#E3F2FD,stroke:#1976D2
    classDef infra fill:#E8F5E9,stroke:#388E3C

    class flightdeck,workbench,connect,pm,chronicle product
    class keycloak,postgres,storage infra
```

## Key Concepts

### Site CRD

The `Site` Custom Resource is the primary configuration point. It contains:
- Global settings (domain, secrets, storage)
- Product-specific configuration sections
- Feature flags and experimental options

### Configuration Propagation

Configuration flows from Site CRD to individual product CRDs:

1. User edits Site spec
2. Site controller detects change
3. Site controller updates product CRs
4. Product controllers reconcile deployments

See [Adding Config Options](../guides/adding-config-options.md) for details on extending configuration.

## Quick Start

### View Sites

```bash
kubectl get sites -n posit-team
```

### Edit a Site

```bash
kubectl edit site main -n posit-team
```

### Check Operator Logs

```bash
# Operator runs in posit-team-system namespace
kubectl logs -n posit-team-system deployment/team-operator-controller-manager
```

## Namespaces

Team Operator uses two namespaces:

| Namespace | Purpose |
|-----------|---------|
| `posit-team-system` | Where the operator controller runs |
| `posit-team` (or configured `watchNamespace`) | Where Site CRs and deployed products live |

## Related Documentation

### Deployment and Operations

- [Site Management Guide](guides/product-team-site-management.md) - Creating, updating, and managing Site resources
- [Upgrading Guide](guides/upgrading.md) - Upgrade procedures and version migrations
- [Troubleshooting Guide](guides/troubleshooting.md) - Common issues and debugging techniques

### Product Configuration

- [Workbench Configuration](guides/workbench-configuration.md) - Interactive development environment setup
- [Connect Configuration](guides/connect-configuration.md) - Publishing platform configuration
- [Package Manager Configuration](guides/packagemanager-configuration.md) - Package repository management

### Authentication and Security

- [Authentication Setup](guides/authentication-setup.md) - SSO, OAuth, and Keycloak integration

### Reference

- [Architecture](architecture.md) - Detailed architecture diagrams with component explanations
- [API Reference](api-reference.md) - Complete CRD field reference for all resources

### For Contributors

- [Adding Config Options](guides/adding-config-options.md) - How to extend Site/product configurations
