# Deploying on AKS

## Overview

Team Operator manages the deployment and lifecycle of Posit Team products (Workbench, Connect, Package Manager, and Chronicle) on Kubernetes. It reconciles a single `Site` Custom Resource into the full set of deployments, services, ingress routes, and database schemas required to run the products.

This guide walks through deploying Team Operator on Azure Kubernetes Service (AKS) without the PTD CLI. It covers all infrastructure that the operator expects to exist before the `Site` CR is created: secrets, storage, database, and ingress.

For product-specific configuration (OIDC, Databricks, custom session images, etc.), see the guides linked in each step.

## Prerequisites

- AKS cluster running Kubernetes 1.29+ with the **Azure Files CSI driver** enabled (enabled by default on AKS 1.21+)
- **Azure Database for PostgreSQL Flexible Server** reachable from the cluster — either VNet-injected (subnet delegation to `Microsoft.DBforPostgreSQL/flexibleServers`) or via private endpoint
- **Traefik** ingress controller — Team Operator generates Traefik-specific `Middleware` and `IngressRoute` CRDs; other ingress controllers are not supported
- `kubectl` configured against the target cluster
- Helm 3.x

## Step 1: Install the Operator

Install the operator from the OCI Helm chart into the `posit-team-system` namespace:

```bash
helm install team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace
```

### AKS System Node Pool Toleration

AKS system node pools carry a `CriticalAddonsOnly=Exists:NoSchedule` taint by default. If you are running a system-only node pool (no user pool configured), the operator pod will stay `Pending` without a matching toleration. Create a values file:

```yaml
# azure-values.yaml
watchNamespace: posit-team

controllerManager:
  tolerations:
    - key: "CriticalAddonsOnly"
      operator: "Exists"
      effect: "NoSchedule"
```

```bash
helm install team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace \
  --values azure-values.yaml
```

### Optional: Workload Identity

If the operator needs to access Azure resources (e.g., Azure Key Vault) using Workload Identity, annotate the service account and label the pod:

```yaml
# azure-values.yaml
controllerManager:
  serviceAccount:
    annotations:
      azure.workload.identity/client-id: "<AZURE_CLIENT_ID>"
  pod:
    labels:
      azure.workload.identity/use: "true"
```

Verify the operator pod is running:

```bash
kubectl get pods -n posit-team-system
```

## Step 2: Create Kubernetes Secrets

Team Operator requires three Kubernetes secrets to exist in the `posit-team` namespace **before** you create the Site CR. Create the namespace first:

```bash
kubectl create namespace posit-team
```

### Secret 1: Site Secret

Contains product credentials and license keys. The operator reads specific keys from this secret based on which products are enabled.

```bash
kubectl create secret generic site-secrets \
  --namespace posit-team \
  --from-literal=pub-db-password='<connect-db-password>' \
  --from-literal=pub-secret-key='<connect-secret-key>' \
  --from-literal=pub-license='<connect-license-key>' \
  --from-literal=dev-db-password='<workbench-db-password>' \
  --from-literal=dev-license='<workbench-license-key>' \
  --from-literal=dev-admin-token='<workbench-admin-token>' \
  --from-literal=dev-user-token='<workbench-user-token>' \
  --from-literal=pkg-db-password='<packagemanager-db-password>' \
  --from-literal=pkg-secret-key='<packagemanager-secret-key>' \
  --from-literal=pkg-license='<packagemanager-license-key>'
```

Only include the keys for the products you are enabling. See the Pre-flight Secret Checklist in the [Site Management Guide](product-team-site-management.md#pre-flight-secret-checklist) for a full reference of required keys per product.

### Secret 2: Workload Secret

Contains the main PostgreSQL connection URL. All products read the database host from this URL.

```bash
kubectl create secret generic workload-secrets \
  --namespace posit-team \
  --from-literal=main-database-url='postgresql://<fqdn>/<dbname>?sslmode=require'
```

Replace `<fqdn>` with your Azure PostgreSQL Flexible Server hostname (e.g., `myserver.postgres.database.azure.com`) and `<dbname>` with your target database name.

### Secret 3: Database Credential Secret

Contains the PostgreSQL superuser credentials the operator uses to provision per-product databases and roles.

```bash
kubectl create secret generic db-credentials \
  --namespace posit-team \
  --from-literal=username='<db-admin-username>' \
  --from-literal=password='<db-admin-password>'
```

## Step 3: Configure Storage

Workbench requires `ReadWriteMany` storage for user home directories. Azure Files NFS is the recommended option on AKS because it supports `ReadWriteMany` without additional infrastructure.

### Create a StorageClass for Azure Files NFS

```yaml
# azure-files-nfs-sc.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azure-files-nfs
provisioner: file.csi.azure.com
parameters:
  protocol: nfs
  skuName: Premium_LRS
reclaimPolicy: Retain
volumeBindingMode: Immediate
allowVolumeExpansion: true
```

```bash
kubectl apply -f azure-files-nfs-sc.yaml
```

### IAM Requirements

The AKS cluster's managed identity needs the following roles on the Storage Account used by Azure Files:

- **Storage Account Contributor** — to create file shares
- **Network Contributor** — if the storage account is in a VNet with service endpoints

You can assign these via the Azure CLI:

```bash
CLUSTER_IDENTITY=$(az aks show \
  --resource-group <rg> \
  --name <cluster-name> \
  --query identityProfile.kubeletidentity.objectId \
  --output tsv)

az role assignment create \
  --assignee "$CLUSTER_IDENTITY" \
  --role "Storage Account Contributor" \
  --scope /subscriptions/<sub-id>/resourceGroups/<rg>/providers/Microsoft.Storage/storageAccounts/<sa-name>
```

### Pre-provisioned Shared Storage (Optional)

If you need a shared directory mounted across Workbench and Connect (e.g., for shared project data), create a PersistentVolume backed by a pre-provisioned Azure Files share **before** creating the Site CR. The Site controller will look for the PV when `sharedDirectory` is configured.

## Step 4: Configure the Database Connection

Azure Database for PostgreSQL Flexible Server requires `sslmode=require` in the connection string. The operator reads the database host from the workload secret (`main-database-url`) and the credentials from the DB credential secret (`username` / `password`).

Connection string format:

```
postgresql://<server-fqdn>/<dbname>?sslmode=require
```

Example:

```
postgresql://myserver.postgres.database.azure.com/positteam?sslmode=require
```

The operator creates per-product databases (e.g., `connect`, `workbench`, `packagemanager`) and roles automatically using the credentials from the DB credential secret. The admin user must have `CREATE ROLE` and `CREATE DATABASE` privileges on the PostgreSQL server.

### VNet Injection

If your PostgreSQL server uses VNet injection, ensure the AKS subnet and the PostgreSQL subnet are in the same VNet (or peered), and that the PostgreSQL subnet has a delegation to `Microsoft.DBforPostgreSQL/flexibleServers`.

## Step 5: Deploy Traefik

Team Operator generates Traefik `Middleware` and `IngressRoute` custom resources for each product. Traefik **must be deployed and its CRDs must be installed before you create the Site CR**; otherwise the operator cannot create the ingress routes and reconciliation will fail.

Deploy Traefik using Helm with a LoadBalancer service type to receive a public IP:

```yaml
# traefik-values.yaml
service:
  type: LoadBalancer

providers:
  kubernetesIngress:
    enabled: true
  kubernetesCRD:
    enabled: true
    allowCrossNamespace: true
```

```bash
helm repo add traefik https://helm.traefik.io/traefik
helm repo update

helm install traefik traefik/traefik \
  --namespace posit-team \
  --create-namespace \
  --values traefik-values.yaml
```

`allowCrossNamespace: true` is required because the operator creates `IngressRoute` resources in the `posit-team` namespace that reference middlewares in other namespaces.

After Traefik is running, note the external IP assigned to its LoadBalancer service:

```bash
kubectl get svc traefik -n posit-team
```

Create a wildcard DNS record (or individual records for each product subdomain) pointing to that IP.

## Step 6: Create the Site CR

With secrets, storage, database, and Traefik in place, you can create the Site CR. The following example enables Workbench with Azure Files NFS storage and disables Connect and Package Manager for an initial deployment:

```yaml
# site.yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: main
  namespace: posit-team
spec:
  # Base domain — products are available at <prefix>.<domain>
  domain: posit.example.com

  # Ingress class — must match the Traefik deployment
  ingressClass: traefik

  # Site-level secret (DB passwords, license keys, OIDC client secrets)
  secret:
    type: kubernetes
    vaultName: site-secrets

  # Workload secret (main-database-url)
  workloadSecret:
    type: kubernetes
    vaultName: workload-secrets

  # Database credential secret (username, password for provisioning)
  mainDatabaseCredentialSecret:
    type: kubernetes
    vaultName: db-credentials

  # Workbench — enabled with Azure Files NFS for home directories
  workbench:
    enabled: true
    image: "ghcr.io/posit-dev/workbench:jammy-2024.12.0"
    replicas: 1
    auth:
      type: password  # Change to "oidc" for SSO — see authentication-setup.md
    volume:
      create: true
      size: 100Gi
      storageClassName: azure-files-nfs  # StorageClass from Step 3

  # Connect — disabled for initial deployment
  connect:
    enabled: false

  # Package Manager — disabled for initial deployment
  packageManager:
    enabled: false
```

Apply the manifest:

```bash
kubectl apply -f site.yaml -n posit-team
```

For full Site spec options including OIDC auth, session images, node selectors, and shared storage, see the [Site Management Guide](product-team-site-management.md).

## Step 7: Verify the Deployment

### Operator

```bash
# Check operator pod is running
kubectl get pods -n posit-team-system

# Tail operator logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=50 -f
```

### Site Status

```bash
# View Site status and conditions
kubectl describe site main -n posit-team
```

Look for `Conditions` in the output — a healthy site shows `Ready: true`.

### Product Pods

```bash
# Check all pods in the product namespace
kubectl get pods -n posit-team

# View Workbench logs
kubectl logs -n posit-team deploy/main-workbench -c workbench --tail=50
```

### Database Provisioning

```bash
# Check PostgresDatabase resources
kubectl get postgresdatabases -n posit-team
kubectl describe postgresdatabase <name> -n posit-team
```

## Troubleshooting

### CSI Driver Issues

**Symptom:** PVCs stuck in `Pending`, pod events show `MountVolume.SetUp failed`.

```bash
# Verify Azure Files CSI driver pods are running
kubectl get pods -n kube-system -l app=csi-azurefile-node

# Check CSI driver logs
kubectl logs -n kube-system -l app=csi-azurefile-node -c azurefile
```

Azure Files NFS requires the cluster to have network access to the storage account. Verify the storage account's firewall allows traffic from the AKS subnet.

### Node Scheduling (CriticalAddonsOnly Taint)

**Symptom:** Operator pod stays `Pending`, `kubectl describe pod` shows `node(s) had taints that the pod didn't tolerate`.

Add the `CriticalAddonsOnly` toleration to your Helm values (see Step 1) and upgrade:

```bash
helm upgrade team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --values azure-values.yaml
```

See the [Troubleshooting Guide](troubleshooting.md#operator-pod-stuck-in-pending-scheduling-failures) for the full table of common toleration patterns.

### DNS Resolution

**Symptom:** Products load but OIDC callbacks fail, or products cannot reach each other.

```bash
# Test DNS from within the cluster
kubectl run -it --rm dns-test --image=busybox --restart=Never -- \
  nslookup workbench.posit.example.com
```

Ensure your DNS records (wildcard or per-product) point to the Traefik LoadBalancer IP, and that the cluster's DNS policy resolves external names correctly.

### Storage Account Permissions

**Symptom:** Azure Files PVC stuck in `Pending`, CSI driver logs show `403 Forbidden` or `AuthorizationFailed`.

Verify the cluster's kubelet managed identity has **Storage Account Contributor** on the storage account (see Step 3). Changes to role assignments can take a few minutes to propagate.

### Database Connection Failures

**Symptom:** Operator logs show `error determining database url` or `postgres database no main database url found`.

```bash
# Verify the workload secret exists and has the correct key
kubectl get secret workload-secrets -n posit-team -o jsonpath='{.data.main-database-url}' | base64 -d

# Test connectivity from within the cluster
kubectl run -it --rm psql-test --image=postgres:15 --restart=Never -- \
  psql "postgresql://<user>:<password>@<fqdn>/<dbname>?sslmode=require"
```

Ensure the PostgreSQL server's firewall allows inbound connections from the AKS node CIDR.

## Related Documentation

- [Site Management Guide](product-team-site-management.md) — Full Site spec reference and lifecycle management
- [Authentication Setup](authentication-setup.md) — OIDC, SAML, and Keycloak configuration
- [Workbench Configuration](workbench-configuration.md) — Session images, Databricks, Positron, resource profiles
- [Connect Configuration](connect-configuration.md) — Publishing, off-host execution, GPU settings
- [Package Manager Configuration](packagemanager-configuration.md) — Azure Files, Git sources, S3 storage
- [Troubleshooting Guide](troubleshooting.md) — Operator issues, database problems, networking
