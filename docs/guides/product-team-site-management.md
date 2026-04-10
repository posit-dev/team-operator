---
title: Site Management
description: Managing Site resources in Team Operator for platform engineers deploying Posit Team
---

# Site Management Guide

The `Site` Custom Resource is the central concept in Team Operator. It is the single configuration object that describes an entire Posit Team deployment: which products to run, how they authenticate, where they store data, and how they are exposed to users. Everything else the operator manages — Connect, Workbench, Package Manager, Chronicle, Flightdeck — derives from the Site spec.

Understanding the Site CR means understanding how Team Operator works. When you change a Site, the operator propagates that change through the entire product stack automatically. When you delete a Site, the operator tears down all the resources it created. You do not manage individual product CRs directly; you declare the desired state in the Site, and the operator reconciles the rest.

## Overview

A Site represents a complete deployment environment that includes the following components:

- **Flightdeck** - Landing page dashboard
- **Connect** - Publishing and sharing platform
- **Workbench** - Interactive development environment
- **Package Manager** - Package repository management
- **Chronicle** - Telemetry and monitoring
- **Keycloak** - Authentication and identity management (optional)

When you create or update a Site, the Site controller reconciles all child product Custom Resources (Connect, Workbench, Package Manager, Chronicle, Flightdeck) to match your desired configuration.

## Site Lifecycle

### Creating a Site

To create a new Posit Team deployment, apply a Site manifest:

```bash
kubectl apply -f site.yaml -n posit-team
```

On creation, the Site controller works through a predictable sequence: it provisions storage volumes (FSx, NFS, or Azure NetApp based on your configuration), runs subdirectory provisioning jobs for shared storage, creates the Flightdeck landing page CR, creates the Connect, Workbench, Package Manager, and Chronicle CRs, establishes network policies for product communication, and creates any extra service accounts you specified.

### Updating Site Configuration

To update a Site, either edit it in-place or apply an updated manifest:

```bash
kubectl edit site <site-name> -n posit-team
```

```bash
kubectl apply -f site.yaml -n posit-team
```

The Site controller detects the change and propagates it through the stack. Changes flow from the Site spec down through each product CR, and then each product controller reconciles its own deployment, services, and ingress resources:

```
Site spec change
    -> Site controller reconciles
        -> Product CRs updated (Connect, Workbench, PM, Chronicle, Flightdeck)
            -> Product controllers reconcile
                -> Deployments, Services, Ingress updated
```

### Deleting a Site

Deleting a Site removes all resources the operator created for that deployment:

```bash
kubectl delete site <site-name> -n posit-team
```

Child resources hold owner references to the Site, so Kubernetes garbage collection handles most of the cleanup automatically. The operator also removes the Connect, Workbench, Package Manager, Flightdeck, and network policy resources. If you set `dropDatabaseOnTearDown: true`, product databases will be dropped as well — use that option with caution in production environments.

## Site Spec Structure

The Site spec is organized into logical sections: core settings that apply across all products, followed by per-product configuration blocks. The sections below walk through each area with the key fields and their purpose.

### Core Configuration

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: example-site
  namespace: posit-team
spec:
  # Required: Base domain for all products
  domain: example.mycompany.com

  # AWS-specific configuration (for EKS deployments)
  awsAccountId: "123456789012"
  clusterDate: "20240101"
  workloadCompoundName: "my-workload"

  # Enable debug logging for all products
  debug: false

  # Log format: "text" (default) or "json"
  logFormat: text

  # Network trust level (0-100, default 100)
  networkTrust: 100
```

### Domain Configuration

Products are accessed via subdomains of the base domain:

| Product | Default Subdomain | URL Pattern |
|---------|-------------------|-------------|
| Connect | `connect` | `connect.example.mycompany.com` |
| Workbench | `workbench` | `workbench.example.mycompany.com` |
| Package Manager | `packagemanager` | `packagemanager.example.mycompany.com` |

You can customize prefixes in each product's configuration:

```yaml
spec:
  domain: example.mycompany.com
  connect:
    domainPrefix: connect  # Default
  workbench:
    domainPrefix: workbench  # Default
  packageManager:
    domainPrefix: packagemanager  # Default
```

### Ingress Configuration

```yaml
spec:
  # Ingress class for all products
  ingressClass: traefik

  # Annotations applied to all ingress resources
  ingressAnnotations:
    traefik.ingress.kubernetes.io/router.middlewares: kube-system-traefik-forward-auth@kubernetescrd
```

### Secret Management

The operator reads authentication credentials (OIDC client secrets, database passwords, and similar values) from a secrets backend that you configure at the Site level. This keeps sensitive values out of the Site spec itself. The operator supports two backends: standard Kubernetes Secrets and AWS Secrets Manager. For multi-site workloads, you can configure a separate workload-scoped secret in addition to the site-scoped one.

```yaml
spec:
  # Site-level secrets configuration
  secret:
    type: "kubernetes"  # or "aws"
    vaultName: "site-secrets"

  # Workload-level secrets (for multi-site workloads)
  workloadSecret:
    type: "kubernetes"
    vaultName: "workload-secrets"

  # Database credentials secret
  mainDatabaseCredentialSecret:
    type: "aws"  # AWS Secrets Manager
    vaultName: "rds!db-example-database-id"
```

| Type | Description |
|------|-------------|
| `kubernetes` | Standard Kubernetes Secrets |
| `aws` | AWS Secrets Manager |

### Storage Configuration

The operator can provision and manage persistent storage for product data. Storage is configured via a `volumeSource` that specifies the underlying storage technology. In AWS deployments, FSx for OpenZFS is the typical choice; Azure deployments use Azure NetApp Files; and generic NFS works across any environment. If you leave `volumeSource` empty, the operator does not create any managed volumes and you are responsible for providing storage.

#### Volume Source Types

```yaml
spec:
  volumeSource:
    # FSx for OpenZFS (AWS)
    type: fsx-zfs
    volumeId: fsvol-example123456789
    dnsName: fs-example123456789.fsx.us-east-1.amazonaws.com

    # NFS
    type: nfs
    volumeId: nfs-server-address
    dnsName: nfs.example.com

    # Azure NetApp Files
    type: azure-netapp
```

**Supported Volume Types:**

| Type | Description | Cloud Provider |
|------|-------------|----------------|
| `fsx-zfs` | FSx for OpenZFS | AWS |
| `nfs` | Generic NFS | Any |
| `azure-netapp` | Azure NetApp Files | Azure |
| `` (empty) | No managed volumes | Any |

#### Shared Directory

Configure a shared directory mounted across Workbench and Connect:

```yaml
spec:
  # Creates /mnt/shared in both Workbench and Connect
  sharedDirectory: shared
```

#### EFS Configuration (AWS)

```yaml
spec:
  efsEnabled: true
  vpcCIDR: "10.0.0.0/16"  # Required for EFS network policies
```

### Product Enablement

Each product has its own configuration block in the Site spec. All products are enabled by default. Setting `enabled: false` on a product suspends it without deleting its data, while setting `teardown: true` permanently removes all associated resources. The sections below cover the key configuration options for each product.

#### Flightdeck (Landing Page)

```yaml
spec:
  flightdeck:
    enabled: true  # Default: true
    image: "docker.io/posit/ptd-flightdeck:latest"
    imagePullPolicy: Always
    replicas: 1
    logLevel: info  # debug, info, warn, error
    logFormat: text  # text, json
    featureEnabler:
      showConfig: false
      showAcademy: false
```

#### Connect

```yaml
spec:
  connect:
    # Enable/disable Connect deployment (default: true).
    # Setting enabled: false suspends Connect (preserves data).
    # Use teardown: true to permanently delete all Connect data.
    # See the Connect Configuration Guide for details.
    enabled: true

    image: "ghcr.io/posit-dev/connect:ubuntu22-2024.10.0"
    imagePullPolicy: IfNotPresent
    replicas: 1
    domainPrefix: connect

    # License configuration
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pc.lic

    # Volume for Connect data
    volume:
      create: false
      size: 3Gi

    # Authentication
    auth:
      type: "oidc"  # or "password", "saml"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"

    # Node placement
    nodeSelector:
      node-type: posit-products

    # Additional environment variables
    addEnv:
      CUSTOM_VAR: "value"

    # GPU settings for content execution
    gpuSettings:
      nvidiaGPULimit: 1
      maxNvidiaGPULimit: 4

    # Database schema settings
    databaseSettings:
      schema: "connect"
      instrumentationSchema: "connect_instrumentation"

    # Content scheduling concurrency
    scheduleConcurrency: 2

    # Databricks integration
    databricks:
      url: "https://workspace.cloud.databricks.com"
      clientId: "databricks-client-id"

    # Experimental features
    experimentalFeatures:
      mailSender: "connect@example.com"
      mailDisplayName: "Posit Connect"
      sessionServiceAccountName: "custom-session-sa"
```

#### Workbench

```yaml
spec:
  workbench:
    # Enable/disable Workbench deployment (default: true).
    # Setting enabled: false suspends Workbench (preserves data).
    # Use teardown: true to permanently delete all Workbench data.
    # See the Workbench Configuration Guide for details.
    enabled: true

    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"
    imagePullPolicy: IfNotPresent
    replicas: 1
    domainPrefix: workbench

    # License configuration
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic

    # Volume for user home directories
    volume:
      create: false
      size: 3Gi

    # Additional volumes (e.g., project data)
    additionalVolumes:
      - pvcName: project-data
        mountPath: /mnt/projects
        readOnly: false

    # Authentication
    auth:
      type: "oidc"
      clientId: "workbench-client-id"
      issuer: "https://idp.example.com"

    # Auto-create user accounts
    createUsersAutomatically: true

    # Admin groups
    adminGroups:
      - workbench-admin
    adminSuperuserGroups:
      - workbench-superadmin

    # Session images
    defaultSessionImage: "ghcr.io/posit-dev/workbench-session:jammy-2024.12.0"
    extraSessionImages:
      - "ghcr.io/posit-dev/workbench-session:gpu-2024.12.0"

    # Node placement
    nodeSelector:
      node-type: posit-products
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "posit"
        effect: "NoSchedule"

    # Session-specific tolerations
    sessionTolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "workbench-sessions"
        effect: "NoSchedule"

    # Databricks integration
    databricks:
      example-workspace:
        name: "Example Workspace"
        url: "https://example-workspace.cloud.databricks.com"
        clientId: "databricks-client-id"

    # Snowflake integration
    snowflake:
      accountId: "abc12345"
      clientId: "snowflake-client-id"

    # VS Code settings
    vsCodeExtensions:
      - "ms-python.python"
      - "quarto.quarto"

    # Positron settings
    positronConfig:
      enabled: 1
      extensions:
        - "posit.positron-r"

    # API settings
    apiSettings:
      workbenchApiEnabled: 1
      workbenchApiAdminEnabled: 1

    # Experimental features
    experimentalFeatures:
      nonRoot: false
      privilegedSessions: false
      sessionServiceAccountName: "custom-session-sa"
      resourceProfiles:
        small:
          name: "Small"
          cpus: "1"
          memMb: "2000"
        large:
          name: "Large"
          cpus: "4"
          memMb: "8000"
```

#### Package Manager

```yaml
spec:
  packageManager:
    # Enable/disable Package Manager deployment (default: true).
    # Setting enabled: false suspends Package Manager (preserves data).
    # Use teardown: true to permanently delete all Package Manager data.
    # See the Package Manager Configuration Guide for details.
    enabled: true

    image: "ghcr.io/posit-dev/package-manager:jammy-2024.08.0"
    imagePullPolicy: IfNotPresent
    replicas: 1
    domainPrefix: packagemanager

    # License configuration
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic

    # Volume for package cache
    volume:
      create: false
      size: 3Gi

    # S3 storage for packages (recommended for production)
    s3Bucket: "my-package-manager-bucket"

    # Azure Files storage (alternative to S3)
    azureFiles:
      storageClassName: "azure-file"
      shareSizeGiB: 100

    # Git SSH keys for private repositories
    gitSSHKeys:
      - secretName: git-ssh-key
        secretKey: id_rsa
```

#### Chronicle (Telemetry)

```yaml
spec:
  chronicle:
    # Enable/disable Chronicle deployment (default: true).
    # Setting enabled: false suspends Chronicle.
    # Use teardown: true to permanently delete all Chronicle data.
    enabled: true

    image: "ghcr.io/posit-dev/chronicle:2024.11.0"
    imagePullPolicy: IfNotPresent

    # S3 storage for telemetry data
    s3Bucket: "my-chronicle-bucket"

    # Chronicle agent image (injected into other products)
    agentImage: "ghcr.io/posit-dev/chronicle-agent:latest"
```

#### Keycloak (Optional IdP)

```yaml
spec:
  keycloak:
    enabled: true
    image: "quay.io/keycloak/keycloak:latest"
    imagePullPolicy: IfNotPresent
```

### Authentication Configuration

Authentication is configured per-product. Each product (Connect and Workbench) has an independent `auth` block where you specify the type and provider-specific settings. This allows Connect and Workbench to use different IdPs or different clients on the same IdP. For detailed configuration and provider-specific examples, see the [Authentication Setup Guide](./authentication-setup.md).

Team Operator supports multiple authentication methods:

#### OIDC Authentication

```yaml
spec:
  connect:
    auth:
      type: "oidc"
      clientId: "connect-client-id"
      issuer: "https://idp.example.com"
      groups: true
      usernameClaim: "preferred_username"
      emailClaim: "email"
      groupsClaim: "groups"
      scopes:
        - "openid"
        - "profile"
        - "email"
      # Role mappings
      viewerRoleMapping:
        - "connect-viewers"
      publisherRoleMapping:
        - "connect-publishers"
      administratorRoleMapping:
        - "connect-admins"
```

#### SAML Authentication

```yaml
spec:
  workbench:
    auth:
      type: "saml"
      samlMetadataUrl: "https://idp.example.com/metadata"
      samlIdPAttributeProfile: "azure"  # or custom attribute mappings
      samlUsernameAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
      samlEmailAttribute: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
```

#### Password Authentication

```yaml
spec:
  connect:
    auth:
      type: "password"
```

### Database Configuration

All stateful products — Connect, Workbench, and Package Manager — require PostgreSQL. The operator provisions separate database schemas for each product using the credentials you provide in `mainDatabaseCredentialSecret`. Database URLs are derived automatically from the workload secret configuration; you do not specify them directly in the Site spec.

```yaml
spec:
  # Database credentials from AWS Secrets Manager
  mainDatabaseCredentialSecret:
    type: "aws"
    vaultName: "rds!db-example-database-id"

  # Drop databases when Site is deleted (use with caution!)
  dropDatabaseOnTearDown: false
```

### Image Pull Configuration

```yaml
spec:
  # Image pull secrets (must exist in namespace)
  imagePullSecrets:
    - "regcred"
    - "ghcr-secret"

  # Disable pre-pull daemonset
  disablePrePullImages: false
```

### Extra Service Accounts

Create additional service accounts for custom workloads:

```yaml
spec:
  extraSiteServiceAccounts:
    - nameSuffix: "custom-jobs"
      annotations:
        eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/CustomJobsRole"
```

## Common Site Configurations

The examples below show two complete configurations: a minimal development setup using password authentication and Kubernetes secrets, and a production setup with OIDC, FSx storage, and AWS Secrets Manager. Use the minimal configuration to get up and running quickly; use the production configuration as a starting template for environments that need SSO and cloud-native storage.

### Minimal Development Site

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: dev
  namespace: posit-team
spec:
  domain: dev.example.com
  secret:
    type: "kubernetes"
    vaultName: "dev-secrets"
  mainDatabaseCredentialSecret:
    type: "kubernetes"
    vaultName: "dev-db-creds"
  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic
  connect:
    image: ghcr.io/posit-dev/connect:ubuntu22-2024.10.0
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pc.lic
    auth:
      type: "password"
  workbench:
    image: ghcr.io/posit-dev/workbench:jammy-2024.12.0
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic
    auth:
      type: "password"
```

### Production Site with OIDC and S3 Storage

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: production
  namespace: posit-team
spec:
  domain: posit.example.com
  awsAccountId: "123456789012"
  clusterDate: "20240101"

  secret:
    type: "aws"
    vaultName: "production-site-secrets"
  mainDatabaseCredentialSecret:
    type: "aws"
    vaultName: "rds!db-production-id"

  volumeSource:
    type: fsx-zfs
    volumeId: fsvol-abcdef123456
    dnsName: fs-abcdef123456.fsx.us-east-1.amazonaws.com

  sharedDirectory: shared

  ingressClass: traefik
  ingressAnnotations:
    traefik.ingress.kubernetes.io/router.middlewares: kube-system-forward-auth@kubernetescrd

  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    replicas: 2
    s3Bucket: "production-ppm-bucket"
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic

  connect:
    image: ghcr.io/posit-dev/connect:ubuntu22-2024.10.0
    replicas: 2
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pc.lic
    auth:
      type: "oidc"
      clientId: "connect-prod"
      issuer: "https://idp.example.com"
      groups: true

  workbench:
    image: ghcr.io/posit-dev/workbench:jammy-2024.12.0
    replicas: 2
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: pw.lic
    auth:
      type: "oidc"
      clientId: "workbench-prod"
      issuer: "https://idp.example.com"
    createUsersAutomatically: true
    adminGroups:
      - posit-admins

  chronicle:
    image: ghcr.io/posit-dev/chronicle:2024.11.0
    s3Bucket: "production-chronicle-bucket"

  dropDatabaseOnTearDown: false
```

## Troubleshooting

Start by checking the Site status and operator logs. The `kubectl describe` output includes status conditions and recent events that usually point directly to the problem.

```bash
# List all Sites
kubectl get sites -n posit-team

# Describe a Site
kubectl describe site <site-name> -n posit-team

# View Site controller logs
kubectl logs -n posit-team -l app.kubernetes.io/name=team-operator --tail=100
```

For deeper troubleshooting, see the [Troubleshooting Guide](./troubleshooting.md). The quick reference below covers issues specific to the Site lifecycle.

#### If products are not deploying

Check whether the operator created the child product CRs. If they don't exist, the problem is in the Site reconciliation loop. If they exist but pods are not running, the problem is in the individual product controller.

```bash
kubectl logs -n posit-team deploy/team-operator | grep -i error
kubectl get connect,workbench,packagemanager,chronicle -n posit-team
```

#### If database connections are failing

Verify the credential secret exists and contains the expected keys, then confirm the database host is reachable from within the cluster and that the SSL mode matches your database server configuration.

```bash
kubectl get secret <secret-name> -n posit-team
```

#### If volume provisioning is failing

For FSx volumes, double-check the volume ID and DNS name. Then inspect the subdirectory provisioning job for errors:

```bash
kubectl get jobs -n posit-team | grep subdir
kubectl logs job/<site-name>-subdir-creator -n posit-team
```

#### If ingress is not routing traffic

Confirm the ingress class matches your controller, that ingress resources were created, and that DNS records resolve to your ingress controller:

```bash
kubectl get ingress -n posit-team
```

#### If authentication is failing

For OIDC, verify the client ID and issuer URL. Confirm redirect URIs are registered in your IdP and review product logs for detailed error messages:

```bash
kubectl logs -n posit-team deploy/<site-name>-connect
```

#### If the operator is in a constant reconciliation loop

Compare the live Site spec against your source manifest to look for fields that may be mutating on each reconcile. External processes modifying resources can also trigger loops.

```bash
kubectl get site <site-name> -o yaml | diff - site.yaml
```

## Related Documentation

- [Team Operator Overview](../README.md)
- [Adding Config Options](adding-config-options.md) - For contributors extending Site configuration
