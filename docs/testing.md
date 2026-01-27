# Testing Guide

This document describes the testing infrastructure for the Team Operator.

## Testing Tiers

The Team Operator uses a two-tier local integration testing strategy:

### Tier 1: Envtest (Fast API Tests)

**What it is:** Envtest uses a lightweight, embedded Kubernetes API server (etcd + kube-apiserver) to test controller logic without a full cluster.

**When to use:** For testing controller reconciliation logic, CRD validation, and API interactions.

**Execution time:** Seconds

**What it tests:**
- CRD schema validation
- Controller reconciliation logic
- Resource creation and updates
- Status updates

### Tier 2: Kind Cluster (Full Stack Tests)

**What it is:** Kind (Kubernetes IN Docker) creates a real Kubernetes cluster using Docker containers.

**When to use:** For end-to-end testing, Helm chart deployment, and integration with other Kubernetes components.

**Execution time:** Minutes

**What it tests:**
- Helm chart deployment
- Full operator lifecycle
- Inter-pod communication
- Actual resource creation in Kubernetes

## Prerequisites

### For Envtest

Envtest binaries are automatically downloaded by the Makefile:

```bash
make envtest
```

### For Kind Tests

Install these tools:

```bash
# Install kind
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Install kubectl (if not already installed)
# macOS
brew install kubectl

# Install Helm
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify Docker is running
docker info
```

## Running Tests

### Unit Tests (includes Envtest)

Run all Go tests including envtest-based controller tests:

```bash
make go-test
```

Or run the full test suite with code generation:

```bash
make test
```

### Envtest Tests Only

To run only the Ginkgo-based envtest suite:

```bash
KUBEBUILDER_ASSETS="$(pwd)/$(bin/setup-envtest use 1.29.x --bin-dir bin -p path)" \
  go test -v ./internal/controller/core/... -run "TestControllers"
```

### Kind Integration Tests

Create a kind cluster and run integration tests:

```bash
# Create kind cluster
make kind-create

# Run integration tests
make test-kind

# Clean up
make kind-delete
```

For a full clean run:

```bash
make test-kind-full
```

### All Tests

Run both unit tests and integration tests:

```bash
make test-integration
```

## Test Structure

### Envtest Suite (`internal/controller/core/suite_test.go`)

The envtest suite sets up a test environment with:
- Embedded etcd and kube-apiserver
- All operator CRDs loaded
- A `posit-team` namespace for test resources

Example test file: `internal/controller/core/site_envtest_test.go`

```go
var _ = Describe("Site Controller (envtest)", func() {
    Context("When creating a Site CR", func() {
        It("Should create child resources", func() {
            // Test code using k8sClient from suite_test.go
        })
    })
})
```

### Kind Tests (`hack/test-kind.sh`)

The kind test script:
1. Verifies prerequisites (kind, kubectl, helm)
2. Installs CRDs
3. Deploys the operator via Helm
4. Creates test resources
5. Validates reconciliation
6. Cleans up

## CI Integration

Integration tests run automatically via GitHub Actions:

| Event | Envtest | Kind |
|-------|---------|------|
| Pull Request | Yes | No |
| Push to main | Yes | Yes |
| Nightly schedule | Yes | Yes |
| Manual trigger | Yes | Configurable |

See `.github/workflows/integration-tests.yml` for details.

## Troubleshooting

### Envtest fails with "no such file or directory"

The envtest binaries need to be downloaded:

```bash
make envtest
```

Or ensure KUBEBUILDER_ASSETS is set to an absolute path:

```bash
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/1.29.5-$(go env GOOS)-$(go env GOARCH)"
```

### Kind cluster won't start

Check Docker is running:

```bash
docker info
```

Check for existing clusters:

```bash
kind get clusters
```

Delete and recreate:

```bash
make kind-delete kind-create
```

### Tests hang or timeout

For envtest, ensure no other test environment is running.

For kind, check cluster health:

```bash
kubectl cluster-info --context kind-team-operator-test
kubectl get pods -A
```

## Writing New Tests

### Adding Envtest Tests

1. Use the existing `suite_test.go` setup
2. Create a new `*_test.go` file with Ginkgo `Describe` blocks
3. Use `k8sClient` for API operations
4. Use `ctx` for context
5. Clean up resources after each test

### Adding Kind Tests

1. Add test functions to `hack/test-kind.sh`
2. Follow the naming convention: `test_<feature>`
3. Use the helper functions (`log_info`, `wait_for`, etc.)
4. Ensure proper cleanup in the `cleanup` function

## Best Practices

1. **Use envtest for unit-level controller tests** - It's fast and doesn't require Docker
2. **Use kind for integration tests** - When you need a real cluster
3. **Always clean up test resources** - Prevents test pollution
4. **Use Eventually() for async operations** - Controllers are eventually consistent
5. **Keep test data minimal** - Only specify fields needed for the test
