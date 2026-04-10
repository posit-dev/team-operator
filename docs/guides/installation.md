---
title: Installation
description: Install Team Operator on your Kubernetes cluster using Helm
---

# Installation

Team Operator is distributed as a Helm chart from the OCI registry at `ghcr.io/posit-dev/charts/team-operator`. This guide walks you through installing the operator on an existing Kubernetes cluster.

## Prerequisites

Before installing, confirm your environment meets these requirements:

- **Kubernetes 1.29+** — the operator relies on CRD features and API versions introduced in 1.29
- **Helm 3.x** — used to install and manage the chart
- **kubectl** — configured to access your target cluster

You will also need a PostgreSQL database, shared storage (NFS, EFS, or Azure NetApp Files), and an ingress controller (Traefik) before creating your first Site. Those components are out of scope for this guide — see the [Site Management](./product-team-site-management.md) guide once the operator is running.

## Install the operator

The chart is hosted on GitHub Container Registry as an OCI artifact. The following command installs the operator into the `posit-team-system` namespace, creating it if it does not exist:

```bash
helm install team-operator oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace
```

For production use, pin to a specific version to avoid unexpected upgrades:

```bash
helm install team-operator oci://ghcr.io/posit-dev/charts/team-operator \
  --version 1.2.0 \
  --namespace posit-team-system \
  --create-namespace
```

The chart installs the controller manager deployment, CRDs, RBAC resources, and a metrics service. By default, the operator watches for Site Custom Resources in the `posit-team` namespace. You can change this with `--set watchNamespace=<your-namespace>`.

## Configure for your cloud

The operator needs cloud credentials to manage resources like storage and secrets on your behalf. Pass cloud-specific configuration via a values file.

### AWS (EKS with IRSA)

On EKS, annotate the operator's ServiceAccount with an IAM role ARN. The role must have permissions to access the AWS services your Site will use (S3, Secrets Manager, RDS, etc.).

```yaml
# values-aws.yaml
controllerManager:
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/team-operator-role"
```

```bash
helm install team-operator oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace \
  --values values-aws.yaml
```

### Azure (AKS with Workload Identity)

On AKS with Workload Identity enabled, the ServiceAccount annotation and a pod label are both required. The `azure.workload.identity/use: "true"` label causes the Azure mutating webhook to inject the necessary token volume into the operator pod.

```yaml
# values-azure.yaml
controllerManager:
  serviceAccount:
    annotations:
      azure.workload.identity/client-id: "<AZURE_CLIENT_ID>"
  pod:
    labels:
      azure.workload.identity/use: "true"
```

```bash
helm install team-operator oci://ghcr.io/posit-dev/charts/team-operator \
  --namespace posit-team-system \
  --create-namespace \
  --values values-azure.yaml
```

For the full Azure infrastructure setup — including the managed identity, federated credential, and storage prerequisites — see the [AKS Deployment](./aks-deployment.md) guide.

## Verify the installation

Once Helm returns, confirm the operator pod is running:

```bash
kubectl get pods -n posit-team-system
```

You should see the controller manager pod with a `Running` status. If it is stuck in `Pending` or `CrashLoopBackOff`, check the logs:

```bash
kubectl logs -n posit-team-system deployment/team-operator-controller-manager
```

Verify the CRDs were installed successfully:

```bash
kubectl get crds | grep posit.team
```

You should see entries for `sites.core.posit.team`, `connects.core.posit.team`, `workbenches.core.posit.team`, and the other Posit Team resource types.

## Next steps

With the operator running, you are ready to create your first Posit Team deployment:

- **[Site Management](./product-team-site-management.md)** — create a Site Custom Resource to deploy Workbench, Connect, Package Manager, and Chronicle
- **[AKS Deployment](./aks-deployment.md)** — Azure-specific infrastructure prerequisites before creating a Site on AKS
