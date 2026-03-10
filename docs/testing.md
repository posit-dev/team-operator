# Testing Guide

This document covers the testing infrastructure for the Team Operator.

## Testing Tiers

The Team Operator uses a two-tier local integration testing strategy:

### Tier 1: Envtest (Fast API Tests)

**What it is:** Envtest uses a lightweight, embedded Kubernetes API server (etcd + kube-apiserver) to test CRD schema and API storage without a full cluster or running controller.

**When to use:** For testing CRD schema validation and API storage. No controller reconciler runs in the test environment.

**Execution time:** Seconds

**What it tests:**
- CRD schema validation
- API object creation and storage
- Resource serialization/deserialization
- Basic CRUD operations through the API

### Tier 2: Kind Cluster (Full Stack Tests)

**What it is:** Kind (Kubernetes IN Docker) creates a real Kubernetes cluster with Docker containers.

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
        It("Should be able to create and retrieve a Site CR", func() {
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

#### Development Loop

For iterative development, keep the kind cluster running between test runs. Don't recreate it each time.

**Initial setup** (run once):

```bash
make kind-setup
```

This creates the cluster, builds the operator image, loads it into kind, and deploys it via Helm.

**After making code changes**, rebuild and redeploy:

```bash
make kind-setup   # rebuilds image, reloads into kind, helm upgrade
```

**Run tests** against the live cluster:

```bash
make kind-test
```

**Tear down** when done:

```bash
make kind-teardown   # removes Helm release and namespaces
                     # (also deletes the kind cluster)
```

This workflow is much faster than `make test-kind` for iterative development. It skips cluster creation and deletion on every run.

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

1. **Use envtest for unit-level controller tests** - Fast and no Docker needed
2. **Use kind for integration tests** - When you need a real cluster
3. **Always clean up test resources** - Prevents test pollution
4. **Use Eventually() for async operations** - Controllers are eventually consistent
5. **Keep test data minimal** - Specify only the fields the test needs
