# Team Operator Troubleshooting Guide

This comprehensive guide covers common issues and their solutions when running Posit Team products via the Team Operator.

## Table of Contents

1. [General Debugging](#general-debugging)
2. [Operator Issues](#operator-issues)
3. [Site Reconciliation Issues](#site-reconciliation-issues)
4. [Database Issues](#database-issues)
5. [Product-Specific Issues](#product-specific-issues)
   - [Connect Issues](#connect-issues)
   - [Workbench Issues](#workbench-issues)
   - [Package Manager Issues](#package-manager-issues)
   - [Chronicle Issues](#chronicle-issues)
6. [Networking Issues](#networking-issues)
7. [Storage Issues](#storage-issues)
8. [Authentication Issues](#authentication-issues)
9. [Common Error Messages](#common-error-messages)

---

## General Debugging

### Checking Operator Logs

The operator logs are your first stop for diagnosing issues:

```bash
# View operator logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager

# Follow logs in real-time
kubectl logs -n posit-team-system deployment/team-operator-controller-manager -f

# View logs with timestamps
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --timestamps

# View last 100 lines
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=100
```

### Viewing CR Status and Conditions

Check the status of your Custom Resources:

```bash
# View Site status
kubectl describe site <site-name> -n posit-team

# View Connect status
kubectl describe connect <site-name> -n posit-team

# View Workbench status
kubectl describe workbench <site-name> -n posit-team

# View Package Manager status
kubectl describe packagemanager <site-name> -n posit-team

# View PostgresDatabase status
kubectl describe postgresdatabase <database-name> -n posit-team
```

### Common kubectl Commands for Debugging

```bash
# List all Posit Team resources
kubectl get sites,connects,workbenches,packagemanagers,chronicles -n posit-team

# List all pods with labels
kubectl get pods -n posit-team --show-labels

# View pod events
kubectl get events -n posit-team --sort-by='.lastTimestamp'

# Get all resources managed by the operator
kubectl get all -n posit-team -l app.kubernetes.io/managed-by=team-operator

# View ConfigMaps
kubectl get configmaps -n posit-team

# View Secrets (names only)
kubectl get secrets -n posit-team

# View PVCs
kubectl get pvc -n posit-team

# View Ingresses
kubectl get ingress -n posit-team
```

### Enabling Debug Mode

Enable debug mode at the Site level for verbose logging:

```yaml
spec:
  debug: true
```

This enables debug logging for all products deployed by the Site.

---

## Operator Issues

### Operator Not Starting

**Symptoms:**
- Team operator pod not running
- CrashLoopBackOff status on operator pod

**Diagnosis:**
```bash
# Check operator pod status
kubectl get pods -n posit-team-system

# View operator logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --previous

# Describe the operator pod
kubectl describe pod -n posit-team-system -l control-plane=controller-manager
```

**Common Causes and Solutions:**

| Cause | Solution |
|-------|----------|
| CRD not installed | Run `kubectl apply -f config/crd/bases/` or reinstall via Helm |
| Image pull error | Verify image exists and pull secrets are configured |
| Insufficient resources | Increase memory/CPU limits for operator deployment |
| Invalid configuration | Check operator ConfigMap for syntax errors |

### Permission Errors (RBAC)

**Symptoms:**
- Error messages containing `forbidden` or `unauthorized`
- Resources not being created despite no errors in Site spec

**Diagnosis:**
```bash
# Check operator service account
kubectl get serviceaccount -n posit-team-system

# View operator RBAC
kubectl get clusterrole team-operator-manager-role -o yaml
kubectl get rolebinding -n posit-team -l app.kubernetes.io/managed-by=team-operator
```

**Common Causes and Solutions:**

| Error Message | Solution |
|---------------|----------|
| `cannot create resource "deployments"` | Ensure RBAC includes apps/deployments verb |
| `cannot create resource "ingresses"` | Add networking.k8s.io/ingresses to RBAC |
| `cannot patch resource "secrets"` | Verify secrets verbs include patch |
| `object not managed by team-operator` | Resource was created outside operator; delete and let operator recreate |

### Leader Election Issues

**Symptoms:**
- Multiple operator instances running but not reconciling
- Operator logs showing leader election failures

**Diagnosis:**
```bash
# Check for leader election lease
kubectl get lease -n posit-team-system

# View leader election status in logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "leader"
```

**Solutions:**
- Ensure only one operator instance is running (check replicas)
- Delete the leader election lease to force re-election:
  ```bash
  kubectl delete lease team-operator-leader-election -n posit-team-system
  ```

### CRD Installation Problems

**Symptoms:**
- `no matches for kind "Site" in version "core.posit.team/v1beta1"`
- Resources not recognized by kubectl

**Diagnosis:**
```bash
# List installed CRDs
kubectl get crd | grep posit

# Verify CRD details
kubectl describe crd sites.core.posit.team
```

**Solutions:**
- Install CRDs manually:
  ```bash
  kubectl apply -f config/crd/bases/
  ```
- Reinstall via Helm with CRD installation enabled:
  ```bash
  helm upgrade --install team-operator ./dist/chart --set installCRDs=true
  ```

---

## Site Reconciliation Issues

### Site Stuck in Reconciling

**Symptoms:**
- Site CR exists but products not being created
- Operator continuously reconciling without progress

**Diagnosis:**
```bash
# Check Site events
kubectl describe site <site-name> -n posit-team

# View operator logs for the site
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep <site-name>
```

**Common Causes:**

| Cause | Symptom | Solution |
|-------|---------|----------|
| Invalid domain | Error in logs about domain parsing | Ensure `spec.domain` is valid DNS name |
| Missing secrets | Secret not found errors | Create required secrets before Site |
| Database unreachable | Connection timeout errors | Verify database connectivity and credentials |
| Volume provisioning failed | PVC pending | Check storage class and provisioner |

### Products Not Being Created

**Symptoms:**
- Site created but no Connect/Workbench/PackageManager CRs

**Diagnosis:**
```bash
# Check if product CRs exist
kubectl get connects,workbenches,packagemanagers -n posit-team

# Check operator logs for specific product
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "reconcile"
```

**Solutions:**
- Verify product is enabled in Site spec (products are created by default)
- Check for validation errors in operator logs
- Ensure all required fields are populated

### Status Conditions Not Updating

**Symptoms:**
- Product status shows `ready: false` despite pods running

**Diagnosis:**
```bash
# Check product status
kubectl get connect <site-name> -n posit-team -o jsonpath='{.status}'

# Check pod readiness
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect
```

**Solutions:**
- Status updates occur after successful reconciliation
- Check readiness probes are passing on pods
- Operator may need to be restarted if stuck

---

## Database Issues

### PostgresDatabase Not Ready

**Symptoms:**
- PostgresDatabase CR exists but `ready` status is false
- Product pods failing to start due to database errors

**Diagnosis:**
```bash
# Check PostgresDatabase status
kubectl describe postgresdatabase <database-name> -n posit-team

# Check operator logs for database operations
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "database\|postgres"
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Main database unreachable | Verify `mainDatabaseCredentialSecret` points to valid credentials |
| Invalid database URL | Check database host, port, and SSL mode |
| Role creation failed | Ensure main database user has CREATE ROLE permission |
| Database creation failed | Ensure main database user has CREATE DATABASE permission |

### Connection Failures

**Symptoms:**
- `error determining database url` in operator logs
- `postgres database no main database url found`

**Diagnosis:**
```bash
# Check database credential secret
kubectl get secret -n posit-team | grep -i db

# View secret contents (base64 encoded)
kubectl get secret <db-secret-name> -n posit-team -o yaml
```

**Solutions:**

1. **Verify secret exists with correct keys:**
   ```bash
   kubectl get secret <secret-name> -n posit-team -o jsonpath='{.data}' | jq
   ```

2. **Test database connectivity from a pod:**
   ```bash
   kubectl run -it --rm psql-test --image=postgres:15 --restart=Never -- \
     psql "postgresql://<user>:<password>@<host>/<database>?sslmode=require"
   ```

3. **Check SSL mode configuration:**
   - Ensure `sslmode` matches your database requirements (require, verify-full, etc.)

### Schema Creation Errors

**Symptoms:**
- `error with alter schema` or `error creating schema` in logs
- Product pod starts but database operations fail

**Diagnosis:**
```bash
# Check operator logs for schema errors
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "schema"
```

**Common Error Codes:**

| PostgreSQL Error Code | Meaning | Solution |
|-----------------------|---------|----------|
| 3F000 | Schema does not exist | Schema will be created automatically |
| 42501 | Insufficient privileges | Grant schema permissions to user |
| 42P04 | Duplicate database | Database already exists (usually OK) |

### Credential Issues

**Symptoms:**
- `postgres database no spec url credentials found`
- `postgres database mismatched db host`

**Solutions:**

1. **For AWS Secrets Manager:**
   ```yaml
   spec:
     secret:
       type: "aws"
       vaultName: "your-vault-name"
     mainDatabaseCredentialSecret:
       type: "aws"
       vaultName: "rds!db-identifier"
   ```

2. **For Kubernetes Secrets:**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: site-secrets
   stringData:
     pub-db-password: "<connect-db-password>"
     dev-db-password: "<workbench-db-password>"
     pkg-db-password: "<packagemanager-db-password>"
   ```

---

## Product-Specific Issues

### Connect Issues

#### Connect Not Starting

**Symptoms:**
- Connect pod in CrashLoopBackOff or Error state
- Container failing readiness probes

**Diagnosis:**
```bash
# Check Connect pod status
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect

# View Connect logs
kubectl logs -n posit-team deploy/<site-name>-connect -c connect

# Check events
kubectl describe pod -n posit-team -l app.kubernetes.io/name=connect
```

**Common Causes:**

| Symptom | Cause | Solution |
|---------|-------|----------|
| License error in logs | Invalid or missing license | Verify license secret and key |
| Database connection error | Database unreachable or wrong credentials | Check database configuration |
| Permission denied on volume | PVC mounted with wrong permissions | Check storage class and PVC settings |
| Config file not found | ConfigMap not mounted | Verify ConfigMap exists |

#### Connect Sessions Not Running

**Symptoms:**
- Content execution fails
- Session jobs not created or failing

**Diagnosis:**
```bash
# List session jobs
kubectl get jobs -n posit-team -l posit.team/component=connect-session

# Check session pod logs
kubectl logs -n posit-team job/<session-job-name>

# View job events
kubectl describe job <session-job-name> -n posit-team
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Init container failed | Check session image is accessible |
| Runtime image not found | Verify runtime.yaml configuration |
| Service account missing | Check session service account exists |
| RBAC insufficient | Verify session RBAC permissions |

### Workbench Issues

#### Workbench Sessions Failing

**Symptoms:**
- User sessions not starting
- IDE not loading after login

**Diagnosis:**
```bash
# List session pods
kubectl get pods -n posit-team -l posit.team/component=workbench-session

# View Workbench launcher logs
kubectl logs -n posit-team deploy/<site-name>-workbench -c workbench | grep -i launcher
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Launcher not starting | Check launcher configuration in ConfigMap |
| Session image unavailable | Verify default session image is accessible |
| Volume mount issues | Check PVC and storage class |
| Databricks config error | Move Databricks config from `Config` to `SecretConfig` |

**Databricks Configuration Error:**
If you see `the Databricks configuration should be in SecretConfig, not Config`, update your configuration:

```yaml
# Wrong
spec:
  workbench:
    config:
      databricks: {...}  # DO NOT use this

# Correct - configured at Site level
spec:
  workbench:
    databricks:
      myWorkspace:
        name: "My Workspace"
        url: "https://workspace.cloud.databricks.com"
        clientId: "client-id"
```

#### Workbench HTML Login Page Too Large

**Symptoms:**
- Error about `authLoginPageHtml content exceeds maximum size`

**Solution:**
The custom login HTML is limited to 64KB. Reduce the HTML content size or externalize assets.

### Package Manager Issues

#### Package Manager Build Issues

**Symptoms:**
- Package builds failing
- Git sources not accessible

**Diagnosis:**
```bash
# Check Package Manager logs
kubectl logs -n posit-team deploy/<site-name>-packagemanager

# Check for SSH key issues
kubectl get secretproviderclass -n posit-team | grep ssh
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| SSH keys not mounted | Verify GitSSHKeys configuration |
| S3 bucket inaccessible | Check IAM role and bucket permissions |
| Azure Files PVC pending | Verify storage class and share size |

**Azure Files Configuration Error:**
```
Invalid AzureFiles configuration. Missing StorageClassName or invalid ShareSizeGiB (minimum 100 GiB).
```

**Solution:**
```yaml
spec:
  packageManager:
    azureFiles:
      storageClassName: "azurefile-csi"
      shareSizeGiB: 100  # Minimum 100 GiB required
```

### Chronicle Issues

#### Chronicle Sidecar Problems

**Symptoms:**
- Metrics not being collected
- Chronicle container not running in product pods

**Diagnosis:**
```bash
# Check if Chronicle sidecar exists
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect -o jsonpath='{.items[*].spec.containers[*].name}'

# View Chronicle sidecar logs
kubectl logs -n posit-team deploy/<site-name>-connect -c chronicle
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| `agentImage` not set | Configure `spec.chronicle.agentImage` at Site level |
| Chronicle server unreachable | Check Chronicle StatefulSet is running |
| Network policy blocking | Verify network policies allow Chronicle traffic |

---

## Networking Issues

### Ingress Not Working

**Symptoms:**
- Product URLs return 404 or 502
- Cannot access products externally

**Diagnosis:**
```bash
# Check Ingress resources
kubectl get ingress -n posit-team

# Describe Ingress
kubectl describe ingress <site-name>-connect -n posit-team

# Check Ingress controller logs
kubectl logs -n <ingress-namespace> deploy/<ingress-controller>
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Wrong IngressClass | Set `spec.ingressClass` to match your controller |
| TLS certificate missing | Configure TLS in Ingress annotations |
| Backend service unavailable | Verify product service and pods are running |
| Middleware error | Check Traefik middleware configuration |

### TLS/Certificate Problems

**Symptoms:**
- Certificate errors in browser
- HTTPS not working

**Solutions:**

1. **Check certificate secret:**
   ```bash
   kubectl get secret -n posit-team | grep tls
   ```

2. **Verify cert-manager (if used):**
   ```bash
   kubectl get certificate -n posit-team
   kubectl describe certificate <cert-name> -n posit-team
   ```

3. **Configure TLS in Ingress:**
   ```yaml
   spec:
     ingressAnnotations:
       cert-manager.io/cluster-issuer: "letsencrypt-prod"
   ```

### Service Discovery Issues

**Symptoms:**
- Products cannot communicate with each other
- Chronicle cannot reach product metrics endpoints

**Diagnosis:**
```bash
# Test DNS resolution
kubectl run -it --rm dns-test --image=busybox --restart=Never -- nslookup <service-name>.<namespace>.svc.cluster.local

# Test service connectivity
kubectl run -it --rm curl-test --image=curlimages/curl --restart=Never -- curl http://<service-name>.<namespace>.svc.cluster.local
```

**Solutions:**
- Ensure services are in the same namespace
- Check network policies allow inter-service communication
- Verify service selectors match pod labels

---

## Storage Issues

### PVC Not Binding

**Symptoms:**
- PVC stuck in `Pending` state
- Product pods failing to start due to volume issues

**Diagnosis:**
```bash
# Check PVC status
kubectl get pvc -n posit-team

# Describe pending PVC
kubectl describe pvc <pvc-name> -n posit-team

# Check PV availability
kubectl get pv
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Storage class not found | Create storage class or use existing one |
| No matching PV | Check storage provisioner is running |
| Access mode mismatch | Verify PVC access modes match PV |
| Capacity insufficient | Increase PV size or reduce request |

### Volume Mount Failures

**Symptoms:**
- `MountVolume.SetUp failed`
- Pod stuck in `ContainerCreating`

**Diagnosis:**
```bash
# Check pod events
kubectl describe pod <pod-name> -n posit-team | grep -A10 Events

# Check CSI driver status (if using CSI)
kubectl get pods -n kube-system | grep csi
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| NFS server unreachable | Verify NFS server connectivity |
| FSx volume not found | Check FSx volume ID and DNS name |
| CSI driver not running | Restart CSI driver pods |
| Azure Files secret missing | Create storage account credentials secret |

### Permission Issues

**Symptoms:**
- `permission denied` errors in pod logs
- Product cannot write to data directory

**Diagnosis:**
```bash
# Check file ownership in pod
kubectl exec -it <pod-name> -n posit-team -- ls -la /var/lib/<product>

# Check security context
kubectl get pod <pod-name> -n posit-team -o jsonpath='{.spec.securityContext}'
```

**Solutions:**

1. **Set FSGroup in security context:**
   ```yaml
   spec:
     securityContext:
       fsGroup: 999
   ```

2. **Use init container to fix permissions:**
   ```yaml
   initContainers:
     - name: fix-permissions
       image: busybox
       command: ["sh", "-c", "chown -R 999:999 /data"]
       volumeMounts:
         - name: data
           mountPath: /data
   ```

---

## Authentication Issues

### OIDC Callback Errors

**Symptoms:**
- `Invalid redirect URI` error from IdP
- Login redirects fail

**Diagnosis:**
```bash
# Check Connect logs for OAuth errors
kubectl logs -n posit-team deploy/<site-name>-connect -c connect | grep -i oauth

# Verify callback URL in config
kubectl get configmap <site-name>-connect -n posit-team -o yaml | grep -i callback
```

**Solutions:**

1. **Verify redirect URIs in IdP:**
   - Connect: `https://<connect-url>/__login__/callback`
   - Workbench: `https://<workbench-url>/oidc/callback`

2. **Check client ID and issuer:**
   ```yaml
   spec:
     connect:
       auth:
         type: "oidc"
         clientId: "your-client-id"  # Must match IdP
         issuer: "https://your-idp.com"  # Must be exact
   ```

3. **Enable debug logging:**
   ```yaml
   spec:
     debug: true
   ```

### SAML Metadata Issues

**Symptoms:**
- `SAML authentication requires a metadata URL to be specified`
- SAML metadata URL not accessible

**Diagnosis:**
```bash
# Test metadata URL accessibility
kubectl run -it --rm curl-test --image=curlimages/curl --restart=Never -- \
  curl -v <saml-metadata-url>
```

**Solutions:**

1. **Ensure metadata URL is correct:**
   ```yaml
   spec:
     connect:
       auth:
         type: "saml"
         samlMetadataUrl: "https://idp.example.com/saml/metadata"
   ```

2. **Check network access from cluster:**
   - Verify DNS resolution works
   - Check firewall rules allow outbound HTTPS

**Configuration Conflict Error:**
```
SAML IdPAttributeProfile cannot be specified together with individual SAML attribute mappings
```

**Solution:** Use either `samlIdPAttributeProfile` OR individual attributes, not both:
```yaml
# Option 1: Profile
samlIdPAttributeProfile: "azure"

# Option 2: Individual mappings (mutually exclusive with profile)
# samlUsernameAttribute: "..."
# samlEmailAttribute: "..."
```

### Token/Claim Problems

**Symptoms:**
- Users not getting correct roles
- Groups not syncing from IdP

**Diagnosis:**
```bash
# Enable debug logging and check logs
kubectl logs -n posit-team deploy/<site-name>-connect -c connect | grep -i "claim\|group\|role"
```

**Solutions:**

1. **Verify claims configuration:**
   ```yaml
   spec:
     connect:
       auth:
         usernameClaim: "preferred_username"
         emailClaim: "email"
         groupsClaim: "groups"
   ```

2. **Check scopes include groups:**
   ```yaml
   scopes:
     - "openid"
     - "profile"
     - "email"
     - "groups"
   ```

3. **Disable groups claim if IdP doesn't support it:**
   ```yaml
   disableGroupsClaim: true
   ```

4. **Debug JWT tokens:**
   - Use [jwt.io](https://jwt.io) to inspect tokens
   - Verify expected claims are present

---

## Common Error Messages

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `Site not found; cleaning up resources` | Site CR was deleted | Expected during cleanup; ignore |
| `error determining database url` | Database credentials not found | Check `mainDatabaseCredentialSecret` configuration |
| `postgres database no main database url found` | Main database URL not configured | Configure database secret or check workload secret |
| `postgres database mismatched db host` | Product database host differs from main | Ensure all products use same database host |
| `postgres database no spec url credentials found` | Database password missing | Add password to secret or check secret key name |
| `SAML authentication requires a metadata URL` | Missing SAML metadata URL | Set `samlMetadataUrl` in auth config |
| `SAML IdPAttributeProfile cannot be specified together...` | Conflicting SAML config | Use profile OR individual attributes, not both |
| `object not managed by team-operator` | Resource created outside operator | Delete resource and let operator recreate |
| `mutateFn must set managed-by label` | Internal operator error | Report as bug; check operator version |
| `Invalid AzureFiles configuration` | Missing Azure Files settings | Ensure `storageClassName` set and `shareSizeGiB >= 100` |
| `the Databricks configuration should be in SecretConfig` | Deprecated Databricks location | Move Databricks config to Site `spec.workbench.databricks` |
| `authLoginPageHtml content exceeds maximum size` | Custom HTML too large | Reduce HTML to under 64KB |
| `failed to generate random bytes` | System entropy issue | Check `/dev/urandom` availability |
| `error provisioning SecretProviderClass` | CSI secrets driver issue | Verify secrets-store CSI driver is installed |

---

## Getting Help

If you continue to experience issues:

1. **Collect diagnostic information:**
   ```bash
   kubectl get all -n posit-team -o yaml > posit-team-resources.yaml
   kubectl logs -n posit-team-system deployment/team-operator-controller-manager > operator.log
   kubectl get events -n posit-team --sort-by='.lastTimestamp' > events.txt
   ```

2. **Check Posit documentation:**
   - [Connect Admin Guide](https://docs.posit.co/connect/admin/)
   - [Workbench Admin Guide](https://docs.posit.co/ide/server-pro/admin/)
   - [Package Manager Admin Guide](https://docs.posit.co/rspm/admin/)

3. **Contact Posit Support:**
   - Include diagnostic files
   - Describe the issue and steps to reproduce
   - Include operator and product versions

---

## Related Documentation

- [Site Management Guide](product-team-site-management.md) - Overall Site configuration
- [Authentication Setup](authentication-setup.md) - Detailed auth configuration
- [Connect Configuration](connect-configuration.md) - Connect-specific settings
- [Workbench Configuration](workbench-configuration.md) - Workbench-specific settings
- [Package Manager Configuration](packagemanager-configuration.md) - Package Manager settings
