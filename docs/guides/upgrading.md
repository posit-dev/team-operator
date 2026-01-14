# Upgrading Team Operator

This guide provides comprehensive instructions for upgrading the Team Operator, including pre-upgrade preparation, upgrade procedures, version-specific migrations, and troubleshooting.

## Before Upgrading

### Backup Procedures

Before performing any upgrade, create backups of critical resources:

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

If using external databases for products (Connect, Workbench, Package Manager), ensure you have database backups before upgrading. The operator manages `PostgresDatabase` resources that may be affected by schema changes.

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

Always review the [CHANGELOG.md](../../CHANGELOG.md) for breaking changes between your current version and the target version. Pay special attention to:

- Breaking changes that require configuration updates
- Deprecated fields that need migration
- New required fields

### Test in Non-Production

**Critical**: Always test upgrades in a non-production environment first:

1. Create a staging cluster or namespace that mirrors production
2. Apply the same Site configuration
3. Perform the upgrade
4. Verify all products function correctly
5. Test any automated integrations

## Upgrade Methods

### Helm Upgrade Procedure

The recommended method for upgrading is via Helm:

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

CRDs are automatically updated during Helm upgrade when `crd.enable: true` (default). However, if you've disabled CRD management:

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

CRDs require special attention during upgrades:

1. **CRDs Persist Across Helm Uninstall**: By default (`crd.keep: true`), CRDs remain in the cluster even after `helm uninstall`. This prevents accidental data loss but means CRDs must be managed carefully.

2. **CRD Version Compatibility**: The operator manages CRDs at API version `core.posit.team/v1beta1` (and `keycloak.k8s.keycloak.org/v2alpha1` for Keycloak). Ensure your CRs are compatible with the CRD schema in the new version.

3. **Schema Validation**: After CRD updates, existing CRs are validated against the new schema. Invalid CRs may prevent proper reconciliation.

```bash
# Verify CRDs are updated
kubectl get crds sites.core.posit.team -o jsonpath='{.metadata.resourceVersion}'

# Check for validation issues
kubectl get sites -A -o json | jq '.items[] | select(.status.conditions[]?.reason == "InvalidSpec")'
```

## Version-Specific Migrations

### v1.2.0

**New Features:**
- Added `CreateOrUpdateResource` helper for improved reconciliation
- Post-mutation label validation for Traefik resources

**Deprecations:**
- `BasicCreateOrUpdate` function is deprecated in favor of `CreateOrUpdateResource`

No configuration changes required for users.

### v1.1.0

**New Features:**
- Added `tolerations` and `nodeSelector` support for controller manager

**Migration:**
If you were using workarounds for pod scheduling, update your values:

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

**Bug Fixes:**
- Removed `kustomize-adopt` hook that could fail on tainted clusters

No migration required.

### v1.0.0

**Initial Release:**
- Migration from `rstudio/ptd` repository

If upgrading from the legacy `rstudio/ptd` operator, contact Posit support for migration assistance.

### Known Deprecated Fields

The following fields are deprecated and will be removed in future versions:

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

### Key Migration

The operator automatically migrates legacy UUID-format and binary-format encryption keys to the new hex256 format. This migration happens transparently during reconciliation. Monitor logs for migration messages:

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "migrating"
```

## Post-Upgrade Verification

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

Watch operator logs for the first 15-30 minutes after upgrade:

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager -f
```

Look for:
- Reconciliation errors
- CRD validation failures
- Database connection issues
- Certificate/TLS errors

## Rollback Procedures

### Helm Rollback

If issues occur after upgrade, rollback to the previous release:

```bash
# List release history
helm history team-operator -n posit-team-system

# Rollback to previous revision
helm rollback team-operator <revision-number> -n posit-team-system

# Example: rollback to revision 2
helm rollback team-operator 2 -n posit-team-system
```

### CRD Considerations During Rollback

**Important**: CRDs are not automatically rolled back with Helm rollback due to the `keep` annotation. If the new CRDs added fields, older operator versions may still work but won't recognize new fields.

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

Consider these data implications during rollback:

1. **Database Schema Changes**: If the upgrade included database schema changes, rollback may require database schema rollback as well.

2. **Secret Format Changes**: The operator's automatic key migration is one-way. Rolled-back operators will still work with migrated keys.

3. **Configuration Changes**: CRs modified to use new fields will need manual cleanup if rolling back to a version that doesn't support those fields.

## Zero-Downtime Upgrades

### Best Practices for Production Upgrades

1. **Use Maintenance Windows**: Schedule upgrades during low-traffic periods.

2. **Rolling Update Strategy**: The operator uses a single replica by default. For zero-downtime during operator restarts:
   - Products continue running even if the operator is briefly unavailable
   - No reconciliation occurs during operator restart (typically < 30 seconds)

3. **Staged Rollout**:
   ```bash
   # First, upgrade operator in staging
   helm upgrade team-operator ./dist/chart -n posit-team-system-staging

   # Verify staging works
   # Then upgrade production
   helm upgrade team-operator ./dist/chart -n posit-team-system
   ```

4. **Health Check Considerations**:
   - Liveness probe: `/healthz` (port 8081)
   - Readiness probe: `/readyz` (port 8081)
   - These ensure the operator is ready before receiving reconciliation requests

5. **Leader Election**: If running multiple operator replicas (not typical), leader election ensures only one active reconciler:
   ```yaml
   controllerManager:
     container:
       args:
         - "--leader-elect"
   ```

### Product Availability During Upgrades

- **Workbench**: Sessions continue running; new sessions may be delayed
- **Connect**: Published content remains accessible
- **Package Manager**: Package downloads continue working
- **Flightdeck**: Landing page remains accessible

Only reconciliation (applying changes) is affected during operator restart.

## Troubleshooting Upgrades

### Common Upgrade Issues

#### CRD Validation Failures

**Symptom**: CRs fail validation after CRD update

```bash
# Check for invalid CRs
kubectl get sites -A 2>&1 | grep -i error

# View validation errors
kubectl describe site <site-name> -n <namespace>
```

**Solution**: Update CRs to match new schema requirements or remove deprecated fields.

#### Webhook Issues

**Symptom**: Admission webhook errors after upgrade

```bash
# Check webhook configuration
kubectl get validatingwebhookconfigurations | grep posit
kubectl get mutatingwebhookconfigurations | grep posit

# If webhooks are causing issues and you need to disable temporarily
kubectl delete validatingwebhookconfigurations <webhook-name>
```

**Solution**: Ensure cert-manager is properly configured if webhooks are enabled.

#### Operator Pod CrashLoopBackOff

**Symptom**: Operator pod fails to start

```bash
# Check pod events
kubectl describe pod -n posit-team-system -l control-plane=controller-manager

# Check logs
kubectl logs -n posit-team-system -l control-plane=controller-manager --previous
```

**Common Causes**:
- Missing RBAC permissions for new resources
- Invalid environment variables
- Certificate issues

**Solution**: Check Helm values and ensure all required permissions are granted.

#### Reconciliation Loops

**Symptom**: Operator continuously reconciles resources without reaching stable state

```bash
# Watch operator logs for repeated reconciliation
kubectl logs -n posit-team-system deployment/team-operator-controller-manager -f | grep "Reconciling"
```

**Solution**: Check for label/annotation conflicts or resources being modified by multiple controllers.

#### Database Connection Errors

**Symptom**: Products fail to start due to database errors

```bash
# Check database connectivity
kubectl logs -n posit-team <product-pod> | grep -i database
```

**Solution**: Verify database credentials in secrets and ensure network policies allow database access.

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
