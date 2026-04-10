---
title: Package Manager Configuration
description: How to configure Posit Package Manager within Team Operator
---

# Package Manager Configuration Guide

Posit Package Manager (PPM) gives your data science platform a controlled, reproducible source for R and Python packages. Rather than fetching packages directly from CRAN, PyPI, or Bioconductor, users install from Package Manager, which mirrors those sources internally and can also build packages from private Git repositories. This improves reproducibility, speeds up installs in air-gapped or bandwidth-constrained environments, and gives administrators visibility into what packages are in use.

When you enable Package Manager in a Site resource, the operator deploys the server, provisions database schemas, configures storage, and wires up ingress. The most common configuration decisions are: choosing a storage backend (S3 for AWS, Azure Files for Azure, or a local PVC for development), providing a license, and setting up Git SSH keys if you need to build packages from private repositories.

## Overview

Posit Package Manager provides R and Python packages from CRAN, Bioconductor, and PyPI, plus internal packages built from Git repositories. In Team Operator, Package Manager is deployed as a child resource of a Site.

### Architecture

```
Site CR
  └── PackageManager CR (created automatically)
        ├── Deployment (rspm container)
        ├── Service (ClusterIP)
        ├── Ingress
        ├── ConfigMap (rstudio-pm.gcfg)
        ├── ServiceAccount
        ├── PersistentVolumeClaim (optional)
        ├── SecretProviderClass (for AWS secrets)
        └── PodDisruptionBudget
```

When you configure Package Manager in a Site spec, the Site controller creates a `PackageManager` Custom Resource. The PackageManager controller reconciles the Kubernetes resources needed to run the service.

## Basic Configuration

This section covers the foundational fields: lifecycle controls, the container image, replica count, domain prefix, licensing, volume, and storage backend.

### Enabling/Disabling Package Manager

Package Manager can be suspended or permanently torn down using the `enabled` and `teardown` fields.

#### Suspending Package Manager (non-destructive)

Setting `enabled: false` suspends Package Manager: the Deployment, Service, and Ingress are removed, but the PVC, database, and secrets are preserved. Re-enabling restores full service with all existing data intact.

```yaml
spec:
  packageManager:
    enabled: false   # suspend — data is preserved
```

**When to use `enabled: false`:**

- Customer does not have a Package Manager license yet — deploy the site without Package Manager and enable it once a license is purchased
- Temporarily pause Package Manager during a maintenance window or cost-saving period
- Stop Package Manager while retaining all package data and configuration for a possible return

**Re-enabling Package Manager** after a suspend is as simple as removing the field or setting it back to `true`:

```yaml
spec:
  packageManager:
    enabled: true   # or omit the field entirely — defaults to true
```

#### Tearing down Package Manager (destructive)

To permanently destroy all Package Manager resources — including the database, secrets, and PVC — set both `enabled: false` and `teardown: true`:

```yaml
spec:
  packageManager:
    enabled: false
    teardown: true   # DESTRUCTIVE: deletes database, secrets, and PVC
```

**This is irreversible.** Re-enabling Package Manager after a teardown starts completely fresh with a new empty database and no prior package repositories or configuration.

**When to use `teardown: true`:**

- Permanently decommissioning Package Manager with no intent to restore data
- Reclaiming cluster storage after migrating to a different Package Manager instance
- Explicitly wiping Package Manager to start fresh

> **Note:** `teardown: true` has no effect while `enabled` is `true` or unset. You must set `enabled: false` first.

---

### Minimal Configuration

The minimal configuration requires only an image and a license. Add a storage backend (`s3Bucket` or `azureFiles`) for production use.

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: example
  namespace: posit-team
spec:
  domain: example.mycompany.com
  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic
```

### Full Configuration Reference

The table below covers all top-level Package Manager fields. Detailed explanations of storage, SSH keys, and database settings follow in subsequent sections.

```yaml
spec:
  packageManager:
    # Container image (required)
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0

    # Image pull policy: Always, IfNotPresent, Never
    imagePullPolicy: IfNotPresent

    # Number of replicas (default: 1)
    replicas: 2

    # URL subdomain prefix (default: packagemanager)
    domainPrefix: packagemanager

    # License configuration (required)
    license:
      type: FILE                      # FILE or KEY
      existingSecretName: license     # Name of existing K8s secret
      existingSecretKey: ppm.lic      # Key within the secret

    # Volume for local package cache
    volume:
      create: true
      size: 10Gi
      storageClassName: gp3
      accessModes:
        - ReadWriteOnce

    # S3 storage backend (recommended for production)
    s3Bucket: my-package-manager-bucket

    # Azure Files storage backend (alternative to S3)
    azureFiles:
      storageClassName: azure-file
      shareSizeGiB: 100

    # Git SSH keys for private repository access
    gitSSHKeys:
      - name: github
        host: github.com
        secretRef:
          source: aws-secrets-manager
          name: github
      - name: gitlab
        host: gitlab.company.com
        secretRef:
          source: kubernetes
          name: gitlab-ssh
          key: private-key

    # Node placement
    nodeSelector:
      node-type: posit-products

    # Additional environment variables
    addEnv:
      RSPM_ADDRESS: "https://packagemanager.example.com"
```

## License Configuration

Package Manager requires a valid license to start. The operator supports three ways to provide it: a Kubernetes Secret containing the license file, a license key string, or a secret stored in AWS Secrets Manager. File-based licenses are recommended for production.

The license can be provided in the following ways:

### File License (Recommended)

Store your license file in a Kubernetes secret:

```bash
kubectl create secret generic license \
  --from-file=ppm.lic=/path/to/license.lic \
  -n posit-team
```

Then reference it in the Site spec:

```yaml
spec:
  packageManager:
    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic
```

### Key License

For license keys (activation keys):

```yaml
spec:
  packageManager:
    license:
      type: KEY
      key: "XXXX-XXXX-XXXX-XXXX"
```

### AWS Secrets Manager License

When using AWS secret management, the license is stored in the site's secret vault:

```yaml
spec:
  secret:
    type: aws
    vaultName: "my-site-secrets.posit.team"
  packageManager:
    license:
      type: FILE
```

The license is expected in the vault under the key `pkg-license`.

## Database Configuration

Package Manager uses PostgreSQL for application metadata and usage metrics. The operator constructs database connection strings automatically from your Site's database credentials secret — you do not need to supply connection details manually. The operator provisions two schemas:

| Schema | Purpose |
|--------|---------|
| `pm` | Main application data |
| `metrics` | Usage data and analytics |

### Database Connection

Database credentials are configured at the Site level and propagated automatically to Package Manager. Reference an AWS Secrets Manager vault or a Kubernetes Secret containing your database credentials.

```yaml
spec:
  # Database credentials from AWS Secrets Manager
  mainDatabaseCredentialSecret:
    type: aws
    vaultName: "rds!db-example-database-id"

  # Or from Kubernetes secrets
  mainDatabaseCredentialSecret:
    type: kubernetes
    vaultName: my-db-credentials
```

### Database URLs

The operator constructs database URLs using this format:

```
postgres://username:password@host/database?search_path=pm&sslmode=require
postgres://username:password@host/database?search_path=metrics&sslmode=require
```

### SSL Mode

SSL mode is configured at the Site level through the database config:

```yaml
spec:
  mainDatabaseCredentialSecret:
    type: aws
    vaultName: "rds!db-mydb"
```

The operator uses `require` SSL mode by default for production deployments.

## Storage Backends

Package Manager stores package binaries and metadata on a storage backend. Choose the backend that matches your infrastructure: S3 for AWS deployments, Azure Files for Azure deployments, or a local PVC for development and testing. S3 and Azure Files are recommended for production because they scale independently of the Package Manager pod.

Package Manager supports the following storage backends.

### S3 Storage (AWS Recommended)

For production deployments on AWS, use S3 storage. The operator configures IRSA (IAM Roles for Service Accounts) automatically — the Package Manager ServiceAccount receives an annotation with the appropriate IAM role ARN.

```yaml
spec:
  packageManager:
    s3Bucket: my-ppm-bucket
```

This generates the following configuration:

```ini
[Storage]
Default = S3

[S3Storage]
Bucket = my-ppm-bucket
Prefix = <site-name>/ppm-v0
```

#### IAM Permissions

The Package Manager service account requires the following S3 permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-ppm-bucket",
        "arn:aws:s3:::my-ppm-bucket/<site-name>/ppm-v0/*"
      ]
    }
  ]
}
```

#### IAM Role Association

The operator creates a ServiceAccount with the IAM role annotation:

```yaml
annotations:
  eks.amazonaws.com/role-arn: arn:aws:iam::<account-id>:role/pkg.<cluster-date>.<site-name>.<workload-name>.posit.team
```

### Azure Files Storage

For Azure deployments, use Azure Files:

```yaml
spec:
  packageManager:
    azureFiles:
      storageClassName: azure-file
      shareSizeGiB: 100   # Minimum 100 GiB required
```

This creates a PersistentVolumeClaim with:
- `ReadWriteMany` access mode
- Dynamic provisioning via the Azure Files CSI driver
- Mount path at `/mnt/azure-files`

#### Azure Files Prerequisites

1. Create an Azure Storage Account
2. Create a StorageClass that uses the Azure Files CSI driver:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azure-file
provisioner: file.csi.azure.com
parameters:
  skuName: Premium_LRS
  protocol: nfs
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

3. Configure workload identity or storage account credentials for the CSI driver.

### Local Volume Storage

For development or small deployments without a cloud storage backend, use a local persistent volume. This is not recommended for production — the volume is tied to the pod's lifecycle and does not scale with the deployment.

```yaml
spec:
  packageManager:
    volume:
      create: true
      size: 10Gi
      storageClassName: gp3
      accessModes:
        - ReadWriteOnce
```

## Git Builder Configuration

Package Manager can build R packages directly from Git repositories, making it possible to serve internal packages alongside public CRAN and PyPI mirrors. Private repositories require SSH key authentication; configure one entry in `gitSSHKeys` per host you need to access.

### SSH Key Configuration

SSH keys are configured in the `gitSSHKeys` array:

```yaml
spec:
  packageManager:
    gitSSHKeys:
      - name: github           # Unique identifier
        host: github.com       # Git host domain
        secretRef:
          source: aws-secrets-manager
          name: github         # Key name in AWS secret
      - name: gitlab-internal
        host: gitlab.internal.com
        secretRef:
          source: kubernetes
          name: gitlab-ssh-key
          key: id_rsa
```

### AWS Secrets Manager SSH Keys

When using AWS Secrets Manager, SSH keys are stored in a dedicated vault following a naming convention. The operator creates a `SecretProviderClass` and mounts each key as a file at `/mnt/ssh-keys/<name>`.

**Vault naming convention:**
```
{workloadCompoundName}-{siteName}-ssh-ppm-keys.posit.team
```

Example vault structure:
```json
{
  "github": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----",
  "gitlab": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"
}
```

The operator creates:
1. A `SecretProviderClass` for the SSH secrets
2. CSI volume mounts for each SSH key at `/mnt/ssh-keys/<name>`

### Kubernetes Secret SSH Keys

For clusters not using AWS Secrets Manager, store SSH keys as Kubernetes Secrets and reference them directly.

```bash
# Create the SSH key secret
kubectl create secret generic gitlab-ssh-key \
  --from-file=id_rsa=/path/to/private/key \
  -n posit-team
```

```yaml
spec:
  packageManager:
    gitSSHKeys:
      - name: gitlab
        host: gitlab.company.com
        secretRef:
          source: kubernetes
          name: gitlab-ssh-key
          key: id_rsa
```

### Passphrase-Protected Keys

If your SSH keys are encrypted with a passphrase, reference the passphrase secret alongside the key. The operator passes it to the SSH agent at startup.

```yaml
spec:
  packageManager:
    gitSSHKeys:
      - name: secure-git
        host: secure-git.company.com
        secretRef:
          source: aws-secrets-manager
          name: secure-git
        passphraseSecretRef:
          source: aws-secrets-manager
          name: secure-git-passphrase
```

### Git Build Settings

Enable unsandboxed Git builds (required for many build scenarios):

```yaml
# This is configured automatically by the operator
# The generated rstudio-pm.gcfg will contain:
[Git]
AllowUnsandboxedGitBuilds = true
```

## Package Repository Configuration

The operator pre-configures default repository names for CRAN, Bioconductor, and PyPI in the generated `rstudio-pm.gcfg`. These names determine the URL paths under which packages are served. You can adjust them through the Package Manager admin interface after deployment.

```yaml
# Generated configuration
[Repos]
PyPI = pypi
CRAN = cran
Bioconductor = bioconductor
```

### R Version Configuration

R versions available for building packages:

```yaml
# Generated configuration (default)
[Server]
RVersion = /opt/R/default
```

## Secret Management

Package Manager requires three secrets at runtime: the license, an encryption key for sensitive data, and the database password. The operator retrieves these from the secret backend you configured at the Site level — either AWS Secrets Manager or a Kubernetes Secret.

### AWS Secrets Manager

When `secret.type: aws`, these secrets are retrieved from AWS Secrets Manager:

| Secret Key | Purpose |
|------------|---------|
| `pkg-license` | Package Manager license file |
| `pkg-secret-key` | Encryption key for sensitive data |
| `pkg-db-password` | Database password |

These are stored in the Site's vault (configured via `secret.vaultName`).

### Kubernetes Secrets

When `secret.type: kubernetes`, secrets are retrieved from Kubernetes:

```yaml
spec:
  secret:
    type: kubernetes
    vaultName: my-site-secrets
```

Expected secret keys:
- `pkg-secret-key`: Encryption key
- `pkg-db-password`: Database password

## Resource Configuration

The operator applies default resource requests and limits to the Package Manager pod. For most deployments these defaults are appropriate; adjust them if your Package Manager serves a large number of users or hosts a very large package mirror.

### Default Resources

The operator applies these resource limits by default:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 2Gi
    ephemeral-storage: 500Mi
  limits:
    cpu: 2000m
    memory: 4Gi
    ephemeral-storage: 2Gi
```

### Pod Disruption Budget

A PodDisruptionBudget is created to maintain availability during cluster maintenance. For single-replica deployments, `minAvailable: 0`. For multi-replica deployments, the operator calculates an appropriate `minAvailable` value.

### Affinity

Pods are scheduled with anti-affinity to distribute replicas across nodes:

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 1
        podAffinityTerm:
          topologyKey: kubernetes.io/hostname
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/instance
                operator: In
                values: ["<site>-packagemanager"]
```

## Example Configurations

### AWS Production Deployment

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
  workloadCompoundName: my-workload

  secret:
    type: aws
    vaultName: production-site-secrets.posit.team

  mainDatabaseCredentialSecret:
    type: aws
    vaultName: "rds!db-production-id"

  ingressClass: traefik
  ingressAnnotations:
    traefik.ingress.kubernetes.io/router.middlewares: kube-system-forward-auth@kubernetescrd

  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    imagePullPolicy: IfNotPresent
    replicas: 2

    license:
      type: FILE

    # S3 for package storage
    s3Bucket: production-ppm-packages

    # Git SSH keys for private repos
    gitSSHKeys:
      - name: github
        host: github.com
        secretRef:
          source: aws-secrets-manager
          name: github
      - name: gitlab
        host: gitlab.company.com
        secretRef:
          source: aws-secrets-manager
          name: gitlab

    nodeSelector:
      node-type: posit-products

    addEnv:
      RSPM_LOG_LEVEL: info
```

### Azure Deployment

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: azure-site
  namespace: posit-team
spec:
  domain: posit.azurecompany.com

  secret:
    type: kubernetes
    vaultName: azure-site-secrets

  mainDatabaseCredentialSecret:
    type: kubernetes
    vaultName: azure-db-creds

  volumeSource:
    type: azure-netapp

  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    replicas: 2

    license:
      type: FILE
      existingSecretName: ppm-license
      existingSecretKey: license.lic

    # Azure Files for package storage
    azureFiles:
      storageClassName: azure-file-premium
      shareSizeGiB: 500

    # Kubernetes-native SSH keys
    gitSSHKeys:
      - name: azure-devops
        host: ssh.dev.azure.com
        secretRef:
          source: kubernetes
          name: azure-devops-ssh
          key: id_rsa
```

### Development/Testing Deployment

```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: dev
  namespace: posit-team
spec:
  domain: dev.example.com
  debug: true

  secret:
    type: kubernetes
    vaultName: dev-secrets

  mainDatabaseCredentialSecret:
    type: kubernetes
    vaultName: dev-db-creds

  packageManager:
    image: ghcr.io/posit-dev/package-manager:jammy-2024.08.0
    replicas: 1

    license:
      type: FILE
      existingSecretName: license
      existingSecretKey: ppm.lic

    # Local volume for development
    volume:
      create: true
      size: 5Gi
      storageClassName: standard
```

## Troubleshooting

### Viewing Package Manager Resources

```bash
# List Package Manager CRs
kubectl get packagemanagers -n posit-team

# Describe Package Manager
kubectl describe packagemanager <site-name> -n posit-team

# View Package Manager pods
kubectl get pods -n posit-team -l app.kubernetes.io/name=package-manager

# View Package Manager logs
kubectl logs -n posit-team -l app.kubernetes.io/name=package-manager --tail=100

# View generated configuration
kubectl get configmap <site-name>-packagemanager -n posit-team -o yaml
```

### Common Issues

#### Pod Stuck in CrashLoopBackOff

1. Check logs for startup errors:
   ```bash
   kubectl logs -n posit-team deploy/<site-name>-packagemanager --previous
   ```

2. Verify license is valid and accessible:
   ```bash
   # For Kubernetes secrets
   kubectl get secret license -n posit-team
   ```

3. Check database connectivity:
   ```bash
   kubectl exec -it deploy/<site-name>-packagemanager -n posit-team -- \
     /bin/bash -c 'echo "SELECT 1" | psql $PACKAGEMANAGER_POSTGRES_URL'
   ```

#### S3 Access Denied

1. Verify the IAM role is correctly associated:
   ```bash
   kubectl get sa <site-name>-packagemanager -n posit-team -o yaml | grep role-arn
   ```

2. Check the bucket policy allows the role.

3. Verify the bucket prefix is correct (`<site-name>/ppm-v0`).

#### SSH Keys Not Working

1. Verify the SecretProviderClass exists:
   ```bash
   kubectl get secretproviderclass <site-name>-packagemanager-ssh-secrets -n posit-team
   ```

2. Check the SSH key is mounted:
   ```bash
   kubectl exec -it deploy/<site-name>-packagemanager -n posit-team -- \
     ls -la /mnt/ssh-keys/
   ```

3. Verify the SSH key permissions and format.

#### Azure Files PVC Pending

1. Check the StorageClass exists:
   ```bash
   kubectl get sc azure-file
   ```

2. Verify the CSI driver is installed:
   ```bash
   kubectl get pods -n kube-system | grep csi-azurefile
   ```

3. Check PVC events:
   ```bash
   kubectl describe pvc <site-name>-packagemanager-azure-files -n posit-team
   ```

### Debug Mode

Enable debug logging for detailed troubleshooting:

```yaml
spec:
  debug: true  # Site-level debug
  packageManager:
    addEnv:
      RSPM_LOG_LEVEL: debug
```

The generated config will include:

```ini
[Debug]
Log = verbose
```

### Sleep Mode for Debugging

To debug crash loops, enable sleep mode:

```yaml
# Directly on PackageManager CR (not recommended for production)
apiVersion: core.posit.team/v1beta1
kind: PackageManager
metadata:
  name: example
  namespace: posit-team
spec:
  sleep: true
```

This changes the container command to `sleep infinity`, allowing you to exec into the container for debugging.

## Configuration Reference

The following sections describe the configuration file the operator generates and the environment variables it injects at runtime. These are provided for reference when troubleshooting or auditing the deployment — you do not need to manage these directly.

### Generated Config File (rstudio-pm.gcfg)

The operator generates a configuration file from the Site spec and mounts it into the Package Manager pod. The sections below represent a typical production configuration:

```ini
[Server]
Address = https://packagemanager.example.com
RVersion = /opt/R/default
LauncherDir = /var/lib/rstudio-pm/launcher_internal
AccessLogFormat = common

[Http]
Listen = :4242

[Git]
AllowUnsandboxedGitBuilds = true

[Database]
Provider = postgres

[Postgres]
URL = postgres://user:pass@host/db?search_path=pm&sslmode=require
UsageDataURL = postgres://user:pass@host/db?search_path=metrics&sslmode=require

[Metrics]
Enabled = true

[Repos]
PyPI = pypi
CRAN = cran
Bioconductor = bioconductor

[Storage]
Default = S3

[S3Storage]
Bucket = my-bucket
Prefix = site-name/ppm-v0

[Debug]
Log = verbose
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `PACKAGEMANAGER_SECRET_KEY` | Encryption key for sensitive data |
| `PACKAGEMANAGER_POSTGRES_PASSWORD` | Database password |
| `PACKAGEMANAGER_POSTGRES_USAGEDATAPASSWORD` | Metrics database password |
| `RSPM_LICENSE_FILE_PATH` | Path to license file |

### Kubernetes Labels

All Package Manager resources are labeled with:

```yaml
app.kubernetes.io/managed-by: team-operator
app.kubernetes.io/name: package-manager
app.kubernetes.io/instance: <site-name>-packagemanager
posit.team/site: <site-name>
posit.team/component: package-manager
```

## Related Documentation

- [Site Management Guide](product-team-site-management.md) - Complete Site configuration reference
- [Adding Config Options](adding-config-options.md) - Extending Package Manager configuration
- [Posit Package Manager Documentation](https://docs.posit.co/rspm/) - Official product documentation
