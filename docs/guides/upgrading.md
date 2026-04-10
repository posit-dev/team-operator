---
title: Upgrading
description: Upgrading Team Operator including pre-upgrade preparation and version-specific migrations
---

# Upgrading Team Operator

Upgrading Team Operator is straightforward in most cases, but doing it safely requires preparation. The operator manages stateful workloads — databases, storage volumes, running user sessions — so an upgrade that goes wrong can affect production users. This guide walks through the full process: what to do before upgrading, how to perform the upgrade, what version-specific migrations are required, how to verify success, and how to roll back if needed.

**Read the version-specific migrations section** for your target version before doing anything else. Some versions require manual steps that must be completed before upgrading the operator, and skipping them will cause failures.

## CRD Management (v1.15+)

Starting with v1.15.0, the operator automatically applies its own CRDs at startup using server-side apply. This ensures the CRD schema always matches the running operator binary, even in cases where only the container image is updated without a full Helm chart upgrade (e.g., adhoc images for testing).

The operator uses the `--manage-crds` flag (default: `true`) to control this behavior. To opt out (for example, if you manage CRDs via Flux or ArgoCD), set:

```yaml
controllerManager:
  container:
    args:
      - "--manage-crds=false"
```

When `--manage-crds=false`, the operator starts without touching CRDs, and you are responsible for keeping them in sync with the operator version.

**Benefits of automatic CRD management:**
- CRDs are always in sync with the operator version
- Works with adhoc images (e.g., PR branches) without requiring Helm chart changes
- Uses server-side apply (SSA) which is idempotent and only updates when schema differs
- No manual CRD management needed for most deployments

**When to disable:**
- GitOps workflows (Flux, ArgoCD) that manage CRDs separately
- Security policies requiring explicit CRD review before application
- Multi-tenant clusters where CRD updates require approval

**RBAC Permissions:**
The operator requires the following RBAC permissions on its own CRDs:
- `get` - to check if CRDs exist
- `patch` - to apply schema updates via server-side apply
- `update` - to modify CRD metadata

The Helm chart automatically grants these permissions. The operator intentionally omits the `delete` verb to prevent accidental data loss.

**Note on CRD deletion:** Because the operator's RBAC omits the `delete` verb for CRDs, if a future operator version removes a resource type, the now-orphaned CRD will remain in the cluster and must be removed manually:

```bash
kubectl delete crd <crd-name>.core.posit.team
```

Before deleting an orphaned CRD, ensure all custom resources of that type have been removed to avoid losing data:

```bash
kubectl get <resource-plural> -A  # verify no instances remain
kubectl delete crd <crd-name>.core.posit.team
```

## Before Upgrading

Before you run any upgrade command, work through the steps in this section. A few minutes of preparation can prevent hours of recovery work.

### Backup Procedures

Create backups of all critical resources before performing any upgrade. If something goes wrong, these backups let you restore to a known-good state.

#### 1. Backup Custom Resources

```bash
# Backup all Site resources
kubectl get sites -A -o yaml > sites-backup.yaml

# Backup all product resources
kubectl get workbenches -A -o yaml > workbenches-backup.yaml
kubectl get connects -A -o yaml > connects-backup.yaml
kubectl get packagemanagers -A -o yaml > packagemanagers-backup.yaml
kubectl get chronicles -A -o yaml > chronicles-backup.yaml
kubectl get flightdecks -A -o yaml > flightdecks-backup.yaml
kubectl get postgresdatabases -A -o yaml > postgresdatabases-backup.yaml

# Backup all Posit Team resources at once
kubectl get sites,workbenches,connects,packagemanagers,chronicles,flightdecks,postgresdatabases -A -o yaml > posit-team-resources-backup.yaml
```

#### 2. Backup Secrets

```bash
# Backup secrets in the Posit Team namespace
kubectl get secrets -n posit-team -o yaml > secrets-backup.yaml

# For sensitive backups, consider encrypting
kubectl get secrets -n posit-team -o yaml | gpg -c > secrets-backup.yaml.gpg
```

#### 3. Backup Databases

If you are using external databases for Connect, Workbench, or Package Manager, back them up before upgrading. The operator manages `PostgresDatabase` resources that schema changes may affect, and some version upgrades include database-related migrations.

```bash
# List managed databases
kubectl get postgresdatabases -A

# For each database, create a backup using your database backup procedures
# Example for PostgreSQL:
# pg_dump -h <host> -U <user> -d <database> > database-backup.sql
```

### Check Current Version

Verify your current installation:

```bash
# Check Helm release version
helm list -n posit-team-system

# Check operator deployment image
kubectl get deployment team-operator-controller-manager -n posit-team-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Check CRD versions
kubectl get crds | grep posit.team
```

### Review Changelog

Review the [CHANGELOG.md](../../CHANGELOG.md) for every version between your current version and the target. Pay attention to breaking changes that require configuration updates, deprecated fields that need migration, and new required fields. The version-specific migrations section in this guide summarizes the most significant changes, but the CHANGELOG is the authoritative source.

### Test in Non-Production

Test the upgrade in a staging environment before touching production. Create an environment that mirrors production as closely as possible, apply the same Site configuration, perform the upgrade, verify all products function, and test any automated integrations before proceeding to production.

## Upgrade Methods

### Helm Upgrade Procedure

The recommended upgrade method is Helm:

#### Standard Upgrade

```bash
# Update Helm repository (if using external repo)
helm repo update

# View changes before applying
helm diff upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  --values my-values.yaml

# Perform the upgrade
helm upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  --values my-values.yaml
```

#### Upgrade with Specific Version

```bash
helm upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  --set controllerManager.container.image.tag=v1.2.0 \
  --values my-values.yaml
```

#### Upgrade with CRD Updates

CRDs are updated during Helm upgrade when `crd.enable: true` (default). If you've disabled CRD management:

```bash
# Manually apply CRD updates first
kubectl apply -f dist/chart/templates/crd/

# Then upgrade the operator
helm upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  --values my-values.yaml
```

### Kustomize Upgrade Procedure

If using Kustomize for deployment:

```bash
# Update the kustomization.yaml to reference the new version
# Then apply:
kubectl apply -k config/default

# Or for specific overlays:
kubectl apply -k config/overlays/production
```

### CRD Upgrade Considerations

CRDs require attention during upgrades:

1. **CRDs Persist Across Helm Uninstall**: By default (`crd.keep: true`), CRDs remain in the cluster after `helm uninstall`. This prevents accidental data loss but requires careful CRD management.

2. **CRD Version Compatibility**: The operator manages CRDs at API version `core.posit.team/v1beta1` (and `keycloak.k8s.keycloak.org/v2alpha1` for Keycloak). Your CRs must be compatible with the CRD schema in the new version.

3. **Schema Validation**: After CRD updates, existing CRs are validated against the new schema. Invalid CRs may prevent reconciliation.

```bash
# Verify CRDs are updated
kubectl get crds sites.core.posit.team -o jsonpath='{.metadata.resourceVersion}'

# Check for validation issues
kubectl get sites -A -o json | jq '.items[] | select(.status.conditions[]?.reason == "InvalidSpec")'
```

## Version-Specific Migrations

This section covers changes that require action before or after upgrading. Read the section for every version between your current version and your target version, not just the latest.

### v1.15.0

**Breaking Change: Database Password Secret Rename**

In v1.15.0, the Kubernetes Secret that stores each product component's database password was renamed from `<component-name>` to `<component-name>-db-password`. This change was made to make the purpose of the secret unambiguous when listing secrets in a namespace.

If you are upgrading an existing installation, you must migrate the secrets before upgrading the operator. If you do not, the operator will generate new secrets at the new name with freshly generated passwords. The old secrets will be orphaned with the old passwords still set in PostgreSQL, and products will fail to connect to their databases.

**Migration steps (run before upgrading the operator):**

1. Identify the components with existing DB password secrets:

   ```bash
   for comp in workbench connect packagemanager; do
     kubectl get secret "${comp}" -n posit-team --ignore-not-found -o name
   done
   ```

2. For each component (workbench, connect, packagemanager), rename the secret:

   > **Warning:** If `${NEW_NAME}` already exists in the cluster, do not apply this migration — the operator has already generated a new password and you must re-synchronize the database password manually.

   ```bash
   # Get the old secret data
   OLD_NAME=<component-name>
   NEW_NAME="${OLD_NAME}-db-password"
   NAMESPACE=posit-team

   # Create new secret with old data
   kubectl get secret "${OLD_NAME}" -n "${NAMESPACE}" -o json \
     | python3 -c "import json,sys; d=json.load(sys.stdin); d['metadata']['name']='${NEW_NAME}'; [d['metadata'].pop(k,None) for k in ['resourceVersion','uid','creationTimestamp','managedFields','ownerReferences']]; print(json.dumps(d))" \
     | kubectl apply -f -

   # Delete old secret
   kubectl delete secret "${OLD_NAME}" -n "${NAMESPACE}"
   ```

3. Proceed with the operator upgrade.

Fresh installations and clusters that have never had the operator running against them do not need this migration.

### v1.2.0

v1.2.0 adds an improved `CreateOrUpdateResource` helper for reconciliation and post-mutation label validation for Traefik resources. The older `BasicCreateOrUpdate` function is deprecated in favor of the new helper, but this is an internal implementation change. No configuration changes are required for users upgrading to this version.

### v1.1.0

v1.1.0 adds native `tolerations` and `nodeSelector` support for the controller manager. Previously, users who needed the operator to schedule on specific nodes had to use workarounds. If you have a custom workaround in place, replace it with the official Helm values:

```yaml
controllerManager:
  tolerations:
    - key: "node-role.kubernetes.io/control-plane"
      operator: "Exists"
      effect: "NoSchedule"
  nodeSelector:
    kubernetes.io/os: linux
```

### v1.0.4

v1.0.4 removes the `kustomize-adopt` hook that could fail on clusters with tainted nodes. No migration is required.

### v1.0.0

v1.0.0 is the initial release following migration from the `rstudio/ptd` repository. If you are upgrading from the legacy `rstudio/ptd` operator, contact Posit support for migration assistance.

### Known Deprecated Fields

The fields listed below are deprecated and will be removed in a future version. Migrate away from them before they are removed to avoid unexpected breakage during a future upgrade.

| CRD | Field | Replacement | Notes |
|-----|-------|-------------|-------|
| Site | `spec.secretType` | `spec.secret.type` | Use the new Secret configuration block |
| Workbench | `spec.config.databricks.conf` | `spec.secretConfig.databricks` | Databricks config moved to SecretConfig |
| PackageManager | `spec.config.CRAN` | N/A | PackageManagerCRANConfig is deprecated |

**Migration Example - Databricks Configuration:**

Before (deprecated):
```yaml
apiVersion: core.posit.team/v1beta1
kind: Workbench
spec:
  config:
    databricks.conf:
      workspace1:
        name: "My Workspace"
        url: "https://workspace.cloud.databricks.com"
```

After (recommended):
```yaml
apiVersion: core.posit.team/v1beta1
kind: Site
spec:
  workbench:
    databricks:
      workspace1:
        name: "My Workspace"
        url: "https://workspace.cloud.databricks.com"
        clientId: "<client-id>"
```

### Encryption Key Migration

The operator automatically migrates legacy UUID-format and binary-format encryption keys to the current hex256 format. This migration happens transparently during reconciliation — no manual action is required. You can monitor logs to confirm the migration completed:

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "migrating"
```

## Post-Upgrade Verification

After upgrading, verify the operator and all products are healthy before declaring success. Work through these checks in order.

### 1. Check Operator Health

```bash
# Verify the operator pod is running
kubectl get pods -n posit-team-system -l control-plane=controller-manager

# Check operator logs for errors
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=100

# Verify health endpoints
kubectl exec -n posit-team-system deployment/team-operator-controller-manager -- wget -qO- http://localhost:8081/healthz
kubectl exec -n posit-team-system deployment/team-operator-controller-manager -- wget -qO- http://localhost:8081/readyz
```

### 2. Verify CRD Versions

```bash
# List all Posit Team CRDs with versions
kubectl get crds -o custom-columns=NAME:.metadata.name,VERSION:.spec.versions[0].name | grep posit.team

# Expected output:
# chronicles.core.posit.team        v1beta1
# connects.core.posit.team          v1beta1
# flightdecks.core.posit.team       v1beta1
# packagemanagers.core.posit.team   v1beta1
# postgresdatabases.core.posit.team v1beta1
# sites.core.posit.team             v1beta1
# workbenches.core.posit.team       v1beta1
```

### 3. Test Product Functionality

```bash
# Check all Sites are reconciling
kubectl get sites -A

# Check individual product resources
kubectl get workbenches -A
kubectl get connects -A
kubectl get packagemanagers -A

# Verify deployments are healthy
kubectl get deployments -n posit-team

# Test product endpoints
curl -I https://workbench.<your-domain>
curl -I https://connect.<your-domain>
curl -I https://packagemanager.<your-domain>
```

### 4. Monitor for Issues

Watch operator logs for the first 15-30 minutes after upgrade. The reconciliation loop runs frequently, so issues typically surface quickly:

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager -f
```

Pay particular attention to reconciliation errors, CRD validation failures, database connection issues, and certificate or TLS errors.

## Rollback Procedures

If you discover a problem after upgrading that cannot be quickly fixed forward, roll back to the previous operator version. Be aware of the data implications described below before proceeding.

### Helm Rollback

Roll back to a previous Helm release revision:

```bash
# List release history
helm history team-operator -n posit-team-system

# Rollback to previous revision
helm rollback team-operator <revision-number> -n posit-team-system

# Example: rollback to revision 2
helm rollback team-operator 2 -n posit-team-system
```

### CRD Considerations During Rollback

CRDs are not rolled back automatically with Helm rollback due to the `keep` annotation. If the new CRD version added fields, the older operator will still function but will not recognize or manage those fields. In most cases this is acceptable, but if the CRD schema changed in a way that breaks older operators, you will need to roll back the CRDs manually as well.

If CRD rollback is necessary:

```bash
# Save current CRs
kubectl get sites,workbenches,connects,packagemanagers -A -o yaml > pre-rollback-backup.yaml

# Apply old CRDs (from your backup or previous chart version)
kubectl apply -f old-crds/

# Verify CRs are still valid
kubectl get sites -A
```

### Data Implications

Before rolling back, understand these constraints:

Database schema changes made by the upgrade are not automatically reversed. If the new version ran migrations against your PostgreSQL databases, rolling back the operator does not undo those changes, and you may need to roll back the database schema separately.

Encryption key migrations are one-way. The automatic migration of legacy keys to the hex256 format is idempotent and cannot be reversed, but rolled-back operators will continue to work with the migrated key format.

Custom Resources that were updated to use new fields will need manual cleanup if rolling back to a version that does not support those fields. Review any CR changes you made since the upgrade before proceeding.

## Zero-Downtime Upgrades

The operator restart during an upgrade typically takes less than 30 seconds, during which reconciliation is paused but products continue serving traffic normally. Understanding this separation is key to planning a production upgrade with minimal risk.

### Best Practices for Production Upgrades

Schedule upgrades during low-traffic periods when possible. While products remain available during operator restart, any configuration changes submitted during that window will not be applied until reconciliation resumes.

The operator runs a single replica by default. Products continue running if the operator is briefly unavailable during the restart, but no reconciliation occurs during that period.

For staged rollouts, upgrade staging before production and verify each environment fully before proceeding:

```bash
# First, upgrade operator in staging
helm upgrade team-operator ./dist/chart -n posit-team-system-staging

# Verify staging works, then upgrade production
helm upgrade team-operator ./dist/chart -n posit-team-system
```

The operator exposes health endpoints that Kubernetes uses to verify readiness before routing reconciliation requests:

- Liveness probe: `/healthz` (port 8081)
- Readiness probe: `/readyz` (port 8081)

If running multiple operator replicas (uncommon), enable leader election to ensure only one active reconciler:

```yaml
controllerManager:
  container:
    args:
      - "--leader-elect"
```

### Product Availability During Upgrades

During an operator restart, all products remain available to end users. Workbench sessions continue running, though new sessions may be delayed until reconciliation resumes. Connect published content stays accessible, Package Manager package downloads continue working, and the Flightdeck landing page remains accessible. Only the ability to apply configuration changes is temporarily suspended.

## Troubleshooting Upgrades

### If CRs fail validation after a CRD update

After a CRD schema change, existing CRs are validated against the new schema. CRs that use removed or renamed fields will fail validation.

```bash
# Check for invalid CRs
kubectl get sites -A 2>&1 | grep -i error

# View validation errors
kubectl describe site <site-name> -n <namespace>
```

Update CRs to match the new schema requirements, removing deprecated fields and adding any new required ones.

### If you see admission webhook errors after upgrading

Webhook failures usually mean cert-manager is not properly configured or the webhook certificate has not been provisioned.

```bash
# Check webhook configuration
kubectl get validatingwebhookconfigurations | grep posit
kubectl get mutatingwebhookconfigurations | grep posit
```

If webhooks are preventing the operator from starting and you need to remove them temporarily:

```bash
kubectl delete validatingwebhookconfigurations <webhook-name>
```

Ensure cert-manager is running and the operator's certificate resources are healthy before re-enabling webhooks.

### If the operator pod is in CrashLoopBackOff after upgrading

A new version may require RBAC permissions for resources it did not previously manage. Check pod events and logs:

```bash
# Check pod events
kubectl describe pod -n posit-team-system -l control-plane=controller-manager

# Check logs
kubectl logs -n posit-team-system -l control-plane=controller-manager --previous
```

Common causes are missing RBAC permissions for new resources, invalid environment variables, and certificate issues. Verify Helm values are complete for the new version and that all required permissions are granted.

### If the operator is in a continuous reconciliation loop after upgrading

If the operator reconciles repeatedly without reaching a stable state, check for label or annotation conflicts between the new operator version and existing resources:

```bash
# Watch operator logs for repeated reconciliation
kubectl logs -n posit-team-system deployment/team-operator-controller-manager -f | grep "Reconciling"
```

Resources being modified by multiple controllers simultaneously can also trigger loops. Check whether any other tools (Flux, ArgoCD, custom controllers) are modifying the same resources.

### If products fail to start due to database errors after upgrading

Verify database credentials are still correct in the secrets and that network policies have not been tightened during the upgrade:

```bash
kubectl logs -n posit-team <product-pod> | grep -i database
```

### Getting Help

If you encounter issues not covered in this guide:

1. **Check Operator Logs**:
   ```bash
   kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=200
   ```

2. **Review GitHub Issues**: Check [existing issues](https://github.com/posit-dev/team-operator/issues)

3. **Contact Support**: [Contact Posit](https://posit.co/schedule-a-call/) for enterprise support

4. **Collect Diagnostic Information**:
   ```bash
   # Create a diagnostic bundle
   kubectl get all -n posit-team-system -o yaml > diag-system.yaml
   kubectl get sites,workbenches,connects,packagemanagers -A -o yaml > diag-resources.yaml
   kubectl logs -n posit-team-system deployment/team-operator-controller-manager > diag-logs.txt
   ```

## Related Documentation

- [Helm Chart README](../../dist/chart/README.md) - Installation and configuration reference
- [Site Management Guide](./product-team-site-management.md) - Managing Posit Team sites
- [CHANGELOG](../../CHANGELOG.md) - Version history and release notes
