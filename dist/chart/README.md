# Team Operator Helm Chart

A Helm chart for deploying the Team Operator, a Kubernetes operator that manages the deployment and lifecycle of Posit Team products (Workbench, Connect, Package Manager, and Chronicle) within Kubernetes clusters.

> **Warning**
> This operator is under active development and is not yet ready for production use. Please [contact Posit](https://posit.co/schedule-a-call/) before using this operator.

## Overview

The Team Operator automates the deployment, configuration, and management of Posit Team products. It handles:

- Multi-product Posit Team deployments through a single `Site` Custom Resource
- Database provisioning and management for each product
- Secure credential management via Kubernetes secrets or AWS Secrets Manager
- License configuration and validation
- Ingress routing and load balancing
- Shared storage configuration across products
- Keycloak integration for authentication
- Off-host execution support for Workbench and Connect

## Prerequisites

- Kubernetes 1.29+
- Helm 3.x
- kubectl configured to access your cluster
- (Optional) cert-manager for TLS certificate management
- (Optional) Prometheus Operator for metrics collection

## What Gets Installed

This chart installs the following resources:

| Resource Type | Description |
|---------------|-------------|
| Deployment | Controller manager that runs the operator |
| ServiceAccount | Identity for the operator pod |
| ClusterRole | Cluster-wide permissions (PersistentVolumes) |
| Role | Namespace-scoped permissions for managed resources |
| ClusterRoleBinding | Binds ClusterRole to ServiceAccount |
| RoleBinding | Binds Role to ServiceAccount |
| Service | Metrics endpoint for the operator |
| CRDs | Custom Resource Definitions for Posit Team resources |

### Custom Resource Definitions (CRDs)

The chart installs the following CRDs:

- `sites.core.posit.team` - Top-level resource for Posit Team deployments
- `workbenches.core.posit.team` - RStudio Workbench instances
- `connects.core.posit.team` - Posit Connect instances
- `packagemanagers.core.posit.team` - Posit Package Manager instances
- `chronicles.core.posit.team` - Chronicle instances
- `flightdecks.core.posit.team` - Landing page dashboard
- `postgresdatabases.core.posit.team` - Database provisioning

## Installation

### Basic Installation

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace
```

### Installation with Custom Values

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace \
  --values my-values.yaml
```

### Installation with Inline Overrides

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace \
  --set controllerManager.container.image.repository=posit/team-operator \
  --set controllerManager.container.image.tag=v1.2.0 \
  --set watchNamespace=my-posit-team
```

### Upgrading

```bash
helm upgrade team-operator ./dist/chart \
  --namespace posit-team-system \
  --values my-values.yaml
```

### Uninstalling

```bash
helm uninstall team-operator --namespace posit-team-system
```

> **Note**: By default, CRDs are preserved after uninstallation due to the `crd.keep: true` setting. To remove CRDs, set `crd.keep: false` before uninstalling or manually delete them:
> ```bash
> kubectl delete crd sites.core.posit.team workbenches.core.posit.team connects.core.posit.team packagemanagers.core.posit.team chronicles.core.posit.team flightdecks.core.posit.team postgresdatabases.core.posit.team
> ```

## Configuration Reference

### Global Settings

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `watchNamespace` | Namespace where the operator watches for Site CRs | `posit-team` | No |

### Controller Manager

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `controllerManager.replicas` | Number of operator replicas | `1` | No |
| `controllerManager.serviceAccountName` | Name of the ServiceAccount | `team-operator-controller-manager` | No |
| `controllerManager.terminationGracePeriodSeconds` | Grace period for pod termination | `10` | No |
| `controllerManager.tolerations` | Pod tolerations for scheduling | `[]` | No |
| `controllerManager.nodeSelector` | Node selector for pod placement | `{}` | No |

### Controller Manager Container

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `controllerManager.container.image.repository` | Operator image repository | `posit/team-operator` | No |
| `controllerManager.container.image.tag` | Operator image tag | `latest` | No |
| `controllerManager.container.args` | Container arguments | See values.yaml | No |
| `controllerManager.container.env` | Environment variables | See values.yaml | No |
| `controllerManager.container.resources.limits.cpu` | CPU limit | `500m` | No |
| `controllerManager.container.resources.limits.memory` | Memory limit | `128Mi` | No |
| `controllerManager.container.resources.requests.cpu` | CPU request | `10m` | No |
| `controllerManager.container.resources.requests.memory` | Memory request | `64Mi` | No |

### Controller Manager Probes

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `controllerManager.container.livenessProbe.initialDelaySeconds` | Initial delay for liveness probe | `15` | No |
| `controllerManager.container.livenessProbe.periodSeconds` | Period for liveness probe | `20` | No |
| `controllerManager.container.livenessProbe.httpGet.path` | Liveness probe path | `/healthz` | No |
| `controllerManager.container.livenessProbe.httpGet.port` | Liveness probe port | `8081` | No |
| `controllerManager.container.readinessProbe.initialDelaySeconds` | Initial delay for readiness probe | `5` | No |
| `controllerManager.container.readinessProbe.periodSeconds` | Period for readiness probe | `10` | No |
| `controllerManager.container.readinessProbe.httpGet.path` | Readiness probe path | `/readyz` | No |
| `controllerManager.container.readinessProbe.httpGet.port` | Readiness probe port | `8081` | No |

### Controller Manager Security Context

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `controllerManager.container.securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` | No |
| `controllerManager.container.securityContext.capabilities.drop` | Capabilities to drop | `["ALL"]` | No |
| `controllerManager.securityContext.runAsNonRoot` | Run as non-root user | `true` | No |
| `controllerManager.securityContext.seccompProfile.type` | Seccomp profile type | `RuntimeDefault` | No |

### Service Account

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `controllerManager.serviceAccount.annotations` | Annotations for the ServiceAccount (e.g., for IAM roles) | `{}` | No |

### RBAC

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `rbac.enable` | Enable RBAC resources | `true` | No |

### CRDs

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `crd.enable` | Install CRDs with the chart | `true` | No |
| `crd.keep` | Keep CRDs when chart is uninstalled | `true` | No |

### Metrics

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `metrics.enable` | Enable metrics endpoint | `true` | No |

### Prometheus

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `prometheus.enable` | Enable ServiceMonitor for Prometheus | `false` | No |

### Webhooks

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `webhook.enable` | Enable admission webhooks | `false` | No |

### Cert-Manager

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `certmanager.enable` | Enable cert-manager for TLS certificates | `false` | No |

### Network Policies

| Parameter | Description | Default | Required |
|-----------|-------------|---------|----------|
| `networkPolicy.enable` | Enable NetworkPolicies | `false` | No |

## Examples

### AWS Deployment with EKS IAM Roles

For AWS deployments using IAM Roles for Service Accounts (IRSA):

```yaml
# aws-values.yaml
watchNamespace: posit-team

controllerManager:
  container:
    image:
      repository: posit/team-operator
      tag: v1.2.0
    env:
      WATCH_NAMESPACES: "posit-team"
      AWS_REGION: "us-east-1"
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/team-operator-role"
```

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace \
  --values aws-values.yaml
```

### Azure Deployment with AKS Workload Identity

For Azure deployments using Workload Identity:

```yaml
# azure-values.yaml
watchNamespace: posit-team

controllerManager:
  container:
    image:
      repository: posit/team-operator
      tag: v1.2.0
    env:
      WATCH_NAMESPACES: "posit-team"
  serviceAccount:
    annotations:
      azure.workload.identity/client-id: "<AZURE_CLIENT_ID>"
  pod:
    labels:
      azure.workload.identity/use: "true"
```

```bash
helm install team-operator ./dist/chart \
  --namespace posit-team-system \
  --create-namespace \
  --values azure-values.yaml
```

### Custom Resource Limits

For production deployments with increased resource limits:

```yaml
# production-values.yaml
controllerManager:
  container:
    resources:
      limits:
        cpu: "1"
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 128Mi
```

### Multi-Namespace Watching

To watch multiple namespaces for Site CRs:

```yaml
# multi-namespace-values.yaml
watchNamespace: posit-team

controllerManager:
  container:
    env:
      WATCH_NAMESPACES: "posit-team,posit-team-staging,posit-team-prod"
```

> **Note**: The operator needs appropriate RBAC permissions in each watched namespace.

### Node Selector and Tolerations

To schedule the operator on specific nodes:

```yaml
# node-placement-values.yaml
controllerManager:
  nodeSelector:
    kubernetes.io/os: linux
    node-role.kubernetes.io/control-plane: ""
  tolerations:
    - key: "node-role.kubernetes.io/control-plane"
      operator: "Exists"
      effect: "NoSchedule"
```

### Enabling Prometheus Metrics

To enable Prometheus ServiceMonitor with cert-manager for secure metrics:

```yaml
# prometheus-values.yaml
metrics:
  enable: true

prometheus:
  enable: true

certmanager:
  enable: true
```

## RBAC Permissions

The operator requires the following permissions:

### Cluster-Wide (ClusterRole)

- **PersistentVolumes**: Full CRUD access for shared storage provisioning

### Namespace-Scoped (Role)

- **Core resources**: ConfigMaps, PVCs, Pods, Secrets, ServiceAccounts, Services
- **Apps**: Deployments, StatefulSets, DaemonSets
- **Batch**: Jobs
- **Networking**: Ingresses, NetworkPolicies
- **Policy**: PodDisruptionBudgets
- **RBAC**: Roles, RoleBindings
- **Posit Team CRDs**: Sites, Workbenches, Connects, PackageManagers, Chronicles, Flightdecks, PostgresDatabases
- **Keycloak**: Keycloaks, KeycloakRealmImports (for authentication)
- **Traefik**: Middlewares (for ingress routing)
- **Secrets Store CSI**: SecretProviderClasses (for external secrets)

## Metrics and Monitoring

The operator exposes metrics on port 8443 at the `/metrics` endpoint. When `prometheus.enable` is set to `true`, a ServiceMonitor resource is created for automatic scraping by Prometheus Operator.

### Metrics Endpoint Security

- Without cert-manager: Metrics are served with insecure TLS (for development/testing)
- With cert-manager: Metrics are served with proper TLS certificates

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager
```

### Check Operator Status

```bash
kubectl get pods -n posit-team-system
kubectl describe deployment -n posit-team-system team-operator-controller-manager
```

### Check Site Status

```bash
kubectl describe site -n posit-team <site-name>
```

### Verify CRDs are Installed

```bash
kubectl get crds | grep posit.team
```

## Links

- [Source Code](https://github.com/posit-dev/team-operator)
- [Posit Team Documentation](https://docs.posit.co/)

## License

MIT License - see [LICENSE](../../../LICENSE) file for details.

Copyright (c) 2023-2026 Posit Software, PBC
