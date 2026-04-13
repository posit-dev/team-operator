---
title: AWS Deployment (EKS)
description: How to deploy Posit Team on Amazon Elastic Kubernetes Service using Team Operator
---

# Deploying on AWS (EKS)

## Overview

This guide walks you through deploying Posit Team on Amazon Elastic Kubernetes Service using Team Operator — without the PTD CLI. If you're familiar with Kubernetes but new to Team Operator, here's the mental model: Team Operator is a Kubernetes controller that watches a single `Site` Custom Resource and reconciles it into everything Posit Team needs to run — deployments, services, ingress routes, database schemas, and storage. You declare what you want; the operator makes it so.

The journey has seven steps before products start up: install the operator with IRSA configured, configure secrets (either from AWS Secrets Manager or Kubernetes), provision shared storage via EFS or FSx for OpenZFS, verify the RDS database connection, deploy Traefik for ingress, apply the `Site` CR, and then verify the deployment. Each step builds on the last — you cannot create the `Site` CR until the preceding infrastructure is in place, and the operator will tell you (through `Site` conditions) if something is missing.

By the end, you'll have a running Workbench instance accessible through Traefik at your chosen domain. Connect and Package Manager can be enabled in the same `Site` CR once the initial deployment is stable.

For product-specific configuration (OIDC, Databricks, custom session images, etc.), see the guides linked in each step.

## Prerequisites

Before running any `kubectl` commands, the following AWS resources must exist:

- **EKS cluster** running Kubernetes 1.29+ with the **EBS CSI driver** add-on enabled (required for `gp3` volumes used by product-local PVCs)
- **Amazon RDS for PostgreSQL** (version 14 or later) — deployed in a private subnet with a security group that allows inbound TCP on port 5432 from the EKS node security group
- **EFS file system** (for `ReadWriteMany` Workbench home directories) or **FSx for OpenZFS** volume (for site-level shared storage managed by the operator) — the EFS file system must have mount targets in each availability zone your EKS nodes use
- **AWS Secrets Manager** (recommended) or the plan to use Kubernetes secrets
- **IAM OIDC provider** for the EKS cluster — required for IRSA (IAM Roles for Service Accounts); if not already enabled, run `eksctl utils associate-iam-oidc-provider --cluster <name> --approve`
- **Traefik** ingress controller — Team Operator generates Traefik-specific `Middleware` and `IngressRoute` CRDs; other ingress controllers are not supported
- `kubectl` configured against the target cluster
- Helm 3.x

## Step 1: Install the Operator

The operator runs in its own namespace (`posit-team-system`) and watches the `posit-team` namespace where product workloads live. Installing it first ensures the CRDs are registered before you try to create a `Site` resource.

```bash
helm install team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace
```

### IRSA: IAM Role for the Operator

When Team Operator reads secrets from AWS Secrets Manager, it uses the identity of its own service account. You must create an IAM role with the appropriate permissions and annotate the operator's service account via Helm values. The trust policy on the IAM role must allow the EKS OIDC provider to assume it for the `posit-team-system/team-operator-controller-manager` service account.

```yaml
# aws-values.yaml
watchNamespace: posit-team

controllerManager:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::<ACCOUNT_ID>:role/team-operator-role"
```

The IAM role needs at minimum the ability to read secrets from Secrets Manager:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "arn:aws:secretsmanager:<REGION>:<ACCOUNT_ID>:secret:*"
    }
  ]
}
```

If you are using Kubernetes secrets instead of Secrets Manager, IRSA is optional for the operator itself (though individual product service accounts may still need IAM roles for S3 or EFS access — see later steps).

Pass the values file during installation:

```bash
helm install team-operator \
  oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace \
  --values aws-values.yaml
```

Once installed, verify the operator pod reaches `Running` before moving on — the CRDs it registers are required by every subsequent step:

```bash
kubectl get pods -n posit-team-system
```

## Step 2: Configure Secrets

Team Operator reads credentials from secrets in the `posit-team` namespace. AWS deployments have two options: storing secrets in AWS Secrets Manager (recommended for production) or using standard Kubernetes secrets. Both approaches require the same logical set of keys — the difference is where the values live and how the operator fetches them.

Create the namespace first so secrets have somewhere to live:

```bash
kubectl create namespace posit-team
```

### Option A: AWS Secrets Manager (Recommended)

AWS Secrets Manager is the preferred approach for production EKS deployments. You create JSON secrets in Secrets Manager, and the operator fetches them directly at reconciliation time using IRSA. No sensitive values touch Kubernetes etcd.

The operator expects three logical secrets, each stored as a separate Secrets Manager secret containing a JSON object. The `vaultName` in your Site CR must exactly match the secret name (or ARN) in Secrets Manager.

**Site secret** — product credentials and license keys:

Create a secret named `my-workload.posit.team` (replace with your naming convention) with these JSON keys:

```json
{
  "pub-db-password": "<connect-db-password>",
  "pub-secret-key": "<connect-secret-key>",
  "pub-license": "<connect-license-key>",
  "dev-db-password": "<workbench-db-password>",
  "dev-license": "<workbench-license-key>",
  "dev-admin-token": "<workbench-admin-token>",
  "dev-user-token": "<workbench-user-token>",
  "pkg-db-password": "<packagemanager-db-password>",
  "pkg-secret-key": "<packagemanager-secret-key>",
  "pkg-license": "<packagemanager-license-key>"
}
```

Only include keys for the products you are enabling.

**Workload secret** — the database URL, read by all products:

```json
{
  "main-database-url": "postgresql://<rds-endpoint>/<dbname>?sslmode=require"
}
```

**Database credential secret** — the RDS superuser used during database provisioning. For RDS, AWS Secrets Manager automatically creates and manages a secret when you enable Secrets Manager integration on the RDS instance. The auto-generated secret name follows the pattern `rds!db-<db-resource-id>` and contains `username` and `password` keys. Use that vault name directly in your Site CR:

```yaml
mainDatabaseCredentialSecret:
  type: aws
  vaultName: "rds!db-<your-db-resource-id>"
```

If you created your RDS credentials manually, store them as:

```json
{
  "username": "<db-admin-username>",
  "password": "<db-admin-password>"
}
```

### Option B: Kubernetes Secrets

If your environment policy requires secrets to stay in Kubernetes, create three secrets in the `posit-team` namespace using `kubectl`. This approach mirrors the AKS setup exactly.

**Secret 1: Site Secret**

The operator reads specific keys based on which products are enabled. Missing a required key for an enabled product will cause that product's reconciliation to fail.

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

**Secret 2: Workload Secret**

All products use the same PostgreSQL host, read from the `main-database-url` key. Replace `<rds-endpoint>` with your RDS instance endpoint (e.g., `mydb.abc123.us-east-1.rds.amazonaws.com`):

```bash
kubectl create secret generic workload-secrets \
  --namespace posit-team \
  --from-literal=main-database-url='postgresql://<rds-endpoint>/<dbname>?sslmode=require'
```

**Secret 3: Database Credential Secret**

The admin user must have `CREATE ROLE` and `CREATE DATABASE` privileges on the RDS instance. Once the operator provisions per-product roles during initial startup, these credentials are only used for schema migrations.

```bash
kubectl create secret generic db-credentials \
  --namespace posit-team \
  --from-literal=username='<db-admin-username>' \
  --from-literal=password='<db-admin-password>'
```

See the Pre-flight Secret Checklist in the [Site Management Guide](product-team-site-management.md#pre-flight-secret-checklist) for a full reference of required keys per product.

### Secret Key Reference

| Key | Product | Purpose |
|-----|---------|---------|
| `pub-db-password` | Connect | PostgreSQL password for Connect's database user |
| `pub-secret-key` | Connect | Application secret key |
| `pub-license` | Connect | License key or file content |
| `dev-db-password` | Workbench | PostgreSQL password for Workbench's database user |
| `dev-license` | Workbench | License key or file content |
| `dev-admin-token` | Workbench | Workbench admin API token |
| `dev-user-token` | Workbench | Workbench user API token |
| `pkg-db-password` | Package Manager | PostgreSQL password for Package Manager's database user |
| `pkg-secret-key` | Package Manager | Application secret key |
| `pkg-license` | Package Manager | License key or file content |
| `main-database-url` | All | Full PostgreSQL connection URL (workload secret) |
| `username` | All | Database superuser name (db-credentials secret) |
| `password` | All | Database superuser password (db-credentials secret) |

## Step 3: Configure Storage

Workbench requires `ReadWriteMany` storage for user home directories — multiple Workbench pods (and optionally Connect) need to mount the same volume simultaneously. On AWS there are two approaches: EFS with the EFS CSI driver (recommended for most deployments), or FSx for OpenZFS managed directly by the operator's `volumeSource` mechanism.

### Option A: EFS with the EFS CSI Driver

EFS is the most straightforward path to `ReadWriteMany` on EKS. The EFS CSI driver provisions access points dynamically from a pre-existing EFS file system. You need the file system created and mount targets in place before this step — the driver does not create the EFS file system itself.

**Install the EFS CSI driver** if it is not already installed (it is available as an EKS add-on):

```bash
# Via EKS add-on (recommended)
aws eks create-addon \
  --cluster-name <cluster-name> \
  --addon-name aws-efs-csi-driver \
  --service-account-role-arn arn:aws:iam::<ACCOUNT_ID>:role/AmazonEKS_EFS_CSI_DriverRole
```

The EFS CSI driver's service account needs the `AmazonEFSCSIDriverPolicy` IAM policy (or equivalent) attached via IRSA.

**Create a StorageClass** that points to your EFS file system:

```yaml
# efs-sc.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: efs-sc
provisioner: efs.csi.aws.com
parameters:
  provisioningMode: efs-ap
  fileSystemId: <fs-XXXXXXXXXXXXXXXXX>
  directoryPerms: "700"
reclaimPolicy: Retain
volumeBindingMode: Immediate
```

```bash
kubectl apply -f efs-sc.yaml
```

The `Retain` reclaim policy ensures that deleting a PVC does not delete the underlying EFS access point or user data.

**Enable EFS in the Site CR** to allow the operator to configure network policies that permit Workbench session pods to reach EFS mount targets:

```yaml
spec:
  efsEnabled: true
  vpcCIDR: "10.0.0.0/16"  # Replace with your VPC CIDR
```

These fields tell the operator to open network policy egress from Workbench session pods to the VPC CIDR on the NFS port (2049), which is required for EFS mounts to succeed.

### Option B: FSx for OpenZFS (Operator-Managed)

If you have an FSx for OpenZFS volume, the operator can manage subdirectory provisioning for each product automatically. This requires the FSx volume to already exist and be accessible from your EKS nodes. Reference it via `volumeSource` in the Site CR rather than creating a StorageClass:

```yaml
spec:
  volumeSource:
    type: fsx-zfs
    volumeId: fsvol-<XXXXXXXXXXXXXXXXX>
    dnsName: fs-<XXXXXXXXXXXXXXXXX>.fsx.<region>.amazonaws.com
```

The operator will create subdirectories within the volume for each product and provision PersistentVolumes backed by NFS mounts to the FSx DNS name.

## Step 4: Configure the Database Connection

Amazon RDS for PostgreSQL requires `sslmode=require` in the connection string. The operator reads the database host from the workload secret (`main-database-url`) you configured in Step 2 and the admin credentials from the DB credential secret. No additional configuration is needed here unless you need to adjust SSL settings.

Connection string format:

```
postgresql://<rds-endpoint>/<dbname>?sslmode=require
```

Example:

```
postgresql://mydb.abc123.us-east-1.rds.amazonaws.com/positteam?sslmode=require
```

The operator creates per-product databases (e.g., `connect`, `workbench`, `packagemanager`) and roles automatically using the admin credentials from the DB credential secret. The admin user must have `CREATE ROLE` and `CREATE DATABASE` privileges on the RDS instance.

### Security Group Requirements

The RDS security group must allow inbound TCP on port 5432 from the EKS node security group (or the VPC CIDR if you prefer a broader rule). If the operator pod cannot reach the database endpoint during reconciliation, it will fail with a connection timeout and the `Site` conditions will surface the error.

You can verify connectivity from within the cluster before creating the `Site` CR:

```bash
kubectl run -it --rm pg-test --image=postgres:14 --restart=Never -- \
  psql "postgresql://<admin-user>:<password>@<rds-endpoint>/<dbname>?sslmode=require" -c '\l'
```

## Step 5: Deploy Traefik

Team Operator generates Traefik `Middleware` and `IngressRoute` custom resources for each product. Traefik must be deployed and its CRDs must be registered in the cluster before you create the `Site` CR — if the CRDs don't exist when the operator tries to create ingress routes, reconciliation will fail and stay failed until Traefik is present.

Deploy Traefik using Helm. The `allowCrossNamespace: true` setting is required because the operator creates `IngressRoute` resources in the `posit-team` namespace that reference middlewares in other namespaces:

```yaml
# traefik-values.yaml
service:
  type: LoadBalancer
  annotations:
    # Use a Network Load Balancer instead of a Classic Load Balancer on AWS
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"

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

On AWS, the `LoadBalancer` service type provisions an Elastic Load Balancer automatically. Using the `aws-load-balancer-type: nlb` annotation requests a Network Load Balancer, which preserves client IP addresses and integrates with AWS Certificate Manager for TLS termination if you choose to terminate TLS at the load balancer layer. Without this annotation, AWS provisions a Classic Load Balancer, which works but lacks NLB features.

Once Traefik is running, retrieve the NLB hostname assigned to its LoadBalancer service and create DNS records pointing to it before products become accessible:

```bash
kubectl get svc traefik -n posit-team
```

The `EXTERNAL-IP` column will show an NLB hostname (e.g., `abc123.elb.us-east-1.amazonaws.com`). Create a wildcard CNAME record (or individual records for each product subdomain) pointing to that hostname. Note that NLB hostnames are not static IP addresses — use CNAME records, not A records.

## Step 6: Create the Site CR

With secrets, storage, database, and Traefik in place, you're ready to tell Team Operator what to deploy. The `Site` CR is the single resource the operator watches — everything else (deployments, services, ingress routes, databases) flows from it.

The following example enables Workbench with EFS storage and uses AWS Secrets Manager for credentials. It disables Connect and Package Manager for an initial deployment — starting with one product lets you validate the full stack before enabling additional products:

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

  # AWS account metadata — used to construct IRSA annotations for product service accounts
  awsAccountId: "<ACCOUNT_ID>"
  clusterDate: "20240101"
  workloadCompoundName: "my-workload"

  # Ingress class — must match the Traefik deployment
  ingressClass: traefik

  # Site-level secret (DB passwords, license keys)
  # Use type: aws for Secrets Manager, type: kubernetes for K8s secrets
  secret:
    type: aws
    vaultName: "my-workload.posit.team"

  # Workload secret (main-database-url)
  workloadSecret:
    type: aws
    vaultName: "my-workload-workload-secrets"

  # Database credential secret (username, password for provisioning)
  # For RDS-managed secrets, use the rds! prefix format
  mainDatabaseCredentialSecret:
    type: aws
    vaultName: "rds!db-<your-db-resource-id>"

  # EFS network policy configuration — required when using EFS for home directories
  efsEnabled: true
  vpcCIDR: "10.0.0.0/16"

  # Workbench — enabled with EFS storage for home directories
  workbench:
    enabled: true
    image: "ghcr.io/rstudio/rstudio-workbench:ubuntu2204-2025.12.0"  # Check https://ghcr.io/rstudio/rstudio-workbench for the latest tag
    replicas: 1
    auth:
      type: password  # Change to "oidc" for SSO — see authentication-setup.md
    volume:
      create: true
      size: 100Gi
      storageClassName: efs-sc  # StorageClass from Step 3

  # Connect — disabled for initial deployment
  connect:
    enabled: false

  # Package Manager — disabled for initial deployment
  packageManager:
    enabled: false
```

If you are using Kubernetes secrets instead of Secrets Manager, change all `type: aws` to `type: kubernetes` and set `vaultName` to the Kubernetes secret name you created in Step 2.

Apply the manifest:

```bash
kubectl apply -f site.yaml -n posit-team
```

For full Site spec options including OIDC auth, session images, node selectors, FSx storage, and S3 package storage, see the [Site Management Guide](product-team-site-management.md).

## Step 7: Verify the Deployment

After applying the `Site` CR, the operator begins reconciling — provisioning databases, creating PVCs, deploying pods, and setting up ingress. Reconciliation takes a minute or two on first run. Use the following commands to follow the progress.

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

### EFS Mount Issues

If PVCs are stuck in `Pending` or Workbench pods fail to start with a `MountVolume.SetUp failed` event, the most common causes are missing EFS mount targets, security group restrictions, or EFS CSI driver misconfiguration.

EFS mount targets must exist in each availability zone used by your EKS nodes. If a node's AZ has no mount target, pods scheduled to that node cannot mount the volume. Verify mount target coverage in the AWS console or via the CLI:

```bash
aws efs describe-mount-targets --file-system-id <fs-id>
```

The security group attached to EFS mount targets must allow inbound TCP on port 2049 (NFS) from the EKS node security group. The `efsEnabled: true` and `vpcCIDR` fields in the Site CR control the Kubernetes network policies that allow pod-to-EFS traffic — if those fields are missing, Workbench session pods will be blocked by network policy before even reaching the EFS security group.

Verify the EFS CSI driver is running and check its logs for permission errors:

```bash
kubectl get pods -n kube-system -l app=efs-csi-node
kubectl logs -n kube-system -l app=efs-csi-node -c efs-plugin --tail=50
```

### IRSA Not Working

If the operator logs show authentication errors when reading from Secrets Manager (`NoCredentialProviders` or `AccessDenied`), IRSA is likely misconfigured. The three most common causes are: the service account annotation is missing or incorrect, the IAM role's trust policy does not reference the correct OIDC provider, or the cluster's OIDC provider is not associated with IAM.

Verify the annotation is on the operator service account:

```bash
kubectl get serviceaccount team-operator-controller-manager \
  -n posit-team-system \
  -o jsonpath='{.metadata.annotations}'
```

Confirm the OIDC provider is registered:

```bash
aws eks describe-cluster --name <cluster-name> \
  --query "cluster.identity.oidc.issuer" --output text
```

Then verify that issuer URL appears in the trust policy of your IAM role under `Condition.StringEquals`.

### Security Group Blocking Database Access

If operator logs show `error determining database url`, `dial tcp: i/o timeout`, or `FATAL: password authentication failed`, the connection from the EKS cluster to RDS is either blocked or the credentials are wrong.

First check that the secret contains the correct key name — the workload secret key must be exactly `main-database-url`:

```bash
kubectl get secret workload-secrets -n posit-team \
  -o jsonpath='{.data.main-database-url}' | base64 -d
```

If you are using Secrets Manager, confirm the operator can read the secret by checking operator logs for Secrets Manager API errors. If the secret looks correct but connections still fail, verify the RDS security group has an inbound rule allowing TCP 5432 from the EKS node security group:

```bash
# Get the EKS node security group
aws eks describe-cluster --name <cluster-name> \
  --query "cluster.resourcesVpcConfig.clusterSecurityGroupId" --output text

# Check operator logs for database errors
kubectl logs -n posit-team-system deployment/team-operator-controller-manager \
  --tail=100 | grep -i database
```

### DNS Resolution

If products load but OIDC callbacks fail, or products cannot reach each other by hostname, the issue is usually that DNS records have not been created yet or point to the wrong hostname. NLB hostnames can take a few minutes to resolve after creation — verify DNS propagation from inside the cluster to rule out local configuration as a factor:

```bash
kubectl run -it --rm dns-test --image=busybox --restart=Never -- \
  nslookup workbench.posit.example.com
```

Ensure your DNS records (wildcard CNAME or per-product) resolve to the NLB hostname, and that the NLB is healthy in the AWS console with all targets registered.

## Related Documentation

- [Site Management Guide](product-team-site-management.md) — Full Site spec reference and lifecycle management
- [Authentication Setup](authentication-setup.md) — OIDC, SAML, and Keycloak configuration
- [Workbench Configuration](workbench-configuration.md) — Session images, Databricks, Positron, resource profiles
- [Connect Configuration](connect-configuration.md) — Publishing, off-host execution, GPU settings
- [Package Manager Configuration](packagemanager-configuration.md) — S3 storage, Git sources, IRSA for S3
- [Troubleshooting Guide](troubleshooting.md) — Operator issues, database problems, networking
