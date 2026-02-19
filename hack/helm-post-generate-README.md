# Helm Chart Post-Generation Script

## Overview

The `helm-post-generate.sh` script applies customizations to the Helm chart after the kubebuilder `helm/v2-alpha` plugin regenerates templates. This ensures that our custom configurations are not lost during chart regeneration.

## Why Is This Needed?

The kubebuilder `helm/v2-alpha` plugin (see [issue #5486](https://github.com/kubernetes-sigs/kubebuilder/issues/5486)) has several limitations and bugs that cause it to:

1. Not template certain fields (e.g., `replicas: 1` is hardcoded)
2. Miss support for optional features (e.g., `imagePullSecrets`, cert-manager volumes)
3. Not support ServiceAccount annotations
4. Not create namespace-scoped RoleBindings

This script fixes all these issues automatically after each regeneration.

## What It Fixes

### 1. values.yaml - Duplicate Args
**Problem**: The v2-alpha plugin now handles `--metrics-bind-address` and `--health-probe-bind-address` in the template, but leaves duplicate entries in values.yaml.

**Fix**: Removes the duplicate args, leaving only `--leader-elect`.

### 2. manager.yaml - Replicas Field
**Problem**: The plugin generates `replicas: 1` as a hardcoded value.

**Fix**: Replaces with `replicas: {{ .Values.manager.replicas }}` to make it configurable.

### 3. manager.yaml - imagePullSecrets
**Problem**: No support for imagePullSecrets in the pod spec.

**Fix**: Adds conditional imagePullSecrets support:
```yaml
{{- with .Values.manager.imagePullSecrets }}
imagePullSecrets: {{ toYaml . | nindent 12 }}
{{- end }}
```

### 4. manager.yaml - cert-manager Volumes
**Problem**: No support for cert-manager webhook certificates.

**Fix**: Adds conditional volumeMounts and volumes when `certManager.enable` is true:
- volumeMounts in the container for `/tmp/k8s-webhook-server/serving-certs`
- volumes at the pod level referencing the webhook-server-cert secret

### 5. controller-manager.yaml - ServiceAccount Annotations
**Problem**: No way to add annotations to the ServiceAccount (needed for IAM roles, etc.).

**Fix**: Adds annotation support:
```yaml
{{- with .Values.manager.serviceAccount }}
{{- with .annotations }}
annotations:
  {{- toYaml . | nindent 6 }}
{{- end }}
{{- end }}
```

### 6. manager-rolebinding.yaml - Namespace-Scoped RoleBinding
**Problem**: Only creates a ClusterRoleBinding, no namespace-scoped RoleBinding.

**Fix**: Appends a namespace-scoped RoleBinding that binds the manager-role to the controller-manager ServiceAccount in the `watchNamespace`.

## Usage

This script is called automatically by `make helm-generate`. You should not need to run it manually.

However, if you need to test it:

```bash
# Ensure SED is set (gsed on macOS, sed on Linux)
export SED=gsed  # or just 'sed' on Linux

# Run the script
./hack/helm-post-generate.sh
```

## Idempotency

The script is designed to be idempotent - running it multiple times produces the same result. This is verified by the following test:

```bash
# First generation
make helm-generate

# Stage changes
git add dist/chart/

# Second generation
make helm-generate

# Verify no changes
git diff dist/chart/  # Should be empty
```

## Future Improvements

When kubebuilder fixes these issues upstream, we can remove the corresponding sections from this script. Track progress at:
- https://github.com/kubernetes-sigs/kubebuilder/issues/5486
