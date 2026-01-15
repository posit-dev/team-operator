# Team Operator API Reference

This document provides a comprehensive reference for the Custom Resource Definitions (CRDs) provided by the Team Operator.

**API Group:** `team.posit.co/v1beta1`

## Table of Contents

- [Site](#site)
- [Connect](#connect)
- [Workbench](#workbench)
- [PackageManager](#packagemanager)
- [Chronicle](#chronicle)
- [PostgresDatabase](#postgresdatabase)
- [Flightdeck](#flightdeck)
- [Shared Types Reference](#shared-types-reference)
  - [AuthSpec](#authspec)
  - [SecretConfig](#secretconfig)
  - [VolumeSource](#volumesource)
  - [VolumeSpec](#volumespec)
  - [LicenseSpec](#licensespec)
  - [SessionConfig](#sessionconfig)
  - [SSHKeyConfig](#sshkeyconfig)

---

## Site

The Site CRD is the primary resource for managing a complete Posit Team deployment. It orchestrates all product components (Connect, Workbench, Package Manager, Chronicle) within a single site.

**Kind:** `Site`
**Plural:** `sites`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.domain` | `string` | **Yes** | The core domain name associated with the Posit Team Site |
| `.spec.awsAccountId` | `string` | No | AWS Account ID used for EKS-to-IAM annotations |
| `.spec.clusterDate` | `string` | No | Cluster date ID (YYYYmmdd) used for EKS-to-IAM annotations |
| `.spec.workloadCompoundName` | `string` | No | Name for the workload |
| `.spec.secretType` | `SiteSecretType` | No | **DEPRECATED** - Type of secret management to use |
| `.spec.ingressClass` | `string` | No | Ingress class for creating ingress routes |
| `.spec.ingressAnnotations` | `map[string]string` | No | Annotations applied to all ingress routes |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets for all image pulls (must exist in namespace) |
| `.spec.volumeSource` | [`VolumeSource`](#volumesource) | No | Definition of where volumes should be created from |
| `.spec.sharedDirectory` | `string` | No | Name of directory mounted into Workbench and Connect at `/mnt/<sharedDirectory>` (no slashes) |
| `.spec.volumeSubdirJobOff` | `bool` | No | Disables VolumeSubdir provisioning Kubernetes job |
| `.spec.extraSiteServiceAccounts` | `[]ServiceAccountConfig` | No | Additional service accounts prefixed by `<siteName>-` |
| `.spec.secret` | [`SecretConfig`](#secretconfig) | No | Secret management configuration for this Site |
| `.spec.workloadSecret` | [`SecretConfig`](#secretconfig) | No | Managed persistent secret for the entire workload account |
| `.spec.mainDatabaseCredentialSecret` | [`SecretConfig`](#secretconfig) | No | Secret for storing main database credentials |
| `.spec.disablePrePullImages` | `bool` | No | Disables pre-pulling of images |
| `.spec.dropDatabaseOnTeardown` | `bool` | No | Drop database when tearing down the site |
| `.spec.debug` | `bool` | No | Enable debug settings |
| `.spec.logFormat` | `LogFormat` | No | Log output format |
| `.spec.networkTrust` | `NetworkTrust` | No | Network trust level (0-100, default: 100) |
| `.spec.packageManagerUrl` | `string` | No | Package Manager URL for Workbench (defaults to local Package Manager) |
| `.spec.efsEnabled` | `bool` | No | Enable EFS for this site (allows workbench sessions to access EFS mount targets) |
| `.spec.vpcCIDR` | `string` | No | VPC CIDR block for EFS network policies |
| `.spec.enableFqdnHealthChecks` | `*bool` | No | Enable FQDN-based health check targets for Grafana Alloy (default: true) |

#### Product Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.flightdeck` | [`InternalFlightdeckSpec`](#internalflightdeckspec) | No | Flightdeck (landing page) configuration |
| `.spec.packageManager` | [`InternalPackageManagerSpec`](#internalpackagemanagerspec) | No | Posit Package Manager configuration |
| `.spec.connect` | [`InternalConnectSpec`](#internalconnectspec) | No | Posit Connect configuration |
| `.spec.workbench` | [`InternalWorkbenchSpec`](#internalworkbenchspec) | No | Posit Workbench configuration |
| `.spec.chronicle` | [`InternalChronicleSpec`](#internalchroniclespec) | No | Posit Chronicle configuration |
| `.spec.keycloak` | `InternalKeycloakSpec` | No | Keycloak configuration |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: Site
metadata:
  name: my-site
  namespace: posit-team
spec:
  domain: example.posit.team
  ingressClass: nginx
  ingressAnnotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
  secret:
    type: kubernetes
    vaultName: my-site-secrets
  volumeSource:
    type: nfs
    dnsName: nfs.example.com
  connect:
    license:
      type: FILE
      existingSecretName: connect-license
    image: ghcr.io/rstudio/rstudio-connect:2024.06.0
    replicas: 2
    auth:
      type: oidc
      clientId: connect-client
      issuer: https://auth.example.com
  workbench:
    license:
      type: FILE
      existingSecretName: workbench-license
    image: ghcr.io/rstudio/rstudio-workbench:2024.04.2
    replicas: 1
  packageManager:
    license:
      type: FILE
      existingSecretName: packagemanager-license
    image: ghcr.io/rstudio/rstudio-package-manager:2024.04.4
    replicas: 1
```

---

## Connect

The Connect CRD manages standalone Posit Connect deployments. When using the Site CRD, Connect configuration is typically specified via `.spec.connect` rather than creating a separate Connect resource.

**Kind:** `Connect`
**Plural:** `connects`
**Short Names:** `con`, `cons`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.license` | [`LicenseSpec`](#licensespec) | No | License configuration |
| `.spec.config` | `ConnectConfig` | No | Connect application configuration |
| `.spec.sessionConfig` | [`SessionConfig`](#sessionconfig) | No | Session pod configuration |
| `.spec.volume` | [`VolumeSpec`](#volumespec) | No | Data volume configuration |
| `.spec.secretType` | `SiteSecretType` | No | Secret management type |
| `.spec.auth` | [`AuthSpec`](#authspec) | No | Authentication configuration |
| `.spec.url` | `string` | No | Public URL for Connect |
| `.spec.databaseConfig` | `PostgresDatabaseConfig` | No | PostgreSQL database configuration |
| `.spec.ingressClass` | `string` | No | Ingress class for routing |
| `.spec.ingressAnnotations` | `map[string]string` | No | Ingress annotations |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets |
| `.spec.nodeSelector` | `map[string]string` | No | Node selector for pod scheduling |
| `.spec.addEnv` | `map[string]string` | No | Additional environment variables |
| `.spec.offHostExecution` | `bool` | No | Enable off-host execution (Kubernetes launcher) |
| `.spec.image` | `string` | No | Connect container image |
| `.spec.imagePullPolicy` | `PullPolicy` | No | Image pull policy |
| `.spec.sleep` | `bool` | No | Put service to sleep (debugging) |
| `.spec.sessionImage` | `string` | No | Container image for sessions |
| `.spec.awsAccountId` | `string` | No | AWS Account ID for IAM annotations |
| `.spec.clusterDate` | `string` | No | Cluster date ID for IAM annotations |
| `.spec.workloadCompoundName` | `string` | No | Workload name |
| `.spec.chronicleAgentImage` | `string` | No | Chronicle Agent container image |
| `.spec.additionalVolumes` | `[]VolumeSpec` | No | Additional volume definitions |
| `.spec.secret` | [`SecretConfig`](#secretconfig) | No | Secret management configuration |
| `.spec.workloadSecret` | [`SecretConfig`](#secretconfig) | No | Workload secret configuration |
| `.spec.mainDatabaseCredentialSecret` | [`SecretConfig`](#secretconfig) | No | Database credential secret |
| `.spec.debug` | `bool` | No | Enable debug settings |
| `.spec.replicas` | `int` | No | Number of Connect replicas |
| `.spec.dsnSecret` | `string` | No | DSN secret name for sessions |
| `.spec.chronicleSidecarProductApiKeyEnabled` | `bool` | No | Enable Chronicle sidecar API key injection |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `.status.keySecretRef` | `SecretReference` | Reference to the key secret |
| `.status.ready` | `bool` | Whether Connect is ready |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: Connect
metadata:
  name: my-connect
  namespace: posit-team
spec:
  license:
    type: FILE
    existingSecretName: connect-license
  image: ghcr.io/rstudio/rstudio-connect:2024.06.0
  imagePullPolicy: IfNotPresent
  replicas: 2
  offHostExecution: true
  auth:
    type: oidc
    clientId: connect-client
    issuer: https://auth.example.com
  config:
    Server:
      Address: "https://connect.example.com"
    Database:
      Provider: postgres
  volume:
    create: true
    size: 100Gi
    storageClassName: gp3
```

---

## Workbench

The Workbench CRD manages standalone Posit Workbench deployments. When using the Site CRD, Workbench configuration is typically specified via `.spec.workbench` rather than creating a separate Workbench resource.

**Kind:** `Workbench`
**Plural:** `workbenches`
**Singular:** `workbench`
**Short Names:** `wb`, `wbs`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.license` | [`LicenseSpec`](#licensespec) | No | License configuration |
| `.spec.config` | `WorkbenchConfig` | No | Workbench application configuration |
| `.spec.secretConfig` | `WorkbenchSecretConfig` | No | Secret configuration for Workbench |
| `.spec.sessionConfig` | [`SessionConfig`](#sessionconfig) | No | Session pod configuration |
| `.spec.volume` | [`VolumeSpec`](#volumespec) | No | Home directory volume configuration |
| `.spec.secretType` | `SiteSecretType` | No | Secret management type |
| `.spec.auth` | [`AuthSpec`](#authspec) | No | Authentication configuration |
| `.spec.url` | `string` | No | Public URL for Workbench |
| `.spec.parentUrl` | `string` | No | Parent URL for navigation |
| `.spec.nonRoot` | `bool` | No | Enable rootless execution mode |
| `.spec.databaseConfig` | `PostgresDatabaseConfig` | No | PostgreSQL database configuration |
| `.spec.ingressClass` | `string` | No | Ingress class for routing |
| `.spec.ingressAnnotations` | `map[string]string` | No | Ingress annotations |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets |
| `.spec.nodeSelector` | `map[string]string` | No | Node selector for pod scheduling |
| `.spec.tolerations` | `[]Toleration` | No | Pod tolerations |
| `.spec.addEnv` | `map[string]string` | No | Additional environment variables |
| `.spec.offHostExecution` | `bool` | No | Enable off-host execution (Kubernetes launcher) |
| `.spec.image` | `string` | No | Workbench container image |
| `.spec.imagePullPolicy` | `PullPolicy` | No | Image pull policy |
| `.spec.sleep` | `bool` | No | Put service to sleep (debugging) |
| `.spec.snowflake` | `SnowflakeConfig` | No | Snowflake integration configuration |
| `.spec.awsAccountId` | `string` | No | AWS Account ID for IAM annotations |
| `.spec.clusterDate` | `string` | No | Cluster date ID for IAM annotations |
| `.spec.workloadCompoundName` | `string` | No | Workload name |
| `.spec.chronicleAgentImage` | `string` | No | Chronicle Agent container image |
| `.spec.additionalVolumes` | `[]VolumeSpec` | No | Additional volume definitions |
| `.spec.secret` | [`SecretConfig`](#secretconfig) | No | Secret management configuration |
| `.spec.workloadSecret` | [`SecretConfig`](#secretconfig) | No | Workload secret configuration |
| `.spec.mainDatabaseCredentialSecret` | [`SecretConfig`](#secretconfig) | No | Database credential secret |
| `.spec.replicas` | `int` | No | Number of Workbench replicas |
| `.spec.dsnSecret` | `string` | No | DSN secret name for sessions |
| `.spec.chronicleSidecarProductApiKeyEnabled` | `bool` | No | Enable Chronicle sidecar API key injection |
| `.spec.authLoginPageHtml` | `string` | No | Custom HTML for login page (max 64KB) |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `.status.ready` | `bool` | Whether Workbench is ready |
| `.status.keySecretRef` | `SecretReference` | Reference to the key secret |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: Workbench
metadata:
  name: my-workbench
  namespace: posit-team
spec:
  license:
    type: FILE
    existingSecretName: workbench-license
  image: ghcr.io/rstudio/rstudio-workbench:2024.04.2
  imagePullPolicy: IfNotPresent
  replicas: 1
  offHostExecution: true
  auth:
    type: oidc
    clientId: workbench-client
    issuer: https://auth.example.com
  config:
    WorkbenchIniConfig:
      RServer:
        adminEnabled: 1
        adminGroup: "workbench-admin"
  volume:
    create: true
    size: 500Gi
    storageClassName: gp3
```

---

## PackageManager

The PackageManager CRD manages standalone Posit Package Manager deployments. When using the Site CRD, Package Manager configuration is typically specified via `.spec.packageManager` rather than creating a separate PackageManager resource.

**Kind:** `PackageManager`
**Plural:** `packagemanagers`
**Short Names:** `pm`, `pms`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.license` | [`LicenseSpec`](#licensespec) | No | License configuration |
| `.spec.config` | `PackageManagerConfig` | No | Package Manager application configuration |
| `.spec.volume` | [`VolumeSpec`](#volumespec) | No | Data volume configuration |
| `.spec.secretType` | `SiteSecretType` | No | Secret management type |
| `.spec.url` | `string` | No | Public URL for Package Manager |
| `.spec.databaseConfig` | `PostgresDatabaseConfig` | No | PostgreSQL database configuration |
| `.spec.ingressClass` | `string` | No | Ingress class for routing |
| `.spec.ingressAnnotations` | `map[string]string` | No | Ingress annotations |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets |
| `.spec.nodeSelector` | `map[string]string` | No | Node selector for pod scheduling |
| `.spec.addEnv` | `map[string]string` | No | Additional environment variables |
| `.spec.image` | `string` | No | Package Manager container image |
| `.spec.imagePullPolicy` | `PullPolicy` | No | Image pull policy |
| `.spec.sleep` | `bool` | No | Put service to sleep (debugging) |
| `.spec.awsAccountId` | `string` | No | AWS Account ID for IAM annotations |
| `.spec.workloadCompoundName` | `string` | No | Workload name |
| `.spec.clusterDate` | `string` | No | Cluster date ID for IAM annotations |
| `.spec.chronicleAgentImage` | `string` | No | Chronicle Agent container image |
| `.spec.secret` | [`SecretConfig`](#secretconfig) | No | Secret management configuration |
| `.spec.workloadSecret` | [`SecretConfig`](#secretconfig) | No | Workload secret configuration |
| `.spec.mainDatabaseCredentialSecret` | [`SecretConfig`](#secretconfig) | No | Database credential secret |
| `.spec.replicas` | `int` | No | Number of Package Manager replicas |
| `.spec.gitSSHKeys` | [`[]SSHKeyConfig`](#sshkeyconfig) | No | SSH key configurations for Git authentication |
| `.spec.azureFiles` | `AzureFilesConfig` | No | Azure Files integration configuration |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `.status.keySecretRef` | `SecretReference` | Reference to the key secret |
| `.status.ready` | `bool` | Whether Package Manager is ready |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: PackageManager
metadata:
  name: my-packagemanager
  namespace: posit-team
spec:
  license:
    type: FILE
    existingSecretName: packagemanager-license
  image: ghcr.io/rstudio/rstudio-package-manager:2024.04.4
  imagePullPolicy: IfNotPresent
  replicas: 1
  config:
    Server:
      Address: ":4242"
      DataDir: /data
    Database:
      Provider: postgres
    S3Storage:
      Bucket: my-rspm-bucket
      Region: us-east-1
```

---

## Chronicle

The Chronicle CRD manages Posit Chronicle deployments for usage tracking and auditing.

**Kind:** `Chronicle`
**Plural:** `chronicles`
**Short Names:** `pcr`, `chr`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.config` | `ChronicleConfig` | No | Chronicle application configuration |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets |
| `.spec.nodeSelector` | `map[string]string` | No | Node selector for pod scheduling |
| `.spec.addEnv` | `map[string]string` | No | Additional environment variables |
| `.spec.image` | `string` | No | Chronicle container image |
| `.spec.awsAccountId` | `string` | No | AWS Account ID for IAM annotations |
| `.spec.clusterDate` | `string` | No | Cluster date ID for IAM annotations |
| `.spec.workloadCompoundName` | `string` | No | Workload name |

### ChronicleConfig

| Field | Type | Description |
|-------|------|-------------|
| `.Http` | `ChronicleHttpConfig` | HTTP server configuration |
| `.Metrics` | `ChronicleMetricsConfig` | Prometheus metrics configuration |
| `.Profiling` | `ChronicleProfilingConfig` | Profiling configuration |
| `.S3Storage` | `ChronicleS3StorageConfig` | S3 storage configuration |
| `.LocalStorage` | `ChronicleLocalStorageConfig` | Local storage configuration |
| `.Logging` | `ChronicleLoggingConfig` | Logging configuration |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `.status.ready` | `bool` | Whether Chronicle is ready |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: Chronicle
metadata:
  name: my-chronicle
  namespace: posit-team
spec:
  image: ghcr.io/rstudio/chronicle:2023.10.4
  config:
    Http:
      Listen: ":8080"
    S3Storage:
      Enabled: true
      Bucket: my-chronicle-bucket
      Region: us-east-1
    Metrics:
      Enabled: true
      Listen: ":9090"
```

---

## PostgresDatabase

The PostgresDatabase CRD manages PostgreSQL database provisioning for Posit Team products.

**Kind:** `PostgresDatabase`
**Plural:** `postgresdatabases`
**Short Names:** `pgdb`, `pgdbs`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.url` | `string` | **Yes** | PostgreSQL connection URL (must match `^postgres.+@.+/.+`) |
| `.spec.secretVault` | `string` | **Yes** | Secret ID for retrieving the password |
| `.spec.secretPasswordKey` | `string` | **Yes** | Password key within the SecretVault |
| `.spec.secret` | [`SecretConfig`](#secretconfig) | No | Secret configuration for password retrieval |
| `.spec.workloadSecret` | [`SecretConfig`](#secretconfig) | No | Workload secret configuration |
| `.spec.mainDbCredentialSecret` | [`SecretConfig`](#secretconfig) | No | Main database credential secret |
| `.spec.extensions` | `[]string` | No | PostgreSQL extensions to enable |
| `.spec.schemas` | `[]string` | No | Database schemas to create |
| `.spec.teardown` | `PostgresDatabaseSpecTeardown` | No | Teardown behavior configuration |

### PostgresDatabaseConfig

Used by products to configure database connections:

| Field | Type | Description |
|-------|------|-------------|
| `.host` | `string` | Database host |
| `.sslMode` | `string` | SSL mode for connections |
| `.dropOnTeardown` | `bool` | Drop database on teardown |
| `.schema` | `string` | Default schema |
| `.instrumentationSchema` | `string` | Schema for instrumentation data |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: PostgresDatabase
metadata:
  name: connect-db
  namespace: posit-team
spec:
  url: "postgres://connect_user@db.example.com/connect_db"
  secretVault: my-site-secrets
  secretPasswordKey: connect-db-password
  extensions:
    - pgcrypto
  schemas:
    - connect
    - instrumentation
```

---

## Flightdeck

The Flightdeck CRD manages the Posit Team landing page dashboard.

**Kind:** `Flightdeck`
**Plural:** `flightdecks`
**Scope:** Namespaced

### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.spec.siteName` | `string` | No | Name of the Site that owns this Flightdeck |
| `.spec.image` | `string` | No | Flightdeck container image |
| `.spec.imagePullPolicy` | `PullPolicy` | No | Image pull policy |
| `.spec.port` | `int32` | No | Container listening port (default: 8080) |
| `.spec.replicas` | `int` | No | Number of replicas (default: 1) |
| `.spec.featureEnabler` | `FeatureEnablerConfig` | No | Feature toggles |
| `.spec.domain` | `string` | No | Domain name for ingress |
| `.spec.ingressClass` | `string` | No | Ingress class to use |
| `.spec.ingressAnnotations` | `map[string]string` | No | Ingress annotations |
| `.spec.imagePullSecrets` | `[]string` | No | Image pull secrets |
| `.spec.awsAccountId` | `string` | No | AWS Account ID |
| `.spec.clusterDate` | `string` | No | Cluster date ID |
| `.spec.workloadCompoundName` | `string` | No | Workload name |
| `.spec.logLevel` | `string` | No | Log level (debug, info, warn, error) - default: "info" |
| `.spec.logFormat` | `string` | No | Log format (text, json) - default: "text" |

### FeatureEnablerConfig

| Field | Type | Description |
|-------|------|-------------|
| `.showConfig` | `bool` | Enable configuration page (default: false) |
| `.showAcademy` | `bool` | Enable academy page (default: false) |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `.status.ready` | `bool` | Whether Flightdeck is ready |

### Example Manifest

```yaml
apiVersion: team.posit.co/v1beta1
kind: Flightdeck
metadata:
  name: my-flightdeck
  namespace: posit-team
spec:
  siteName: my-site
  image: docker.io/posit/ptd-flightdeck:latest
  replicas: 1
  domain: flightdeck.example.com
  ingressClass: nginx
  featureEnabler:
    showConfig: true
```

---

## Shared Types Reference

### AuthSpec

Authentication configuration used by Connect and Workbench.

| Field | Type | Description |
|-------|------|-------------|
| `.type` | `AuthType` | Authentication type: `password`, `oidc`, or `saml` |
| `.clientId` | `string` | OAuth2/OIDC client ID |
| `.issuer` | `string` | OIDC issuer URL |
| `.groups` | `bool` | Enable group synchronization |
| `.usernameClaim` | `string` | OIDC claim for username |
| `.emailClaim` | `string` | OIDC claim for email |
| `.uniqueIdClaim` | `string` | OIDC claim for unique ID |
| `.groupsClaim` | `string` | OIDC claim for groups |
| `.disableGroupsClaim` | `bool` | Disable groups claim processing |
| `.samlMetadataUrl` | `string` | SAML IdP metadata URL |
| `.samlIdPAttributeProfile` | `string` | SAML IdP attribute profile |
| `.samlUsernameAttribute` | `string` | SAML attribute for username |
| `.samlFirstNameAttribute` | `string` | SAML attribute for first name |
| `.samlLastNameAttribute` | `string` | SAML attribute for last name |
| `.samlEmailAttribute` | `string` | SAML attribute for email |
| `.scopes` | `[]string` | Additional OIDC scopes |
| `.viewerRoleMapping` | `[]string` | Groups mapped to viewer role |
| `.publisherRoleMapping` | `[]string` | Groups mapped to publisher role |
| `.administratorRoleMapping` | `[]string` | Groups mapped to administrator role |

**AuthType Values:**

| Value | Description |
|-------|-------------|
| `password` | Local username/password authentication |
| `oidc` | OpenID Connect authentication |
| `saml` | SAML 2.0 authentication |

### SecretConfig

Configuration for secret management.

| Field | Type | Description |
|-------|------|-------------|
| `.vaultName` | `string` | Name of the secret vault/secret |
| `.type` | `SiteSecretType` | Secret management type |

**SiteSecretType Values:**

| Value | Description |
|-------|-------------|
| `kubernetes` | Use Kubernetes Secrets |
| `aws` | Use AWS Secrets Manager with CSI driver |
| `test` | Test mode (in-memory) |

### VolumeSource

Configuration for the source of persistent volumes.

| Field | Type | Description |
|-------|------|-------------|
| `.type` | `VolumeSourceType` | Volume source type |
| `.volumeId` | `string` | Volume identifier (e.g., FSx volume ID) |
| `.dnsName` | `string` | DNS name for volume access |

**VolumeSourceType Values:**

| Value | Description |
|-------|-------------|
| `fsx-zfs` | Amazon FSx for OpenZFS |
| `nfs` | NFS server |
| `azure-netapp` | Azure NetApp Files |

### VolumeSpec

Specification for creating or mounting a PersistentVolumeClaim.

| Field | Type | Description |
|-------|------|-------------|
| `.create` | `bool` | Whether to create the PVC |
| `.accessModes` | `[]string` | Access modes (when creating) |
| `.volumeName` | `string` | PV name to reference (when creating) |
| `.storageClassName` | `string` | Storage class name (when creating) |
| `.size` | `string` | PVC size (when creating) |
| `.pvcName` | `string` | Existing PVC name (when not creating) |
| `.mountPath` | `string` | Mount path for additional volumes |
| `.readOnly` | `bool` | Mount as read-only (default: false) |

### LicenseSpec

Product license configuration.

| Field | Type | Description |
|-------|------|-------------|
| `.type` | `LicenseType` | License type |
| `.key` | `string` | License key (for KEY type) |
| `.existingSecretName` | `string` | Name of existing secret containing license |
| `.existingSecretKey` | `string` | Key within the secret (default: "license.lic") |

**LicenseType Values:**

| Value | Description |
|-------|-------------|
| `KEY` | License key string |
| `FILE` | License file |

### SessionConfig

Configuration for session pods (Connect and Workbench).

| Field | Type | Description |
|-------|------|-------------|
| `.service` | `ServiceConfig` | Service configuration for sessions |
| `.pod` | `PodConfig` | Pod configuration for sessions |
| `.job` | `JobConfig` | Job configuration for sessions |

**ServiceConfig:**

| Field | Type | Description |
|-------|------|-------------|
| `.type` | `string` | Kubernetes service type |
| `.annotations` | `map[string]string` | Service annotations |
| `.labels` | `map[string]string` | Service labels |

**PodConfig:**

| Field | Type | Description |
|-------|------|-------------|
| `.annotations` | `map[string]string` | Pod annotations |
| `.labels` | `map[string]string` | Pod labels |
| `.serviceAccountName` | `string` | Service account for pods |
| `.volumes` | `[]Volume` | Additional volumes |
| `.volumeMounts` | `[]VolumeMount` | Additional volume mounts |
| `.env` | `[]EnvVar` | Environment variables |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.imagePullSecrets` | `[]LocalObjectReference` | Image pull secrets |
| `.initContainers` | `[]Container` | Init containers |
| `.extraContainers` | `[]Container` | Sidecar containers |
| `.containerSecurityContext` | `SecurityContext` | Container security context |
| `.tolerations` | `[]Toleration` | Pod tolerations |
| `.affinity` | `*Affinity` | Pod affinity rules |
| `.nodeSelector` | `map[string]string` | Node selector |
| `.priorityClassName` | `string` | Priority class name |
| `.command` | `[]string` | Override container command |

### SSHKeyConfig

SSH key configuration for Git authentication in Package Manager.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.name` | `string` | **Yes** | Unique identifier (1-63 chars, lowercase alphanumeric with hyphens) |
| `.host` | `string` | **Yes** | Git host domain (e.g., "github.com") |
| `.secretRef` | `SecretReference` | **Yes** | Reference to the SSH key secret |
| `.passphraseSecretRef` | `*SecretReference` | No | Reference to passphrase secret for encrypted keys |

**SecretReference:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `.source` | `string` | **Yes** | Secret source: `aws-secrets-manager`, `kubernetes`, or `azure-key-vault` |
| `.name` | `string` | **Yes** | Secret name in the specified source |
| `.key` | `string` | No | Key within the secret (primarily for Kubernetes secrets) |

---

## Site Internal Specs

These types are used within the Site CRD for product configuration.

### InternalFlightdeckSpec

| Field | Type | Description |
|-------|------|-------------|
| `.enabled` | `*bool` | Enable Flightdeck (default: true) |
| `.image` | `string` | Container image |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.replicas` | `int` | Number of replicas |
| `.featureEnabler` | `FeatureEnablerConfig` | Feature toggles |
| `.logLevel` | `string` | Log level (default: "info") |
| `.logFormat` | `string` | Log format (default: "text") |

### InternalPackageManagerSpec

| Field | Type | Description |
|-------|------|-------------|
| `.license` | `LicenseSpec` | License configuration |
| `.volume` | `*VolumeSpec` | Data volume |
| `.nodeSelector` | `map[string]string` | Node selector |
| `.addEnv` | `map[string]string` | Environment variables |
| `.image` | `string` | Container image |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.s3Bucket` | `string` | S3 bucket for package storage |
| `.replicas` | `int` | Number of replicas |
| `.domainPrefix` | `string` | Domain prefix (default: "packagemanager") |
| `.gitSSHKeys` | `[]SSHKeyConfig` | SSH keys for Git authentication |
| `.azureFiles` | `*AzureFilesConfig` | Azure Files configuration |

### InternalConnectSpec

| Field | Type | Description |
|-------|------|-------------|
| `.license` | `LicenseSpec` | License configuration |
| `.volume` | `*VolumeSpec` | Data volume |
| `.nodeSelector` | `map[string]string` | Node selector |
| `.auth` | `AuthSpec` | Authentication configuration |
| `.addEnv` | `map[string]string` | Environment variables |
| `.image` | `string` | Container image |
| `.sessionImage` | `string` | Session container image |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.databricks` | `*DatabricksConfig` | Databricks integration |
| `.loggedInWarning` | `string` | Warning message for logged-in users |
| `.publicWarning` | `string` | Public warning message |
| `.replicas` | `int` | Number of replicas |
| `.experimentalFeatures` | `*InternalConnectExperimentalFeatures` | Experimental features |
| `.domainPrefix` | `string` | Domain prefix (default: "connect") |
| `.gpuSettings` | `*GPUSettings` | GPU resource configuration |
| `.databaseSettings` | `*DatabaseSettings` | Database schema settings |
| `.scheduleConcurrency` | `int` | Schedule concurrency (default: 2) |

### InternalWorkbenchSpec

| Field | Type | Description |
|-------|------|-------------|
| `.databricks` | `map[string]DatabricksConfig` | Databricks configurations |
| `.snowflake` | `SnowflakeConfig` | Snowflake configuration |
| `.license` | `LicenseSpec` | License configuration |
| `.volume` | `*VolumeSpec` | Home directory volume |
| `.additionalVolumes` | `[]VolumeSpec` | Additional volumes |
| `.nodeSelector` | `map[string]string` | Node selector |
| `.tolerations` | `[]Toleration` | Pod tolerations |
| `.sessionTolerations` | `[]Toleration` | Session-only tolerations |
| `.createUsersAutomatically` | `bool` | Auto-create users |
| `.adminGroups` | `[]string` | Admin groups (default: ["workbench-admin"]) |
| `.adminSuperuserGroups` | `[]string` | Superuser groups |
| `.addEnv` | `map[string]string` | Environment variables |
| `.auth` | `AuthSpec` | Authentication configuration |
| `.image` | `string` | Container image |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.defaultSessionImage` | `string` | Default session image |
| `.extraSessionImages` | `[]string` | Additional session images |
| `.sessionInitContainerImageName` | `string` | Init container image name |
| `.sessionInitContainerImageTag` | `string` | Init container image tag |
| `.replicas` | `int` | Number of replicas |
| `.experimentalFeatures` | `*InternalWorkbenchExperimentalFeatures` | Experimental features |
| `.vsCodeExtensions` | `[]string` | VS Code extensions to install |
| `.vsCodeUserSettings` | `map[string]*JSON` | VS Code user settings |
| `.positronConfig` | `PositronConfig` | Positron configuration |
| `.vsCodeConfig` | `VSCodeConfig` | VS Code configuration |
| `.apiSettings` | `ApiSettingsConfig` | API settings |
| `.domainPrefix` | `string` | Domain prefix (default: "workbench") |
| `.authLoginPageHtml` | `string` | Custom login page HTML |
| `.jupyterConfig` | `*WorkbenchJupyterConfig` | Jupyter configuration |

### InternalChronicleSpec

| Field | Type | Description |
|-------|------|-------------|
| `.nodeSelector` | `map[string]string` | Node selector |
| `.image` | `string` | Container image |
| `.addEnv` | `map[string]string` | Environment variables |
| `.imagePullPolicy` | `PullPolicy` | Image pull policy |
| `.s3Bucket` | `string` | S3 bucket for storage |
| `.agentImage` | `string` | Agent container image |

---

## Labels Applied by the Operator

The Team Operator applies the following labels to managed resources:

| Label | Description |
|-------|-------------|
| `app.kubernetes.io/managed-by: team-operator` | Indicates resource is managed by the operator |
| `app.kubernetes.io/name` | Component type (e.g., "connect", "workbench") |
| `app.kubernetes.io/instance` | Component instance name |
| `posit.team/site` | Site name |
| `posit.team/component` | Component type |
