---
title: Troubleshooting
description: Common issues and solutions when running Posit Team products via Team Operator
---

# Team Operator Troubleshooting Guide

This guide is organized by symptom so you can quickly find the section that matches what you are seeing. Start with the General Debugging section to collect the basic information needed for any investigation, then jump to the symptom that applies.

## Table of Contents

1. [General Debugging](#general-debugging)
2. [Operator Issues](#operator-issues)
   - [Operator Pod Stuck in Pending (Scheduling Failures)](#operator-pod-stuck-in-pending-scheduling-failures)
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

When something is wrong, start here. Operator logs and CR status conditions together tell most of the story.

### Checking Operator Logs

Start diagnosing issues by checking the operator logs:

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

### If the operator pod is not running or is in CrashLoopBackOff

Check the pod status and logs to identify the cause:

```bash
# Check operator pod status
kubectl get pods -n posit-team-system

# View operator logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --previous

# Describe the operator pod
kubectl describe pod -n posit-team-system -l control-plane=controller-manager
```

| Cause | Solution |
|-------|----------|
| CRD not installed | Run `kubectl apply -f config/crd/bases/` or reinstall via Helm |
| Image pull error | Verify image exists and pull secrets are configured |
| Insufficient resources | Increase memory/CPU limits for operator deployment |
| Invalid configuration | Check operator ConfigMap for syntax errors |

### If you see "forbidden" or "unauthorized" errors in operator logs

These errors mean the operator's service account does not have the permissions it needs. Check the current RBAC configuration:

```bash
# Check operator service account
kubectl get serviceaccount -n posit-team-system

# View operator RBAC
kubectl get clusterrole team-operator-manager-role -o yaml
kubectl get rolebinding -n posit-team -l app.kubernetes.io/managed-by=team-operator
```

| Error Message | Solution |
|---------------|----------|
| `cannot create resource "deployments"` | Ensure RBAC includes apps/deployments verb |
| `cannot create resource "ingresses"` | Add networking.k8s.io/ingresses to RBAC |
| `cannot patch resource "secrets"` | Verify secrets verbs include patch |
| `object not managed by team-operator` | Resource was created outside operator; delete and let operator recreate |

### If the operator is not reconciling despite running

If multiple operator instances are running, leader election failures can prevent reconciliation. Check the lease and logs:

```bash
# Check for leader election lease
kubectl get lease -n posit-team-system

# View leader election status in logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep -i "leader"
```

Ensure only one operator instance is running (check replicas). If the lease is stale, delete it to force re-election:

```bash
kubectl delete lease team-operator-leader-election -n posit-team-system
```

### If you see "no matches for kind 'Site' in version 'core.posit.team/v1beta1'"

This error means the CRDs are not installed. Verify their presence and install if missing:

```bash
# List installed CRDs
kubectl get crd | grep posit

# Verify CRD details
kubectl describe crd sites.core.posit.team
```

Install CRDs manually or via Helm:

```bash
kubectl apply -f config/crd/bases/
# or
helm upgrade --install team-operator ./dist/chart --set installCRDs=true
```

### If the operator pod is stuck in Pending

If `kubectl describe pod` shows taint-related scheduling errors or events like `node(s) had taints that the pod didn't tolerate`, the operator pod cannot be scheduled because cluster nodes have taints it doesn't tolerate. This is common with dedicated node pools (GPU nodes, session nodes), nodes reserved for system components, or cloud-provider managed node pools with default taints.

Identify the taints on your nodes, then configure matching tolerations:

```bash
# Check operator pod status
kubectl get pods -n posit-team-system

# Describe the pod to see scheduling failures
kubectl describe pod -n posit-team-system -l control-plane=controller-manager

# List node taints to understand what tolerations are needed
kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints
```

Configure tolerations in your Helm values to match your cluster's node taints:

```yaml
# values.yaml
controllerManager:
  tolerations:
    # Example: Tolerate nodes tainted for session workloads
    - key: "workload-type"
      operator: "Equal"
      value: "session"
      effect: "NoSchedule"

    # Example: Tolerate nodes with GPU taints
    - key: "nvidia.com/gpu"
      operator: "Exists"
      effect: "NoSchedule"

    # Example: Tolerate all taints (use with caution)
    # - operator: "Exists"
```

Apply the configuration:

```bash
helm upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  -f values.yaml
```

**Common Toleration Patterns:**

| Scenario | Toleration Configuration |
|----------|-------------------------|
| Session-dedicated nodes | `key: "workload-type", value: "session", effect: "NoSchedule"` |
| GPU nodes | `key: "nvidia.com/gpu", operator: "Exists", effect: "NoSchedule"` |
| Cloud provider taints (EKS) | `key: "eks.amazonaws.com/compute-type", operator: "Exists"` |
| Cloud provider taints (GKE) | `key: "cloud.google.com/gke-nodepool", operator: "Exists"` |
| Control plane nodes | `key: "node-role.kubernetes.io/control-plane", operator: "Exists"` |

**Alternative: Using nodeSelector**

To run the operator on specific nodes instead of tolerating taints, use `nodeSelector`:

```yaml
controllerManager:
  nodeSelector:
    kubernetes.io/os: linux
    node-type: management
```

**Verification:**

After applying tolerations, verify the pod schedules successfully:

```bash
# Check pod is running
kubectl get pods -n posit-team-system

# Verify tolerations were applied
kubectl get deployment team-operator-controller-manager -n posit-team-system \
  -o jsonpath='{.spec.template.spec.tolerations}' | jq
```

---

## Site Reconciliation Issues

### If the Site exists but products are not being created

Check whether the operator created the child product CRs. If the CRs do not exist, the reconciliation is failing before product creation. If the CRs exist but pods are not running, the problem is downstream in the individual product controller.

```bash
# Check Site events
kubectl describe site <site-name> -n posit-team

# View operator logs for the site
kubectl logs -n posit-team-system deployment/team-operator-controller-manager | grep <site-name>

# Check if product CRs were created
kubectl get connects,workbenches,packagemanagers -n posit-team
```

| Cause | Symptom | Solution |
|-------|---------|----------|
| Invalid domain | Error in logs about domain parsing | Ensure `spec.domain` is valid DNS name |
| Missing secrets | Secret not found errors | Create required secrets before Site |
| Database unreachable | Connection timeout errors | Verify database connectivity and credentials |
| Volume provisioning failed | PVC pending | Check storage class and provisioner |

If product CRs are absent, check for validation errors in operator logs and verify that all required fields are populated in the Site spec. Products are created by default — if `enabled: false` is not set, their absence indicates an error in reconciliation.

### If product status shows "ready: false" despite pods running

Status conditions update after a successful reconciliation cycle. If pods are running but status is not updating, check readiness probes:

```bash
# Check product status
kubectl get connect <site-name> -n posit-team -o jsonpath='{.status}'

# Check pod readiness
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect
```

If readiness probes are passing but status remains stuck, restarting the operator forces a fresh reconciliation cycle.

---

## Database Issues

### If you see "error determining database url" or "postgres database no main database url found"

These errors mean the operator cannot find or read the database credential secret. Check the secret exists and contains the expected keys:

```bash
# Check database credential secret
kubectl get secret -n posit-team | grep -i db

# Verify the secret exists with correct keys
kubectl get secret <secret-name> -n posit-team -o jsonpath='{.data}' | jq
```

If the secret exists, test database connectivity directly from a pod to confirm the host is reachable and credentials are valid:

```bash
kubectl run -it --rm psql-test --image=postgres:15 --restart=Never -- \
  psql "postgresql://<user>:<password>@<host>/<database>?sslmode=require"
```

Verify that the `sslmode` in the connection string matches your database server's requirements (`require`, `verify-full`, etc.).

### If you see "postgres database no spec url credentials found" or "postgres database mismatched db host"

These errors indicate a mismatch between the credential secret content and what the operator expects. Verify the secret structure matches the configuration:

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

### If Connect is in CrashLoopBackOff or failing readiness probes

Check the pod logs and events to identify the specific failure:

```bash
# Check Connect pod status
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect

# View Connect logs
kubectl logs -n posit-team deploy/<site-name>-connect -c connect

# Check events
kubectl describe pod -n posit-team -l app.kubernetes.io/name=connect
```

| Symptom | Cause | Solution |
|---------|-------|----------|
| License error in logs | Invalid or missing license | Verify license secret and key |
| Database connection error | Database unreachable or wrong credentials | Check database configuration |
| Permission denied on volume | PVC mounted with wrong permissions | Check storage class and PVC settings |
| Config file not found | ConfigMap not mounted | Verify ConfigMap exists |

### If Connect content execution fails or session jobs are not running

Check whether session jobs are being created and inspect them for errors:

```bash
# List session jobs
kubectl get jobs -n posit-team -l posit.team/component=connect-session

# Check session pod logs
kubectl logs -n posit-team job/<session-job-name>

# View job events
kubectl describe job <session-job-name> -n posit-team
```

| Cause | Solution |
|-------|----------|
| Init container failed | Check session image is accessible |
| Runtime image not found | Verify runtime.yaml configuration |
| Service account missing | Check session service account exists |
| RBAC insufficient | Verify session RBAC permissions |

### If Workbench user sessions are not starting or the IDE is not loading

Check whether session pods are being created and examine the launcher logs:

```bash
# List session pods
kubectl get pods -n posit-team -l posit.team/component=workbench-session

# View Workbench launcher logs
kubectl logs -n posit-team deploy/<site-name>-workbench -c workbench | grep -i launcher
```

| Cause | Solution |
|-------|----------|
| Launcher not starting | Check launcher configuration in ConfigMap |
| Session image unavailable | Verify default session image is accessible |
| Volume mount issues | Check PVC and storage class |
| Databricks config error | Move Databricks config from `Config` to `SecretConfig` |

### If you see "the Databricks configuration should be in SecretConfig, not Config"

Databricks configuration was moved from the `config` block to the top-level `databricks` field on the Workbench spec. Update the configuration:

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

### If you see "authLoginPageHtml content exceeds maximum size"

The custom login HTML is limited to 64KB. Reduce the HTML content size or externalize assets such as images and stylesheets.

### If Package Manager builds are failing or Git sources are not accessible

Check Package Manager logs and verify SSH key and storage configuration:

```bash
# Check Package Manager logs
kubectl logs -n posit-team deploy/<site-name>-packagemanager

# Check for SSH key issues
kubectl get secretproviderclass -n posit-team | grep ssh
```

| Cause | Solution |
|-------|----------|
| SSH keys not mounted | Verify GitSSHKeys configuration |
| S3 bucket inaccessible | Check IAM role and bucket permissions |
| Azure Files PVC pending | Verify storage class and share size |

### If you see "Invalid AzureFiles configuration. Missing StorageClassName or invalid ShareSizeGiB"

The minimum share size for Azure Files is 100 GiB, and the storage class name must be set:

```yaml
spec:
  packageManager:
    azureFiles:
      storageClassName: "azurefile-csi"
      shareSizeGiB: 100  # Minimum 100 GiB required
```

### If Chronicle metrics are not being collected

Check whether the Chronicle sidecar container is present in the product pods:

```bash
# Check if Chronicle sidecar exists
kubectl get pods -n posit-team -l app.kubernetes.io/name=connect -o jsonpath='{.items[*].spec.containers[*].name}'

# View Chronicle sidecar logs
kubectl logs -n posit-team deploy/<site-name>-connect -c chronicle
```

| Cause | Solution |
|-------|----------|
| `agentImage` not set | Configure `spec.chronicle.agentImage` at Site level |
| Chronicle server unreachable | Check Chronicle StatefulSet is running |
| Network policy blocking | Verify network policies allow Chronicle traffic |

---

## Networking Issues

### If product URLs return 404 or 502

Check that ingress resources were created and the ingress class is correct for your controller:

```bash
# Check Ingress resources
kubectl get ingress -n posit-team

# Describe Ingress
kubectl describe ingress <site-name>-connect -n posit-team

# Check Ingress controller logs
kubectl logs -n <ingress-namespace> deploy/<ingress-controller>
```

| Cause | Solution |
|-------|----------|
| Wrong IngressClass | Set `spec.ingressClass` to match your controller |
| TLS certificate missing | Configure TLS in Ingress annotations |
| Backend service unavailable | Verify product service and pods are running |
| Middleware error | Check Traefik middleware configuration |

### If you see certificate errors in the browser or HTTPS is not working

Check the certificate secret and cert-manager status if you are using it:

```bash
kubectl get secret -n posit-team | grep tls
kubectl get certificate -n posit-team
kubectl describe certificate <cert-name> -n posit-team
```

To configure cert-manager certificate provisioning via ingress annotations:

```yaml
spec:
  ingressAnnotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
```

### If products cannot communicate with each other or Chronicle cannot reach metrics endpoints

Test DNS resolution and service connectivity from within the cluster to isolate the problem:

```bash
# Test DNS resolution
kubectl run -it --rm dns-test --image=busybox --restart=Never -- nslookup <service-name>.<namespace>.svc.cluster.local

# Test service connectivity
kubectl run -it --rm curl-test --image=curlimages/curl --restart=Never -- curl http://<service-name>.<namespace>.svc.cluster.local
```

Ensure services are in the same namespace, that network policies allow inter-service communication, and that service selectors match the pod labels.

---

## Storage Issues

### If a PVC is stuck in Pending and pods cannot start

PVCs stay in Pending when there is no storage provisioner that can satisfy the request. Describe the PVC to see the specific error:

```bash
# Check PVC status
kubectl get pvc -n posit-team

# Describe pending PVC
kubectl describe pvc <pvc-name> -n posit-team

# Check PV availability
kubectl get pv
```

| Cause | Solution |
|-------|----------|
| Storage class not found | Create storage class or use existing one |
| No matching PV | Check storage provisioner is running |
| Access mode mismatch | Verify PVC access modes match PV |
| Capacity insufficient | Increase PV size or reduce request |

### If you see "MountVolume.SetUp failed" or a pod stuck in ContainerCreating

Volume mount failures are usually caused by an unreachable storage backend or a missing CSI driver. Check pod events and CSI driver status:

```bash
# Check pod events
kubectl describe pod <pod-name> -n posit-team | grep -A10 Events

# Check CSI driver status (if using CSI)
kubectl get pods -n kube-system | grep csi
```

| Cause | Solution |
|-------|----------|
| NFS server unreachable | Verify NFS server connectivity |
| FSx volume not found | Check FSx volume ID and DNS name |
| CSI driver not running | Restart CSI driver pods |
| Azure Files secret missing | Create storage account credentials secret |

### If you see "permission denied" errors in pod logs

The pod is running as a user that does not own the volume's files. Check the ownership and security context:

```bash
# Check file ownership in pod
kubectl exec -it <pod-name> -n posit-team -- ls -la /var/lib/<product>

# Check security context
kubectl get pod <pod-name> -n posit-team -o jsonpath='{.spec.securityContext}'
```

Fix by setting the FSGroup in the security context, or by running a one-time init container to correct ownership:

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

For detailed authentication troubleshooting, see the [Authentication Setup Guide](./authentication-setup.md#troubleshooting). The sections below cover the most common issues seen at the operator level.

### If you see "Invalid redirect URI" from your IdP

The redirect URI registered in the IdP does not match what Connect or Workbench sends. Check OAuth errors in the logs and verify the callback URL in the ConfigMap:

```bash
# Check Connect logs for OAuth errors
kubectl logs -n posit-team deploy/<site-name>-connect -c connect | grep -i oauth

# Verify callback URL in config
kubectl get configmap <site-name>-connect -n posit-team -o yaml | grep -i callback
```

Verify redirect URIs are registered exactly as shown:
- Connect: `https://<connect-url>/__login__/callback`
- Workbench: `https://<workbench-url>/oidc/callback`

The client ID and issuer must match the IdP exactly. Enable debug logging to see the full OAuth flow:

```yaml
spec:
  debug: true
```

### If you see "SAML authentication requires a metadata URL to be specified"

The `samlMetadataUrl` field is missing from the auth configuration, or the URL is unreachable from within the cluster. Test the URL from a pod:

```bash
kubectl run -it --rm curl-test --image=curlimages/curl --restart=Never -- \
  curl -v <saml-metadata-url>
```

Verify that DNS resolves the URL and that firewall rules allow outbound HTTPS from the cluster.

### If you see "SAML IdPAttributeProfile cannot be specified together with individual SAML attribute mappings"

`samlIdPAttributeProfile` and individual attribute fields are mutually exclusive. Use one or the other:

```yaml
# Option 1: Profile
samlIdPAttributeProfile: "azure"

# Option 2: Individual mappings (mutually exclusive with profile)
# samlUsernameAttribute: "..."
# samlEmailAttribute: "..."
```

### If users are not getting the correct roles or groups are not syncing

Enable debug logging and check logs for claim and group-related messages:

```bash
kubectl logs -n posit-team deploy/<site-name>-connect -c connect | grep -i "claim\|group\|role"
```

Verify that the claims configuration matches your IdP, that the `groups` scope is requested, and that group names in role mappings are an exact case-sensitive match to what the IdP sends. If your IdP does not support a groups claim, set `disableGroupsClaim: true` while keeping `groups: true` for auto-provisioning. Use [jwt.io](https://jwt.io) to decode a token from your IdP and inspect the actual claim names.

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

If issues persist:

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
