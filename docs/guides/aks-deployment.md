# Deploying on AKS

## Overview

Team Operator is a Kubernetes controller that watches a single `Site` Custom Resource Definition (CRD) and reconciles it into everything Posit Team needs to run: deployments, services, ingress routes, database schemas, and storage. You declare what you want; the operator makes it so. This guide covers deploying Posit Team on Azure Kubernetes Service (AKS) using Team Operator. No PTD CLI required.

The journey has six steps before products start up: install the operator, create the secrets it reads, configure shared storage for Workbench home directories, verify the database connection, deploy Traefik for ingress, and finally apply the `Site` CR. Each step builds on the last. You cannot create the `Site` CR until the preceding infrastructure is in place, and the operator will tell you (through `Site` conditions) if something is missing.

Posit Workbench is the first product enabled in the example below. Posit Connect and Posit Package Manager can be enabled in the same `Site` CR once the initial deployment is stable. Note that Package Manager's OIDC configuration differs from Connect and Workbench; see the [Package Manager Configuration](packagemanager-configuration.md) guide for details.

For product-specific configuration (OIDC, Databricks, custom session images, etc.), see the guides linked in each step.

## Prerequisites

- AKS cluster running Kubernetes 1.29+ with a dedicated subnet delegated to `Microsoft.NetApp/volumes` for Azure NetApp Files storage
- Azure Database for PostgreSQL Flexible Server reachable from the cluster: either Virtual Network (VNet)-injected (subnet delegation to `Microsoft.DBforPostgreSQL/flexibleServers`) or via private endpoint
- Traefik ingress controller. Team Operator generates Traefik-specific `Middleware` and `IngressRoute` CRDs; Team Operator does not support other ingress controllers
- `kubectl` configured against the target cluster
- Helm 3.x

## Step 1: Install the operator {#install-the-operator}

The operator runs in its own namespace (`posit-team-system`) and watches the `posit-team` namespace where product workloads live. Installing it first ensures the CRDs are registered before you try to create a `Site` resource.

```bash
helm install team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace
```

### AKS System Node Pool Toleration

AKS system node pools carry a `CriticalAddonsOnly=Exists:NoSchedule` taint by default. If you are running a system-only node pool (no user pool configured), the operator pod will stay `Pending` without a matching toleration. Create a values file and pass it during installation:

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

Once installed, verify the operator pod reaches `Running` before moving on. The CRDs it registers are required by every subsequent step:

```bash
kubectl get pods -n posit-team-system
```

## Step 2: Create Kubernetes secrets {#create-kubernetes-secrets}

Team Operator reads credentials from three Kubernetes secrets in the `posit-team` namespace.

> **Important:** All three secrets must exist in the `posit-team` namespace before you create the Site CR. The operator checks for them immediately during reconciliation.

```bash
kubectl create namespace posit-team
```

### Secret 1: Site Secret

This secret holds product credentials. The operator reads specific keys based on which products are enabled in the `Site` CR; you only need to include the keys for the products you're deploying. Providing extra keys for disabled products is harmless, but missing a required key for an enabled product will cause that product's reconciliation to fail.

```bash
kubectl create secret generic site-secrets \
  --namespace posit-team \
  --from-literal=pub-db-password='<connect-db-password>' \
  --from-literal=pub-secret-key='<connect-secret-key>' \
  --from-literal=dev-db-password='<workbench-db-password>' \
  --from-literal=dev-admin-token='<workbench-admin-token>' \
  --from-literal=dev-user-token='<workbench-user-token>' \
  --from-literal=pkg-db-password='<packagemanager-db-password>' \
  --from-literal=pkg-secret-key='<packagemanager-secret-key>'
```

> **Note:** `dev-admin-token` and `dev-user-token` are only required when Workbench uses OIDC authentication. For password auth they are not read, but including them is harmless.

License keys use a separate mechanism. Instead of placing them in the site secret, each product references its license through `spec.<product>.license.existingSecretName` and `existingSecretKey` on the Site CR. You can store licenses in the same Kubernetes secret or in a dedicated one:

```bash
kubectl create secret generic site-secrets \
  --namespace posit-team \
  --from-literal=dev-license='<workbench-license-key>' \
  # ... other keys from above
```

Then reference it in the Site CR (see Step 6):

```yaml
workbench:
  license:
    existingSecretName: site-secrets
    existingSecretKey: dev-license
```

Only include the keys for the products you are enabling. See the Pre-flight Secret Checklist in the [Site Management Guide](product-team-site-management.md#pre-flight-secret-checklist) for a full reference of required keys per product.

### Secret 2: Workload Secret

All products use the same PostgreSQL host, read from this secret's `main-database-url` key. The operator parses the URL for the hostname and SSL settings; per-product databases and users are provisioned separately using the credentials in Secret 3.

```bash
kubectl create secret generic workload-secrets \
  --namespace posit-team \
  --from-literal=main-database-url='postgresql://<fqdn>/<dbname>?sslmode=require'
```

Replace `<fqdn>` with your Azure PostgreSQL Flexible Server hostname (e.g., `myserver.postgres.database.azure.com`) and `<dbname>` with your target database name. Do not include credentials in the URL; the operator injects them from the DB credential secret at runtime.

### Secret 3: Database Credential Secret

The operator uses these credentials to connect to PostgreSQL as a superuser and provision per-product databases and roles during initial startup. The admin user must have `CREATE ROLE` and `CREATE DATABASE` privileges. Once provisioning is complete, each product connects using its own role. These superuser credentials are only used during setup and schema migrations.

```bash
kubectl create secret generic db-credentials \
  --namespace posit-team \
  --from-literal=username='<db-admin-username>' \
  --from-literal=password='<db-admin-password>'
```

## Step 3: Configure storage {#configure-storage}

Workbench requires `ReadWriteMany` storage for user home directories and shared configuration. Multiple Workbench pods (and optionally Connect) need to mount the same volume simultaneously. Azure NetApp Files is the recommended option on AKS because it provides high-performance NFS storage with `ReadWriteMany` support.

### Provision Azure NetApp Files

Pre-provision the following Azure resources before creating the Site CR:

1. **Azure NetApp Files account** in the same region as the AKS cluster
2. **Capacity pool** (Premium tier, minimum 1 TiB)
3. **NFSv3 volumes** for Workbench home directories and shared storage, on the delegated subnet
4. **Dedicated subnet** with delegation to `Microsoft.NetApp/volumes` and an NSG allowing all protocols from VirtualNetwork

### Create a StorageClass and static PVs

The Team Operator creates PVCs with `storageClassName: azure-netapp-files` when `volumeSource.type: azure-netapp` is set in the Site CR. Pre-create the StorageClass and static NFS PVs so the PVCs can bind.

```yaml
# azure-netapp-files-sc.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azure-netapp-files
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Retain
volumeBindingMode: Immediate
```

```bash
kubectl apply -f azure-netapp-files-sc.yaml
```

Create a static PV for each ANF volume. Replace `<ANF_MOUNT_IP>` with the volume's mount target IP address and `<EXPORT_PATH>` with the volume's creation token (NFS export path):

```yaml
# workbench-home-pv.yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: <site-name>-workbench
spec:
  capacity:
    storage: 200Gi
  accessModes:
    - ReadWriteMany
  storageClassName: azure-netapp-files
  persistentVolumeReclaimPolicy: Retain
  nfs:
    server: <ANF_MOUNT_IP>
    path: /<EXPORT_PATH>
```

```bash
kubectl apply -f workbench-home-pv.yaml
```

Repeat for shared storage using the name `<site-name>-workbench-shared-storage` and a StorageClass named `<site-name>-workbench-shared-storage-nfs`.

## Step 4: Configure the database connection {#configure-the-database-connection}

Azure Database for PostgreSQL Flexible Server requires `sslmode=require` in the connection string. Connections without SSL will be rejected. The operator reads the database host from the workload secret (`main-database-url`) you created in Step 2 and the admin credentials from the DB credential secret. No additional configuration is needed here unless your PostgreSQL server uses VNet injection.

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

## Step 5: Deploy Traefik {#deploy-traefik}

Team Operator generates Traefik `Middleware` and `IngressRoute` custom resources for each product. Traefik must be deployed and its CRDs must be registered in the cluster before you create the `Site` CR. If the CRDs don't exist when the operator tries to create ingress routes, reconciliation will fail and will not recover until Traefik is present.

Deploy Traefik using Helm. The `allowCrossNamespace: true` setting is required because the operator creates `IngressRoute` resources in the `posit-team` namespace that reference middlewares in other namespaces. In hardened environments, Traefik supports scoping cross-namespace access to specific namespaces rather than a blanket `true`; see the Traefik documentation for `allowCrossNamespace` with namespace filtering.

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
helm repo add traefik https://traefik.github.io/charts
helm repo update

helm install traefik traefik/traefik \
  --namespace posit-team \
  --create-namespace \
  --values traefik-values.yaml
```

Once Traefik is running, retrieve the external IP assigned to its LoadBalancer service and create Domain Name System (DNS) records pointing to it before products become accessible:

```bash
kubectl get svc traefik -n posit-team
```

Create a wildcard DNS record (or individual records for each product subdomain) pointing to that IP.

## Step 6: Create the Site CR {#create-the-site-cr}

> **Important:** Deploy Traefik and verify its CRDs are registered before creating the Site CR. The operator creates Traefik-specific resources during reconciliation and will fail if the CRDs are missing.

With secrets, storage, database, and Traefik in place, you're ready to tell Team Operator what to deploy. The `Site` CR is the single resource the operator watches. Everything else (deployments, services, ingress routes, databases) flows from it.

The following example enables Workbench with Azure NetApp Files storage and disables Connect and Package Manager for an initial deployment. Starting with one product lets you validate the full stack before enabling additional products:

```yaml
# site.yaml
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: main
  namespace: posit-team
spec:
  # Base domain: products are available at <prefix>.<domain>
  domain: posit.example.com

  # Ingress class: must match the Traefik deployment
  ingressClass: traefik

  # Site-level secret (DB passwords, encryption keys, OIDC client secrets)
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

  # Azure NetApp Files for home directories (see Step 3)
  volumeSource:
    type: azure-netapp

  # Workbench — enabled with Azure NetApp Files for home directories
  workbench:
    enabled: true
    image: "ghcr.io/rstudio/rstudio-workbench:ubuntu2204-2025.12.0"  # Check https://ghcr.io/rstudio/rstudio-workbench for the latest tag
    replicas: 1
    auth:
      type: password  # Change to "oidc" for SSO; see authentication-setup.md
    license:
      existingSecretName: site-secrets
      existingSecretKey: dev-license

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

## Step 7: Verify the deployment {#verify-the-deployment}

After applying the `Site` CR, the operator begins reconciling: provisioning databases, creating PVCs, deploying pods, and setting up ingress. Reconciliation takes a minute or two on first run. Use the following commands to follow the progress.

### Operator

Check the operator itself is healthy, then tail its logs to watch reconciliation events as they happen:

```bash
# Check operator pod is running
kubectl get pods -n posit-team-system

# Tail operator logs
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=50 -f
```

### Site Status

The `Site` resource's `Conditions` field is the authoritative signal for deployment health. A healthy site shows `Ready: true`; if something is wrong, the condition message will point you toward the cause:

```bash
# View Site status and conditions
kubectl describe site main -n posit-team
```

### Product Pods

```bash
# Check all pods in the product namespace
kubectl get pods -n posit-team

# View Workbench logs
kubectl logs -n posit-team deploy/main-workbench -c workbench --tail=50
```

### Database Provisioning

The operator tracks per-product database provisioning through `PostgresDatabase` resources. If a product is stuck starting up, checking these resources will tell you whether the database creation step succeeded:

```bash
# Check PostgresDatabase resources
kubectl get postgresdatabases -n posit-team
kubectl describe postgresdatabase <name> -n posit-team
```

## Troubleshooting

### NFS Mount Issues

If pods are stuck in `Init` and events show `MountVolume.SetUp failed` with `mount.nfs: access denied by server`, check the following:

1. **NSG rules**: The NetApp subnet NSG must allow all protocols (TCP and UDP) from VirtualNetwork. NFSv3 uses UDP port 111 for portmapper; blocking UDP causes mount failures.
2. **Export policy**: Verify the ANF volume export policy allows clients from the AKS subnet CIDR (or `0.0.0.0/0` if using NSG-only access control).
3. **Subnet delegation**: Verify the NetApp subnet has delegation to `Microsoft.NetApp/volumes` with actions `Microsoft.Network/networkinterfaces/*` and `Microsoft.Network/virtualNetworks/subnets/join/action`.

```bash
# Check pod mount errors
kubectl describe pod <pod-name> -n posit-team | grep -A5 FailedMount

# Verify PV and PVC bindings
kubectl get pv,pvc -n posit-team
```

### Node Scheduling (CriticalAddonsOnly Taint)

If the operator pod stays `Pending` and `kubectl describe pod` shows `node(s) had taints that the pod didn't tolerate`, your cluster is using a system-only node pool with the `CriticalAddonsOnly` taint. Add the toleration to your Helm values (see Step 1) and upgrade:

```bash
helm upgrade team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --values azure-values.yaml
```

See the [Troubleshooting Guide](troubleshooting.md#operator-pod-stuck-in-pending-scheduling-failures) for the full table of common toleration patterns.

### DNS Resolution

If products load but OIDC callbacks fail, or products cannot reach each other by hostname, the issue is usually that DNS records haven't been created yet or are pointing at the wrong IP. You can verify resolution from inside the cluster to rule out local DNS configuration as a factor:

```bash
# Test DNS from within the cluster
kubectl run -it --rm dns-test --image=busybox --restart=Never -- \
  nslookup workbench.posit.example.com
```

Ensure your DNS records (wildcard or per-product) point to the Traefik LoadBalancer IP, and that the cluster's DNS policy resolves external names correctly.

### NetApp Volume Access

If NFS mounts fail with `access denied`, verify the volume export policy in the Azure portal or CLI:

```bash
az netappfiles volume show \
  --resource-group <rg> \
  --account-name <account> \
  --pool-name <pool> \
  --name <volume> \
  --query "exportPolicy.rules[0].allowedClients" -o tsv
```

Ensure `allowedClients` includes the AKS node CIDR. Also verify the NetApp subnet NSG is not blocking NFS traffic (ports 111, 635, 2049 on both TCP and UDP).

### Database Connection Failures

If operator logs show `error determining database url` or `postgres database no main database url found`, the workload secret either doesn't exist or uses a different key name than the operator expects. The key must be exactly `main-database-url`:

```bash
# Verify the workload secret exists and has the correct key
kubectl get secret workload-secrets -n posit-team -o jsonpath='{.data.main-database-url}' | base64 -d

# Check operator logs for database errors
kubectl logs -n posit-team-system deployment/team-operator-controller-manager --tail=100 | grep -i database
```

If the secret looks correct but connections still fail, verify the PostgreSQL server's firewall allows inbound connections from the AKS node CIDR.

## Related Documentation

- [Site Management Guide](product-team-site-management.md) — Full Site spec reference and lifecycle management
- [Authentication Setup](authentication-setup.md) — OIDC, SAML, and Keycloak configuration
- [Workbench Configuration](workbench-configuration.md) — Session images, Databricks, Positron, resource profiles
- [Connect Configuration](connect-configuration.md) — Publishing, off-host execution, GPU settings
- [Package Manager Configuration](packagemanager-configuration.md) — Azure Files, Git sources, S3 storage
- [Troubleshooting Guide](troubleshooting.md) — Operator issues, database problems, networking
