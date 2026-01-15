# Site Management Guide

This guide covers the management of Site resources in Team Operator for platform engineers deploying Posit Team.

## Overview

The `Site` Custom Resource Definition (CRD) is the **single source of truth** for a Posit Team deployment. A Site represents a complete deployment environment that includes:

- **Flightdeck** - Landing page dashboard
- **Connect** - Publishing and sharing platform
- **Workbench** - Interactive development environment
- **Package Manager** - Package repository management
- **Chronicle** - Telemetry and monitoring
- **Keycloak** - Authentication and identity management (optional)

When you create or update a Site, the Site controller automatically reconciles all child product Custom Resources (Connect, Workbench, Package Manager, Chronicle, Flightdeck) to match your desired configuration.

## Site Lifecycle

### Creating a Site

To create a new Posit Team deployment, apply a Site manifest:

```bash
kubectl apply -f site.yaml -n posit-team
```

When a Site is created, the Site controller:

1. Provisions storage volumes (FSx, NFS, or Azure NetApp based on configuration)
2. Creates subdirectory provisioning jobs for shared storage
3. Reconciles the Flightdeck landing page
4. Creates Connect, Workbench, Package Manager, and Chronicle CRs
5. Sets up network policies for product communication
6. Creates any extra service accounts specified

### Updating Site Configuration

To update a Site:

```bash
kubectl edit site <site-name> -n posit-team
```

Or apply an updated manifest:

```bash
kubectl apply -f site.yaml -n posit-team
```

The Site controller detects changes and propagates them to child product CRs. Product controllers then reconcile their respective deployments.

**Configuration Flow:**
```
Site spec change
    -> Site controller reconciles
        -> Product CRs updated (Connect, Workbench, PM, Chronicle, Flightdeck)
            -> Product controllers reconcile
                -> Deployments, Services, Ingress updated
```

### Deleting a Site

When you delete a Site:

```bash
kubectl delete site <site-name> -n posit-team
```

The Site controller cleans up:

1. Connect CR and all its resources
2. Workbench CR and all its resources
3. Package Manager CR and all its resources
4. Flightdeck CR and all its resources
5. Network policies

**Important:** Child resources have owner references to the Site, so Kubernetes garbage collection handles most cleanup automatically.

If `dropDatabaseOnTearDown: true` is set, product databases will be dropped during cleanup.

## Site Spec Structure

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

Team Operator supports multiple secret backends:

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

**Secret Types:**

| Type | Description |
|------|-------------|
| `kubernetes` | Standard Kubernetes Secrets |
| `aws` | AWS Secrets Manager |

### Storage Configuration

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

All stateful products (Connect, Workbench, Package Manager) use PostgreSQL:

```yaml
spec:
  # Database credentials from AWS Secrets Manager
  mainDatabaseCredentialSecret:
    type: "aws"
    vaultName: "rds!db-example-database-id"

  # Drop databases when Site is deleted (use with caution!)
  dropDatabaseOnTearDown: false
```

Database URLs are determined automatically from the workload secret configuration.

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

### Viewing Site Status

```bash
# List all Sites
kubectl get sites -n posit-team

# Describe a Site
kubectl describe site <site-name> -n posit-team

# View Site controller logs
kubectl logs -n posit-team -l app.kubernetes.io/name=team-operator --tail=100
```

### Common Issues

#### Products Not Deploying

1. Check Site controller logs for errors:
   ```bash
   kubectl logs -n posit-team deploy/team-operator | grep -i error
   ```

2. Verify product CRs were created:
   ```bash
   kubectl get connect,workbench,packagemanager,chronicle -n posit-team
   ```

3. Check individual product controller logs if CRs exist but pods are not running.

#### Database Connection Failures

1. Verify database credential secret exists and is accessible:
   ```bash
   # For Kubernetes secrets
   kubectl get secret <secret-name> -n posit-team

   # For AWS Secrets Manager, check operator logs for fetch errors
   ```

2. Ensure database host is reachable from the cluster.

3. Check SSL mode configuration matches your database server.

#### Volume Provisioning Issues

1. For FSx volumes, verify the volume ID and DNS name are correct.

2. Check subdirectory provisioning job:
   ```bash
   kubectl get jobs -n posit-team | grep subdir
   kubectl logs job/<site-name>-subdir-creator -n posit-team
   ```

3. Verify storage class exists for your volume type.

#### Ingress Not Working

1. Verify ingress class is correct and controller is running.

2. Check ingress resources were created:
   ```bash
   kubectl get ingress -n posit-team
   ```

3. Verify DNS records point to your ingress controller.

#### Authentication Failures

1. For OIDC, verify client ID and issuer URL are correct.

2. Check that redirect URIs are configured in your IdP.

3. Review product logs for detailed auth error messages:
   ```bash
   kubectl logs -n posit-team deploy/<site-name>-connect
   ```

### Reconciliation Loop Detection

If you notice constant reconciliation:

1. Check for spec fields that might be mutating:
   ```bash
   kubectl get site <site-name> -o yaml | diff - site.yaml
   ```

2. Look for validation errors in controller logs.

3. Ensure no external processes are modifying resources.

## Related Documentation

- [Team Operator Overview](../README.md)
- [Adding Config Options](adding-config-options.md) - For contributors extending Site configuration
