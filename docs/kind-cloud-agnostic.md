# Cloud-Agnostic Development with kind

This guide explains how to run the team-operator locally on kind using cloud-agnostic configuration fields instead of cloud-specific ones (AWS/Azure).

## What is Cloud-Agnostic Mode?

Cloud-agnostic mode means the team-operator interacts only with Kubernetes-native APIs, without direct knowledge of AWS, Azure, or other cloud providers. This enables:

- **Local development** on kind without cloud credentials
- **Easier testing** with standard K8s resources
- **Cloud portability** by delegating cloud-specific logic to the infrastructure layer

### Key Differences

| Aspect | Cloud-Specific (Old) | Cloud-Agnostic (New) |
|--------|---------------------|----------------------|
| **Storage** | `volumeSource: {type: "nfs", dnsName: "..."}` | `storageClassName: "standard"` |
| **Secrets** | `secret: {type: "aws", vaultName: "arn:..."}` | `secret: {name: "dev-secrets"}` |
| **IAM** | `awsAccountId`, `clusterDate`, computed IRSA ARN | `serviceAccountName: "dev-connect"` |
| **Ingress** | `Ingress` + Traefik `Middleware` CRDs | `gatewayRef` + `HTTPRoute` (Gateway API) |

## Prerequisites

Install these tools:

```bash
brew install kind kubectl helm
```

## Quick Start

### 1. Set up the kind cluster

```bash
just kind-cloud-agnostic
```

This script:
- Creates a kind cluster with port mappings (host 80→node 30080, host 443→node 30443)
- Installs Gateway API CRDs (v1.2+)
- Deploys Traefik with Gateway API provider
- Creates a Gateway resource
- Creates test namespaces and secrets

### 2. Build and deploy the operator

```bash
# Build the operator binary
just build

# Install the Helm chart
just helm-install
```

### 3. Apply the sample Site CR

```bash
kubectl apply -f hack/kind-cloud-agnostic-site.yaml
```

This creates a Site named `dev` in the `posit-team` namespace with:
- Storage using kind's `standard` StorageClass (local-path-provisioner)
- Secrets referencing K8s Secret names (not AWS/Azure vaults)
- Explicit ServiceAccount names (no IAM annotations)
- Gateway API routes (not Ingress)

### 4. Verify the deployment

```bash
# Check Site status
kubectl get site dev -n posit-team
kubectl describe site dev -n posit-team

# Check product CRs (Connect, Workbench, etc.)
kubectl get connects,workbenches,packagemanagers -n posit-team

# View operator logs
kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator -f

# Check HTTPRoutes (Gateway API)
kubectl get httproutes -n posit-team

# Check Gateway status
kubectl get gateway posit-team -n traefik
```

## What Gets Created

### Infrastructure (by setup script)

- **kind cluster**: `team-operator-cloud-agnostic`
- **Gateway API CRDs**: GatewayClass, Gateway, HTTPRoute
- **Traefik**: Deployed with Gateway API provider enabled
- **Gateway resource**: `posit-team` in `traefik` namespace
- **K8s Secrets**: Test secrets in `posit-team` namespace

### Site Resources (by operator)

When you apply the sample Site CR, the operator creates:

- **Product CRs**: Connect, Workbench, PackageManager, Chronicle, Flightdeck
- **PVCs**: One per product, using `storageClassName: standard`
- **Services**: One per product
- **HTTPRoutes**: One per product URL, attached to the Gateway
- **ServiceAccounts**: Named explicitly (e.g., `dev-connect`, `dev-workbench`)
- **ConfigMaps**: Product configuration
- **Deployments**: Product pods

### What's NOT Created (Cloud-Agnostic Mode)

- **No PersistentVolumes** — The StorageClass provisioner creates them automatically
- **No AWS FSx/NFS volumes** — Kind uses local storage
- **No AWS Secrets Manager calls** — Operator reads K8s Secrets
- **No SecretProviderClass** — Not needed, K8s Secrets are mounted directly
- **No IRSA annotations** — ServiceAccounts have no IAM bindings
- **No Ingress resources** — HTTPRoute is used instead
- **No Traefik Middleware CRDs** — Header manipulation is done via HTTPRoute filters

## Configuration Files

### Site CR: `hack/kind-cloud-agnostic-site.yaml`

The sample Site CR uses only cloud-agnostic fields:

```yaml
spec:
  domain: dev.localhost
  storageClassName: standard              # kind's local-path-provisioner
  nfsEgressCIDR: ""                       # No NFS access needed
  gatewayRef:                             # Gateway API reference
    name: posit-team
    namespace: traefik
  secret:
    name: dev-secrets                     # K8s Secret name
  workloadSecret:
    name: dev-workload-secrets
  mainDatabaseCredentialSecret:
    name: dev-db-creds

  connect:
    serviceAccountName: dev-connect       # Explicit SA name
    url: connect.dev.localhost
    # ... other product config
```

### Secrets: `hack/kind-cloud-agnostic-secrets.yaml`

Test secrets with dummy values:

- `dev-secrets` — Product secrets (license keys, tokens)
- `dev-workload-secrets` — Workload secrets (API keys)
- `dev-db-creds` — Database credentials
- `license` — License files (empty for testing)

### Setup Script: `hack/kind-cloud-agnostic.sh`

Usage:

```bash
# Create cluster and infrastructure
./hack/kind-cloud-agnostic.sh

# Delete cluster
./hack/kind-cloud-agnostic.sh --delete
```

## Testing Workflow

### Basic Reconciliation Test

```bash
# Apply Site CR
kubectl apply -f hack/kind-cloud-agnostic-site.yaml

# Watch reconciliation
kubectl get site dev -n posit-team -w

# Check product CRs were created
kubectl get connects,workbenches -n posit-team

# Verify PVCs were created
kubectl get pvc -n posit-team

# Check HTTPRoutes
kubectl get httproutes -n posit-team
```

### Iterative Development

```bash
# Make code changes
vim internal/controllers/site_controller.go

# Rebuild and redeploy
just build
just helm-uninstall
just helm-install

# Apply test Site
kubectl apply -f hack/kind-cloud-agnostic-site.yaml

# Check logs
kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator -f
```

### Cleaning Up

```bash
# Delete Site CR
kubectl delete site dev -n posit-team

# Or tear down entire cluster
just kind-cloud-agnostic-delete
```

## Troubleshooting

### Gateway not ready

```bash
kubectl get gateway posit-team -n traefik
kubectl describe gateway posit-team -n traefik
```

If Gateway is stuck, check Traefik pods:

```bash
kubectl get pods -n traefik
kubectl logs -n traefik -l app.kubernetes.io/name=traefik
```

### HTTPRoutes not routing traffic

```bash
kubectl get httproutes -n posit-team
kubectl describe httproute <name> -n posit-team
```

### PVCs stuck in Pending

```bash
kubectl get pvc -n posit-team
kubectl describe pvc <name> -n posit-team
```

Check if StorageClass exists:

```bash
kubectl get storageclass
```

Kind's `standard` StorageClass should be present by default.

### Operator not starting

```bash
kubectl get pods -n posit-team-system
kubectl describe pod <pod-name> -n posit-team-system
kubectl logs -n posit-team-system <pod-name>
```

Common issues:
- CRDs not installed (`just install`)
- Helm chart misconfigured (`just helm-lint`)
- Image not loaded into kind (`make docker-build` + `kind load docker-image`)

## Differences from Production

| Feature | kind (Local) | Production (AWS/Azure) |
|---------|-------------|----------------------|
| **Storage** | local-path-provisioner | nfs-subdir-external-provisioner (FSx/NetApp) |
| **Secrets** | K8s Secrets (created manually) | K8s Secrets (synced via external-secrets-operator) |
| **IAM** | No annotations | EKS Pod Identity (AWS) or Workload Identity (Azure) |
| **TLS** | Self-signed cert | ACM/Let's Encrypt cert |
| **Database** | Optional external postgres | RDS/Azure Database |

## Next Steps

- Read the cloud-agnostic design doc (located at `thoughts/shared/plans/2026-02-26-cloud-agnostic-team-operator.md` in the workspace root)
- Explore the [operator source code](../internal/controllers/)
- Run integration tests: `just test`
- Build for production: `just docker-build`

## References

- [Gateway API docs](https://gateway-api.sigs.k8s.io/)
- [Traefik Gateway API](https://doc.traefik.io/traefik/providers/kubernetes-gateway/)
- [kind documentation](https://kind.sigs.k8s.io/)
- [local-path-provisioner](https://github.com/rancher/local-path-provisioner)
