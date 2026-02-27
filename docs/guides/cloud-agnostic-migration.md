# Cloud-Agnostic Migration Runbook

> **DRAFT** — The PTD CLI minimum version for cloud-agnostic features has not been finalized. Do not proceed until the PTD CLI version is confirmed and this notice is removed.

This guide provides step-by-step instructions for migrating existing clusters to use cloud-agnostic Team Operator features. These features eliminate cloud-specific code from the operator by using Kubernetes-native APIs and infrastructure-layer abstractions.

## Overview

### What's Changing

The Team Operator is being updated to interact exclusively with Kubernetes-native APIs instead of calling cloud provider SDKs directly. This migration removes:

- Direct FSx/NFS/NetApp volume provisioning (replaced with StorageClass references)
- AWS Secrets Manager SDK calls and SecretProviderClass creation (replaced with K8s Secrets via external-secrets-operator)
- Hardcoded IRSA ARN computation (replaced with EKS Pod Identity on AWS or passthrough annotations for Azure)
- Traefik-specific Middleware CRDs and Ingress annotations (replaced with HTTPRoute via Gateway API)

After this migration, the operator will work on any Kubernetes cluster (AWS, Azure, kind, etc.) without requiring cloud-specific code changes.

### Feature Flags

The migration is controlled by feature flags in PTD's cluster configuration (`ptd.yaml`):

```yaml
clusters:
  "YYYYMMDD":
    spec:
      # Track 1: Storage
      enable_nfs_subdir_provisioner: true

      # Track 2: IAM
      enable_pod_identity_agent: true  # AWS only

      # Track 3: Secrets
      enable_external_secrets_operator: true

      # Track 4: Ingress (optional)
      enable_gateway_api: true
```

Each flag can be enabled independently, allowing incremental rollout.

### Rollout Order

**Critical**: Always test on staging clusters before production.

Recommended order:
1. Enable on a single staging cluster (e.g., `ganso01-staging`)
2. Verify all products work correctly
3. Enable on remaining staging clusters
4. Monitor for 24-48 hours
5. Roll out to production clusters one at a time

## Prerequisites

Before beginning migration on any cluster, verify the following:

### Required Versions

| Component | Minimum Version | Notes |
|-----------|----------------|-------|
| Team Operator | 1.16.0+ | Contains Phase 1 dual-path fields |
| PTD CLI | TBD (see note) | Contains infrastructure + wiring for cloud-agnostic features |
| EKS Version | 1.24+ | Required for EKS Pod Identity Agent (AWS only) |
| Traefik | v3.1+ | Required for Gateway API HTTPRoute session persistence |

**Note**: The PTD CLI minimum version for cloud-agnostic features has not been finalized. This document is a draft — do not proceed until the PTD CLI version is confirmed and this note is removed.

Check current versions:

```bash
# Team operator version
export AWS_PROFILE=ptd-staging
ptd workon ganso01-staging -- kubectl get deployment team-operator-controller-manager -n posit-team-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# EKS version (AWS)
aws eks describe-cluster --name <cluster-name> --query 'cluster.version'

# Traefik version
ptd workon ganso01-staging -- kubectl get deployment traefik -n traefik -o jsonpath='{.spec.template.spec.containers[0].image}'
```

### AWS Traefik Upgrade (AWS Only)

**AWS clusters require Traefik v2 → v3 upgrade before enabling Gateway API.**

Current state:
- AWS: Traefik chart v24.0.0 (Traefik v2.x)
- Azure: Traefik chart v33.2.1 (Traefik v3.x) - already compatible

The Traefik v2 → v3 upgrade has breaking changes (middleware API, entrypoint configuration). This must be completed as a separate task before migrating Track 4 (Gateway API) on AWS clusters.

Azure clusters can proceed with Gateway API migration immediately.

## Step-by-Step: Enable on a Staging Cluster

The following steps walk through enabling all four tracks on a single staging cluster. You can enable tracks independently, but the recommended order is: IAM → Secrets → Storage → Gateway API.

### Step 1: Enable Pod Identity (AWS Only)

**What it does**: Installs EKS Pod Identity Agent addon and creates Pod Identity associations for product ServiceAccounts. This replaces IRSA (IAM Roles for ServiceAccounts) with EKS Pod Identity, which doesn't require ServiceAccount annotations.

**Prerequisites**:
- EKS cluster version 1.24+
- IAM roles need trust policy updates (from OIDC to `pods.eks.amazonaws.com`)
- PTD must know ServiceAccount names (set explicitly via `serviceAccountName` in CR)

**Update `ptd.yaml`**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_pod_identity_agent: true
```

**Apply changes**:

```bash
export AWS_PROFILE=ptd-staging
ptd ensure ganso01-staging
```

**Verify**:

```bash
# Check EKS Pod Identity Agent addon installed
aws eks describe-addon --cluster-name <cluster-name> --addon-name eks-pod-identity-agent

# Check Pod Identity associations created
aws eks list-pod-identity-associations --cluster-name <cluster-name>

# Verify pods running with correct ServiceAccounts
ptd workon ganso01-staging -- kubectl get pods -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.serviceAccountName}{"\n"}{end}'

# Check product pods have access to AWS resources (e.g., S3 for Connect)
ptd workon ganso01-staging -- kubectl logs -n posit-team -l app=connect --tail=50 | grep -i "aws\|s3\|iam"
```

**Expected ServiceAccount names** (if not overridden in CR):
- `{site_name}-connect`
- `{site_name}-workbench`
- `{site_name}-packagemanager`
- `{site_name}-chronicle`
- `{site_name}-home` (flightdeck)

**Rollback**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_pod_identity_agent: false
```

Then run `ptd ensure ganso01-staging` to revert to IRSA. Operator automatically falls back to computing IRSA ARN annotations when Pod Identity associations are not present.

### Step 2: Enable external-secrets-operator

**What it does**: Deploys external-secrets-operator to sync secrets from AWS Secrets Manager (or Azure Key Vault) into Kubernetes Secrets. The operator then mounts these K8s Secrets directly instead of calling AWS SDK or using SecretProviderClass.

**Prerequisites**:
- IAM role for external-secrets-operator ServiceAccount (created by PTD)
- Secrets Manager ARNs for each site's secrets

**Update `ptd.yaml`**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_external_secrets_operator: true
```

**Apply changes**:

```bash
export AWS_PROFILE=ptd-staging
ptd ensure ganso01-staging
```

**Verify**:

```bash
# Check external-secrets-operator pods running
ptd workon ganso01-staging -- kubectl get pods -n external-secrets

# Check ClusterSecretStore created
ptd workon ganso01-staging -- kubectl get clustersecretstore

# Check ExternalSecrets syncing for each site
ptd workon ganso01-staging -- kubectl get externalsecrets -n posit-team

# Verify ExternalSecret status (should show "SecretSynced")
ptd workon ganso01-staging -- kubectl describe externalsecret <site-name>-secrets -n posit-team

# Check K8s Secrets created with correct keys
ptd workon ganso01-staging -- kubectl get secret <site-name>-secrets -n posit-team -o jsonpath='{.data}' | jq 'keys'
```

**Expected secret keys** (per product):
- Connect: `dev.lic`, `pub.lic`, `pub-db-password`, `pub-db-username`
- Workbench: `dev.lic`, `pub.lic`
- Package Manager: `dev.lic`, `pub.lic`, `pub-db-password`, `pub-db-username`
- Chronicle: `dev.lic`, `pub.lic`

**Troubleshooting**:

If ExternalSecret shows "SecretSyncedError":

```bash
# Check ExternalSecret events
ptd workon ganso01-staging -- kubectl describe externalsecret <site-name>-secrets -n posit-team

# Check external-secrets-operator logs
ptd workon ganso01-staging -- kubectl logs -n external-secrets -l app.kubernetes.io/name=external-secrets --tail=100

# Verify ClusterSecretStore can access Secrets Manager
ptd workon ganso01-staging -- kubectl describe clustersecretstore

# Check IAM permissions for external-secrets ServiceAccount
aws iam get-role --role-name <eso-role-name>
aws iam list-attached-role-policies --role-name <eso-role-name>
```

**Rollback**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_external_secrets_operator: false
```

Then run `ptd ensure ganso01-staging`. Operator falls back to fetching secrets via AWS SDK when K8s Secret references are not present.

### Step 3: Enable nfs-subdir-provisioner

**What it does**: Deploys nfs-subdir-external-provisioner to dynamically provision PVs from a shared NFS/FSx volume. The operator creates PVCs referencing a StorageClass, and the provisioner auto-creates subdirectories and binds them to PVs.

**Prerequisites**:
- FSx OpenZFS or NFS volume with DNS name
- Network connectivity from cluster to NFS/FSx (security groups, NACLs)
- Existing data paths preserved (provisioner uses `nfs.io/storage-path` annotation)

**Update `ptd.yaml`**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_nfs_subdir_provisioner: true
```

**Apply changes**:

```bash
export AWS_PROFILE=ptd-staging
ptd ensure ganso01-staging
```

**Verify**:

```bash
# Check provisioner pod running
ptd workon ganso01-staging -- kubectl get pods -n kube-system -l app=nfs-subdir-external-provisioner

# Check StorageClass created
ptd workon ganso01-staging -- kubectl get storageclass posit-shared-storage

# Check PVCs created with StorageClass
ptd workon ganso01-staging -- kubectl get pvc -n posit-team -o wide

# Verify PVCs bound to PVs
ptd workon ganso01-staging -- kubectl get pvc -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.spec.volumeName}{"\n"}{end}'

# Check product pods mounting PVCs correctly
ptd workon ganso01-staging -- kubectl describe pod -n posit-team -l app=connect | grep -A 5 "Mounts:"
```

**Expected PVC names per site**:
- `{site_name}-connect-volume`
- `{site_name}-dev-volume` (workbench)
- `{site_name}-workbench-shared-storage-volume`
- `{site_name}-shared-volume`
- `{site_name}-packagemanager-volume` (if using separate storage)

**Troubleshooting**:

If PVC stuck in "Pending":

```bash
# Check PVC events
ptd workon ganso01-staging -- kubectl describe pvc <pvc-name> -n posit-team

# Check provisioner logs
ptd workon ganso01-staging -- kubectl logs -n kube-system -l app=nfs-subdir-external-provisioner --tail=100

# Verify StorageClass exists and has correct NFS server
ptd workon ganso01-staging -- kubectl get storageclass posit-shared-storage -o yaml

# Test NFS connectivity from a test pod
# Note: this relies on ptd workon forwarding stdin to kubectl.
# If it doesn't work, write the manifest to a temp file and apply it directly.
cat <<EOF | ptd workon ganso01-staging -- kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: nfs-test
  namespace: posit-team
spec:
  containers:
  - name: test
    image: busybox
    command: ["sh", "-c", "mount -t nfs4 <fsx-dns-name>:/fsx /mnt && ls /mnt && sleep 3600"]
    volumeMounts:
    - name: test
      mountPath: /mnt
  volumes:
  - name: test
    nfs:
      server: <fsx-dns-name>
      path: /fsx
EOF

# Check test pod logs
ptd workon ganso01-staging -- kubectl logs nfs-test -n posit-team
```

**Data migration**: The provisioner uses `nfs.io/storage-path` annotation to set subdirectory paths. PTD configures this as `{site}/{product}` to match existing directory structure. Existing data is reused automatically - no migration needed.

**Rollback**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_nfs_subdir_provisioner: false
```

Then run `ptd ensure ganso01-staging`. Operator falls back to creating PVs directly when `storageClassName` is not set in Site CR.

**Note**: Rollback does NOT delete existing PVCs or data. Storage data remains on the same underlying FSx/NFS volume. However, once the provisioner is removed, new PVC requests against the `posit-shared-storage` StorageClass will fail — existing *bound* PVCs continue working, but any new PVCs (e.g., after pod rescheduling that triggers volume recreation) will stay in Pending until the provisioner is re-enabled.

### Step 4: Enable Gateway API (Optional)

**What it does**: Installs Gateway API CRDs, configures Traefik to use Gateway API provider, and creates a Gateway resource. The operator creates HTTPRoute resources instead of Ingress resources.

**Prerequisites**:
- Traefik v3.1+ (AWS must upgrade from v2 first)
- TLS certificates for HTTPS listeners
- ReferenceGrant for cross-namespace access (if Gateway is in different namespace than products)

**Update `ptd.yaml`**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_gateway_api: true
```

**Apply changes**:

```bash
export AWS_PROFILE=ptd-staging
ptd ensure ganso01-staging
```

**Verify**:

```bash
# Check Gateway API CRDs installed
ptd workon ganso01-staging -- kubectl get crds | grep gateway.networking.k8s.io

# Check Gateway resource created
ptd workon ganso01-staging -- kubectl get gateway -A

# Check Gateway status (should show "Programmed: True")
ptd workon ganso01-staging -- kubectl describe gateway posit-team -n traefik

# Check HTTPRoutes created for each product
ptd workon ganso01-staging -- kubectl get httproutes -n posit-team

# Verify HTTPRoute status (should show accepted by Gateway)
ptd workon ganso01-staging -- kubectl describe httproute <site-name>-connect -n posit-team

# Test product accessibility via HTTPRoute
curl -k https://<connect-hostname> -H "Host: <connect-hostname>"
```

**Expected HTTPRoute names per site**:
- `{site_name}-connect`
- `{site_name}-workbench`
- `{site_name}-packagemanager`
- `{site_name}-chronicle`
- `{site_name}-home` (flightdeck)

**Troubleshooting**:

If HTTPRoute not routing traffic:

```bash
# Check HTTPRoute accepted by Gateway
ptd workon ganso01-staging -- kubectl describe httproute <route-name> -n posit-team | grep -A 10 "Conditions:"

# Check ReferenceGrant exists (if cross-namespace)
ptd workon ganso01-staging -- kubectl get referencegrant -n traefik

# Check Traefik logs
ptd workon ganso01-staging -- kubectl logs -n traefik -l app.kubernetes.io/name=traefik --tail=100

# Verify Gateway listeners
ptd workon ganso01-staging -- kubectl get gateway posit-team -n traefik -o yaml | yq '.spec.listeners'

# Check Gateway address assigned
ptd workon ganso01-staging -- kubectl get gateway posit-team -n traefik -o jsonpath='{.status.addresses}'
```

**Session persistence**: Gateway API v1.2+ supports session persistence via `sessionPersistence` field in HTTPRoute. Traefik v3.1+ supports this natively. If sticky sessions don't work:

```bash
# Check HTTPRoute sessionPersistence config
ptd workon ganso01-staging -- kubectl get httproute <route-name> -n posit-team -o yaml | yq '.spec.rules[0].sessionPersistence'

# Check Traefik sticky cookie in browser dev tools (should see cookie set)
curl -i -k https://<connect-hostname> | grep Set-Cookie
```

**Rollback**:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_gateway_api: false
```

Then run `ptd ensure ganso01-staging`. Operator falls back to creating Ingress resources when `gatewayRef` is not set in Site CR. Traefik continues routing via Ingress provider.

### Azure-Specific Steps

Azure clusters have some differences in the migration process:

**Track 1 (Storage)**: Azure already uses StorageClasses (`azure-netapp-files`, Azure Files CSI). No provisioner needed - just update Site CR to reference the StorageClass name.

**Track 2 (IAM)**: Azure uses Workload Identity, which requires ServiceAccount annotations + pod labels. PTD passes these through the CR:

```yaml
# In Site CR (set by PTD)
connect:
  serviceAccountAnnotations:
    azure.workload.identity/client-id: "<managed-identity-client-id>"
  podLabels:
    azure.workload.identity/use: "true"
```

Verify:

```bash
# Check ServiceAccount annotations
ptd workon <azure-cluster> -- kubectl get serviceaccount <site-name>-connect -n posit-team -o yaml | yq '.metadata.annotations'

# Check pod labels
ptd workon <azure-cluster> -- kubectl get pods -n posit-team -l app=connect -o yaml | yq '.items[0].metadata.labels'
```

**Track 3 (Secrets)**: Azure uses external-secrets-operator with Azure Key Vault provider. Verify ClusterSecretStore uses Workload Identity auth:

```bash
ptd workon <azure-cluster> -- kubectl get clustersecretstore -o yaml | yq '.spec.provider.azurekv.authType'
```

**Track 4 (Gateway API)**: Azure already has Traefik v3, so Gateway API can be enabled immediately (no upgrade needed).

## Verification Checklist

After enabling all tracks on a cluster, verify each product works correctly:

### For Each Site

- [ ] **Site CR reconciled successfully**
  ```bash
  ptd workon ganso01-staging -- kubectl get site <site-name> -n posit-team -o yaml | yq '.status.conditions'
  ```

### For Each Product (Connect, Workbench, PM, Chronicle)

- [ ] **Pod running with correct ServiceAccount**
  ```bash
  ptd workon ganso01-staging -- kubectl get pods -n posit-team -l app=<product> -o jsonpath='{.items[0].spec.serviceAccountName}'
  ```

- [ ] **Secrets mounted from K8s Secret (not SecretProviderClass)**
  ```bash
  ptd workon ganso01-staging -- kubectl describe pod -n posit-team -l app=<product> | grep -A 10 "Mounts:" | grep secret
  ```

- [ ] **Storage using PVC with StorageClass (not direct PV)**
  ```bash
  ptd workon ganso01-staging -- kubectl get pvc -n posit-team -l app=<product> -o jsonpath='{.items[0].spec.storageClassName}'
  ```

- [ ] **Routing via HTTPRoute (if Gateway API enabled) or Ingress**
  ```bash
  # If Gateway API enabled
  ptd workon ganso01-staging -- kubectl get httproute -n posit-team -l app=<product>

  # If Gateway API not enabled (fallback)
  ptd workon ganso01-staging -- kubectl get ingress -n posit-team -l app=<product>
  ```

- [ ] **Product accessible and functional**
  - Connect: Access UI, publish content, verify database access
  - Workbench: Start session, verify home directory access, test Git integration
  - Package Manager: Access UI, verify package listing, test package installation
  - Chronicle: Access audit logs, verify data collection

- [ ] **IAM/Workload Identity working (cloud resources accessible)**
  - Connect: S3 bucket access for content storage
  - Workbench: S3 bucket access for home directories (if applicable)
  - Package Manager: S3 bucket access for package cache (if applicable)

- [ ] **Session persistence working (if applicable)**
  - Workbench: Sessions stick to same pod across requests
  - Check cookie set in browser: `<product>-session` or similar

### Logs Review

Check operator logs for any warnings or errors:

```bash
ptd workon ganso01-staging -- kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator --tail=500 | grep -i "error\|warn"
```

## Rollback Procedure

If issues arise during migration, you can roll back each track independently.

### General Rollback Pattern

1. Set feature flag to `false` in `ptd.yaml`
2. Run `ptd ensure <workload-name>`
3. Operator automatically detects missing new fields in Site CR and falls back to legacy paths
4. Verify products continue working with legacy configuration

### Important Notes

- **No data loss**: Storage data remains on the same underlying volume (FSx/NFS/NetApp). Only the provisioning mechanism changes.
- **No secret data loss**: Secrets synced by external-secrets-operator remain in K8s Secrets even after rollback. Operator fetches via AWS SDK instead.
- **IAM transition**: Rollback from Pod Identity to IRSA requires IAM role trust policy changes. Plan for brief interruption.
- **No downtime for Gateway API rollback**: Operator creates both HTTPRoute and Ingress during transition. Disabling Gateway API just stops creating HTTPRoutes - Ingress continues working.

### Per-Track Rollback

| Track | Rollback Flag | Fallback Behavior | Data Impact |
|-------|--------------|-------------------|-------------|
| Storage | `enable_nfs_subdir_provisioner: false` | Operator creates PVs directly with cloud CSI driver | None - same underlying volume |
| IAM | `enable_pod_identity_agent: false` | Operator computes IRSA ARN and annotates ServiceAccounts | Requires IAM trust policy change back to OIDC (see below) |
| Secrets | `enable_external_secrets_operator: false` | Operator calls AWS SM SDK and creates SecretProviderClass | None - products re-fetch from cloud secrets |
| Gateway API | `enable_gateway_api: false` | Operator creates Ingress resources | None - traffic continues via Ingress |

#### IAM Rollback: OIDC Trust Policy

When rolling back from Pod Identity to IRSA, update each IAM role's trust policy to trust your cluster's OIDC provider instead of `pods.eks.amazonaws.com`:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      "Federated": "arn:aws:iam::<account-id>:oidc-provider/oidc.eks.<region>.amazonaws.com/id/<oidc-id>"
    },
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Condition": {
      "StringEquals": {
        "oidc.eks.<region>.amazonaws.com/id/<oidc-id>:sub": "system:serviceaccount:posit-team:<service-account-name>",
        "oidc.eks.<region>.amazonaws.com/id/<oidc-id>:aud": "sts.amazonaws.com"
      }
    }
  }]
}
```

Get your OIDC provider ID:
```bash
aws eks describe-cluster --name <cluster-name> --query 'cluster.identity.oidc.issuer' --output text | awk -F'/' '{print $NF}'
```

### Emergency Rollback (Full)

If all tracks need rollback:

```yaml
clusters:
  "YYYYMMDD":
    spec:
      enable_pod_identity_agent: false
      enable_external_secrets_operator: false
      enable_nfs_subdir_provisioner: false
      enable_gateway_api: false
```

```bash
export AWS_PROFILE=ptd-staging
ptd ensure ganso01-staging
```

Verify all products return to legacy configuration:

```bash
# Check PVs created directly (not via StorageClass)
ptd workon ganso01-staging -- kubectl get pv -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.csi.driver}{"\n"}{end}'

# Check SecretProviderClass resources exist
ptd workon ganso01-staging -- kubectl get secretproviderclass -n posit-team

# Check ServiceAccount IRSA annotations
ptd workon ganso01-staging -- kubectl get serviceaccount -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.eks\.amazonaws\.com/role-arn}{"\n"}{end}'

# Check Ingress resources (not HTTPRoute)
ptd workon ganso01-staging -- kubectl get ingress -n posit-team
```

## Production Rollout

After successful staging verification, roll out to production clusters incrementally.

### Pre-Production Checklist

- [ ] All staging clusters migrated and stable for 24-48 hours
- [ ] No errors in operator logs across staging clusters
- [ ] All products verified functional in staging
- [ ] Customer-facing testing completed (if applicable)
- [ ] Rollback procedure tested in staging
- [ ] Runbook reviewed by team

### Rollout Schedule

**Do NOT enable all production clusters simultaneously.** Roll out one cluster at a time with monitoring between each.

Example schedule:
1. Day 1: First production cluster (lowest traffic)
2. Day 2: Monitor, verify logs, check product health
3. Day 3: Second production cluster
4. Day 4: Monitor
5. Day 5: Third production cluster
6. Continue until all production clusters migrated

### Per-Cluster Rollout

For each production cluster:

1. **Announce maintenance window** (if required - migration should be zero-downtime)
2. **Update `ptd.yaml` with feature flags**
3. **Run `ptd ensure <workload-name>`**
4. **Verify immediately** using verification checklist
5. **Monitor for 24 hours**
   - Check operator logs every 4-6 hours
   - Verify product uptime metrics
   - Check for customer-reported issues
6. **Proceed to next cluster** only after 24h stability

### Monitoring During Rollout

```bash
# Operator reconciliation errors
ptd workon <workload> -- kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator --since=1h | grep -i error

# Product pod restarts (should be zero)
ptd workon <workload> -- kubectl get pods -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.containerStatuses[0].restartCount}{"\n"}{end}'

# PVC binding issues
ptd workon <workload> -- kubectl get pvc -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}' | grep -v Bound

# ExternalSecret sync status
ptd workon <workload> -- kubectl get externalsecrets -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}' | grep -v True

# HTTPRoute acceptance
ptd workon <workload> -- kubectl get httproutes -n posit-team -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.parents[0].conditions[?(@.type=="Accepted")].status}{"\n"}{end}' | grep -v True
```

### Rollout Halt Criteria

Stop rollout and investigate if:
- Any product becomes unavailable
- Operator logs show repeated reconciliation errors
- Customer-reported issues spike
- PVCs stuck in Pending state
- ExternalSecrets fail to sync
- HTTPRoutes not routing traffic

## Phase 3: Cleanup (Future)

After all clusters are successfully migrated and stable for several weeks/months, the team-operator maintainers will release a new version that removes deprecated CRD fields and legacy code paths.

### What Will Be Removed

- `VolumeSource` field from Site spec (replaced by `storageClassName`)
- `AzureFilesConfig` from PackageManager spec (replaced by `packageManagerStorageClassName`)
- `AwsAccountId`, `ClusterDate`, `WorkloadCompoundName` from all product specs (replaced by `serviceAccountName` + optional annotations)
- `Secret.Type` and `Secret.VaultName` (replaced by `Secret.Name`)
- `IngressClass` and `IngressAnnotations` (replaced by `gatewayRef`)
- `EFSEnabled` and `VPCCIDR` (replaced by `nfsEgressCIDR`)

### Impact

After Phase 3 cleanup release:
- **Clusters NOT yet migrated will break** if upgraded to the new operator version
- **All clusters must be migrated BEFORE upgrading** to Phase 3 operator version
- PTD will track migration status and block Phase 3 upgrade on unmigrated clusters

## Troubleshooting

### Common Issues

#### Issue: PVC Stuck in Pending

**Symptoms**:
- `kubectl get pvc` shows "Pending" status
- Product pods stuck in "ContainerCreating"

**Diagnosis**:
```bash
ptd workon <workload> -- kubectl describe pvc <pvc-name> -n posit-team
```

**Common causes**:
1. **StorageClass not found**: Verify `posit-shared-storage` StorageClass exists
   ```bash
   ptd workon <workload> -- kubectl get storageclass posit-shared-storage
   ```

2. **Provisioner not running**: Check nfs-subdir-provisioner pod
   ```bash
   ptd workon <workload> -- kubectl get pods -n kube-system -l app=nfs-subdir-external-provisioner
   ptd workon <workload> -- kubectl logs -n kube-system -l app=nfs-subdir-external-provisioner --tail=100
   ```

3. **NFS connectivity issue**: Test NFS mount using the test pod spec from Step 3 above (the full pod manifest with `sh -c "mount ... && ls ... && sleep 3600"`). The simple `kubectl run` one-liner will not work — `mount` requires privileges not available in a default busybox container.

4. **Security group blocking NFS**: Verify security groups allow port 2049 from cluster to FSx

**Resolution**: Fix the underlying cause, then PVC should bind automatically. If not, delete and recreate:
```bash
ptd workon <workload> -- kubectl delete pvc <pvc-name> -n posit-team
# Operator will recreate
```

#### Issue: ExternalSecret Not Syncing

**Symptoms**:
- `kubectl get externalsecrets` shows "SecretSyncedError"
- K8s Secret not created or missing keys

**Diagnosis**:
```bash
ptd workon <workload> -- kubectl describe externalsecret <site-name>-secrets -n posit-team
```

**Common causes**:
1. **ClusterSecretStore misconfigured**: Check store status
   ```bash
   ptd workon <workload> -- kubectl describe clustersecretstore
   ```

2. **IAM permissions missing**: Verify external-secrets-operator ServiceAccount can access Secrets Manager
   ```bash
   # Check ServiceAccount has Pod Identity association or IRSA annotation
   ptd workon <workload> -- kubectl get serviceaccount -n external-secrets external-secrets-sa -o yaml

   # Check IAM role has secretsmanager:GetSecretValue permission
   aws iam get-role --role-name <eso-role-name>
   aws iam list-attached-role-policies --role-name <eso-role-name>
   ```

3. **Secret ARN wrong or not found**: Verify Secrets Manager secret exists
   ```bash
   aws secretsmanager describe-secret --secret-id <secret-arn>
   ```

4. **Key mapping incorrect**: Check ExternalSecret spec matches secret structure
   ```bash
   # View raw secret structure
   aws secretsmanager get-secret-value --secret-id <secret-arn> | jq '.SecretString | fromjson | keys'
   ```

**Resolution**: Fix IAM permissions or secret ARN, then ESO will retry automatically. Check logs:
```bash
ptd workon <workload> -- kubectl logs -n external-secrets -l app.kubernetes.io/name=external-secrets --tail=100
```

#### Issue: Pod Identity Not Working (AWS)

**Symptoms**:
- Product pods can't access AWS resources (S3, Secrets Manager)
- Logs show "AccessDenied" or "no valid credentials"

**Diagnosis**:
```bash
# Check Pod Identity Agent running
ptd workon <workload> -- kubectl get pods -n kube-system | grep eks-pod-identity-agent

# Check Pod Identity associations exist
aws eks list-pod-identity-associations --cluster-name <cluster-name>

# Check pod ServiceAccount
ptd workon <workload> -- kubectl get pods -n posit-team -l app=<product> -o jsonpath='{.items[0].spec.serviceAccountName}'

# Check IAM role trust policy
aws iam get-role --role-name <product-role-name> | jq '.Role.AssumeRolePolicyDocument'
```

**Common causes**:
1. **Pod Identity Agent not installed**: Verify addon
   ```bash
   aws eks describe-addon --cluster-name <cluster-name> --addon-name eks-pod-identity-agent
   ```

2. **Pod Identity association missing**: Check associations match ServiceAccount names
   ```bash
   aws eks list-pod-identity-associations --cluster-name <cluster-name>
   ```

3. **IAM role trust policy still using OIDC**: Must be updated to trust `pods.eks.amazonaws.com`
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{
       "Effect": "Allow",
       "Principal": {
         "Service": "pods.eks.amazonaws.com"
       },
       "Action": ["sts:AssumeRole", "sts:TagSession"]
     }]
   }
   ```

4. **ServiceAccount name mismatch**: Verify CR `serviceAccountName` matches Pod Identity association
   ```bash
   ptd workon <workload> -- kubectl get site <site-name> -n posit-team -o yaml | yq '.spec.<product>.serviceAccountName'
   ```

**Resolution**: Update IAM trust policy, recreate Pod Identity associations, or fix ServiceAccount name in CR.

#### Issue: HTTPRoute Not Routing Traffic (Gateway API)

**Symptoms**:
- Product URL returns 404 or times out
- HTTPRoute exists but traffic not routed

**Diagnosis**:
```bash
# Check HTTPRoute status
ptd workon <workload> -- kubectl describe httproute <route-name> -n posit-team | grep -A 10 "Conditions:"

# Check Gateway status
ptd workon <workload> -- kubectl describe gateway posit-team -n traefik | grep -A 10 "Conditions:"

# Check Traefik logs
ptd workon <workload> -- kubectl logs -n traefik -l app.kubernetes.io/name=traefik --tail=100 | grep -i httproute
```

**Common causes**:
1. **HTTPRoute not accepted by Gateway**: Check `Accepted: False` condition
   ```bash
   ptd workon <workload> -- kubectl get httproute <route-name> -n posit-team -o yaml | yq '.status.parents[0].conditions'
   ```

2. **ReferenceGrant missing**: Required for cross-namespace access (Gateway in `traefik` namespace, HTTPRoute in `posit-team`)
   ```bash
   ptd workon <workload> -- kubectl get referencegrant -n traefik
   ```

3. **Gateway listeners not configured**: Check HTTPS listener with correct port and TLS
   ```bash
   ptd workon <workload> -- kubectl get gateway posit-team -n traefik -o yaml | yq '.spec.listeners'
   ```

4. **Hostname mismatch**: Verify HTTPRoute `hostnames` matches Gateway listener
   ```bash
   ptd workon <workload> -- kubectl get httproute <route-name> -n posit-team -o yaml | yq '.spec.hostnames'
   ```

**Resolution**: Create missing ReferenceGrant, fix Gateway listener config, or correct HTTPRoute hostname.

#### Issue: Sticky Sessions Not Working

**Symptoms**:
- Workbench sessions disconnect between requests
- Multiple product sessions created for same user

**Diagnosis**:
```bash
# Check HTTPRoute sessionPersistence config
ptd workon <workload> -- kubectl get httproute <product>-route -n posit-team -o yaml | yq '.spec.rules[0].sessionPersistence'

# Test cookie set in response
curl -i -k https://<product-hostname> | grep Set-Cookie
```

**Common causes**:
1. **Session persistence not configured**: HTTPRoute missing `sessionPersistence` field
2. **Traefik version too old**: Requires Traefik v3.1+ for Gateway API session persistence
3. **Cookie attributes incorrect**: Check cookie flags (Secure, HttpOnly, SameSite)

**Resolution**: Update HTTPRoute with correct `sessionPersistence` config, or upgrade Traefik.

### Getting Help

If issues persist after troubleshooting:

1. **Check operator logs** for reconciliation errors
   ```bash
   ptd workon <workload> -- kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator --tail=500 > operator-logs.txt
   ```

2. **Export affected resources** for review
   ```bash
   ptd workon <workload> -- kubectl get site <site-name> -n posit-team -o yaml > site-cr.yaml
   ptd workon <workload> -- kubectl get pods,pvc,externalsecrets,httproutes -n posit-team -o yaml > resources.yaml
   ```

3. **Contact team** with:
   - Cluster name and workload ID
   - Operator logs
   - Resource definitions
   - Error messages from `kubectl describe`

## Reference

### PTD Commands

```bash
# List all steps for a workload
ptd ensure <workload-name> --list-steps

# Run specific infrastructure step
ptd ensure <workload-name> --only-steps <step-name>

# Dry-run to preview changes
ptd ensure <workload-name> --dry-run

# One-shot kubectl command
ptd workon <workload-name> -- kubectl <command>

# One-shot Pulumi command
ptd workon <workload-name> <step-name> -- pulumi <command>
```

### Key Infrastructure Components

| Component | Purpose | Namespace |
|-----------|---------|-----------|
| nfs-subdir-external-provisioner | Dynamic PV provisioning from NFS/FSx | `kube-system` |
| external-secrets-operator | Sync cloud secrets to K8s Secrets | `external-secrets` |
| eks-pod-identity-agent | AWS Pod Identity agent (AWS only) | `kube-system` |
| Gateway API CRDs | Standard ingress abstraction | cluster-wide |
| Traefik Gateway | Gateway API implementation | `traefik` |

### StorageClass Names

| Cloud | StorageClass Name | Backend |
|-------|------------------|---------|
| AWS | `posit-shared-storage` | FSx OpenZFS via nfs-subdir-provisioner |
| Azure | `azure-netapp-files` | Azure NetApp Files CSI |
| Azure | `azure-files` | Azure Files CSI (for PM) |
| kind | `standard` | local-path-provisioner (built-in) |

### Related Documentation

- [Team Operator Architecture](../architecture.md)
- [Upgrading Team Operator](./upgrading.md)
- [Troubleshooting Guide](./troubleshooting.md)
