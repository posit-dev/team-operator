#!/bin/bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2023-2026 Posit Software, PBC
#
# test-kind.sh - Integration tests on a kind cluster
#
# This script runs integration tests against a kind cluster to verify
# the team-operator can:
# 1. Deploy successfully via Helm
# 2. Create and reconcile Site CRs
# 3. Clean up properly
#
# Usage:
#   ./hack/test-kind.sh <cluster-name> [mode]
#
# Modes:
#   full      (default) Create cluster, deploy, test, and clean up
#   setup     Deploy operator to an existing cluster (or create if needed)
#   test      Run tests against an already-deployed cluster
#   teardown  Clean up namespaces and Helm release (cluster remains)

set -euo pipefail

CLUSTER_NAME="${1:-team-operator-test}"
MODE="${2:-full}"
# Strip --mode= prefix if provided as --mode=setup
if [[ "${MODE}" == --mode=* ]]; then
    MODE="${MODE#--mode=}"
fi
NAMESPACE="posit-team-system"
RELEASE_NAME="team-operator"
CHART_DIR="dist/chart"
# Use a non-latest tag so Kubernetes defaults to imagePullPolicy=IfNotPresent,
# which uses the locally loaded image instead of pulling from a registry.
LOCAL_IMAGE="controller:kind-test"
TIMEOUT="120s"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Ensure kubectl is using the right context
ensure_context() {
    local expected_context="kind-${CLUSTER_NAME}"
    local current_context
    current_context=$(kubectl config current-context 2>/dev/null || echo "")

    if [[ "$current_context" != "$expected_context" ]]; then
        log_info "Switching kubectl context to ${expected_context}"
        kubectl config use-context "$expected_context"
    fi
}

# Wait for a condition with timeout
wait_for() {
    local description="$1"
    local timeout="$2"
    shift 2
    local cmd=("$@")

    log_info "Waiting for: ${description} (timeout: ${timeout})"

    local end_time=$((SECONDS + ${timeout%s}))
    while [[ $SECONDS -lt $end_time ]]; do
        if "${cmd[@]}" &>/dev/null; then
            log_info "Success: ${description}"
            return 0
        fi
        sleep 2
    done

    log_error "Timeout waiting for: ${description}"
    return 1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing=()

    command -v kind &>/dev/null || missing+=("kind")
    command -v kubectl &>/dev/null || missing+=("kubectl")
    command -v helm &>/dev/null || missing+=("helm")

    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing[*]}"
        log_error "Please install them before running this script."
        exit 1
    fi

    # Check if kind cluster exists
    if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_error "Kind cluster '${CLUSTER_NAME}' does not exist"
        log_error "Run 'make kind-create' first"
        exit 1
    fi

    log_info "Prerequisites check passed"
}

# Install CRDs
install_crds() {
    log_info "Installing CRDs..."
    kubectl apply -f config/crd/bases/
    log_info "CRDs installed"
}

# Deploy the operator via Helm
deploy_operator() {
    log_info "Deploying team-operator via Helm..."

    # Create both namespaces: operator system namespace and the watched namespace
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    kubectl create namespace posit-team --dry-run=client -o yaml | kubectl apply -f -

    # Load the locally built image into kind with a non-latest tag.
    # Using a non-latest tag causes Kubernetes to default to imagePullPolicy=IfNotPresent,
    # so it uses the locally loaded image instead of attempting to pull from a registry.
    docker tag controller:latest "${LOCAL_IMAGE}"
    kind load docker-image "${LOCAL_IMAGE}" --name "${CLUSTER_NAME}"

    local image_repo="${LOCAL_IMAGE%%:*}"
    local image_tag="${LOCAL_IMAGE##*:}"

    # Install or upgrade the operator using the chart's value path
    helm upgrade --install "${RELEASE_NAME}" "${CHART_DIR}" \
        --namespace "${NAMESPACE}" \
        --set "controllerManager.container.image.repository=${image_repo}" \
        --set "controllerManager.container.image.tag=${image_tag}" \
        --wait \
        --timeout "${TIMEOUT}" || {
            log_warn "Helm install failed, checking pod status..."
            kubectl get pods -n "${NAMESPACE}" -o wide
            kubectl describe pods -n "${NAMESPACE}"
            return 1
        }

    log_info "Operator deployed successfully"
}

# Wait for operator to be ready
wait_for_operator() {
    log_info "Waiting for operator to be ready..."

    wait_for "operator deployment ready" "${TIMEOUT}" \
        kubectl rollout status deployment/"${RELEASE_NAME}-controller-manager" -n "${NAMESPACE}"

    # Additional check for pod readiness
    local pod_name
    pod_name=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=team-operator" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [[ -n "$pod_name" ]]; then
        log_info "Operator pod: ${pod_name}"
        kubectl logs -n "${NAMESPACE}" "${pod_name}" --tail=20 || true
    fi
}

# Test: Verify CRDs are installed
test_crds_installed() {
    log_info "Testing: CRDs are installed..."

    local crds=("sites.core.posit.team" "connects.core.posit.team" "workbenches.core.posit.team" "packagemanagers.core.posit.team")
    local failed=()

    for crd in "${crds[@]}"; do
        if kubectl get crd "$crd" &>/dev/null; then
            log_info "  CRD found: $crd"
        else
            failed+=("$crd")
        fi
    done

    if [[ ${#failed[@]} -gt 0 ]]; then
        log_error "Missing CRDs: ${failed[*]}"
        return 1
    fi

    log_info "Test passed: All CRDs installed"
}

# Test: Create a minimal Site CR
test_create_site() {
    log_info "Testing: Create Site CR..."

    local test_namespace="posit-team"
    local site_name="test-site-kind"

    # Create test namespace
    kubectl create namespace "${test_namespace}" --dry-run=client -o yaml | kubectl apply -f -

    # Create a minimal Site CR
    cat <<EOF | kubectl apply -f -
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: ${site_name}
  namespace: ${test_namespace}
spec:
  domain: "test.example.com"
  flightdeck:
    image: "nginx:latest"
  workloadSecret:
    vaultName: "test-workload-vault"
    type: test
  mainDatabaseCredentialSecret:
    vaultName: "test-db-vault"
    type: test
EOF

    log_info "Site CR created"

    # Wait for the Site to exist
    wait_for "Site CR to be created" "30s" \
        kubectl get site "${site_name}" -n "${test_namespace}"

    # Show the Site status
    kubectl get site "${site_name}" -n "${test_namespace}" -o yaml || true

    log_info "Test passed: Site CR created"

    # Cleanup
    kubectl delete site "${site_name}" -n "${test_namespace}" --ignore-not-found
    log_info "Site CR cleaned up"
}

# Test: Verify operator reconciled the Site
test_reconciliation() {
    log_info "Testing: Site reconciliation..."

    local test_namespace="posit-team"
    local site_name="test-site-reconcile"

    # Create test namespace
    kubectl create namespace "${test_namespace}" --dry-run=client -o yaml | kubectl apply -f -

    # Create a Site CR
    cat <<EOF | kubectl apply -f -
apiVersion: core.posit.team/v1beta1
kind: Site
metadata:
  name: ${site_name}
  namespace: ${test_namespace}
spec:
  domain: "test.example.com"
  flightdeck:
    image: "nginx:latest"
  workloadSecret:
    vaultName: "test-workload-vault"
    type: test
  mainDatabaseCredentialSecret:
    vaultName: "test-db-vault"
    type: test
EOF

    log_info "Site CR created, waiting for reconciliation..."

    # Wait for child CRs to be created by the controller.
    # The Site controller creates Connect and Workbench CRs with the same name as the Site.
    local timeout=60
    local end_time=$((SECONDS + timeout))

    while [[ $SECONDS -lt $end_time ]]; do
        local connect_exists=false
        local workbench_exists=false

        if kubectl get connect "${site_name}" -n "${test_namespace}" &>/dev/null; then
            connect_exists=true
        fi

        if kubectl get workbench "${site_name}" -n "${test_namespace}" &>/dev/null; then
            workbench_exists=true
        fi

        if [[ "$connect_exists" == true ]] && [[ "$workbench_exists" == true ]]; then
            log_info "Child CRs created successfully"
            break
        fi

        sleep 2
    done

    # Assert child CRs exist — fail if reconciliation did not produce them
    local failed=false

    if kubectl get connect "${site_name}" -n "${test_namespace}" &>/dev/null; then
        log_info "  Connect CR found: ${site_name}"
        kubectl get connect "${site_name}" -n "${test_namespace}" -o jsonpath='{.status}' || true
    else
        log_error "  Connect CR not found: ${site_name} (reconciliation may not have run)"
        failed=true
    fi

    if kubectl get workbench "${site_name}" -n "${test_namespace}" &>/dev/null; then
        log_info "  Workbench CR found: ${site_name}"
        kubectl get workbench "${site_name}" -n "${test_namespace}" -o jsonpath='{.status}' || true
    else
        log_error "  Workbench CR not found: ${site_name} (reconciliation may not have run)"
        failed=true
    fi

    if [[ "$failed" == true ]]; then
        log_error "Test failed: Site reconciliation did not produce expected child CRs"
        return 1
    fi

    log_info "Test passed: Site reconciliation verified"

    # Cleanup
    kubectl delete site "${site_name}" -n "${test_namespace}" --ignore-not-found
    kubectl delete connect "${site_name}" -n "${test_namespace}" --ignore-not-found
    kubectl delete workbench "${site_name}" -n "${test_namespace}" --ignore-not-found
    log_info "Site and child CRs cleaned up"
}

# Test: Check operator logs for errors
test_operator_logs() {
    log_info "Testing: Operator logs..."

    local pod_name
    pod_name=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/name=team-operator" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [[ -z "$pod_name" ]]; then
        log_warn "Operator pod not found, skipping log check"
        return 0
    fi

    log_info "Operator pod: ${pod_name}"

    # Get logs and check for common error patterns
    local logs
    logs=$(kubectl logs -n "${NAMESPACE}" "${pod_name}" --tail=100 2>&1 || echo "")

    # Check for panic
    if echo "$logs" | grep -i "panic:" &>/dev/null; then
        log_error "Found panic in operator logs"
        echo "$logs" | grep -A 10 -i "panic:"
        return 1
    fi

    # Check for reconciliation activity
    if echo "$logs" | grep -i "reconcil" &>/dev/null; then
        log_info "Operator is reconciling resources"
    fi

    # Show recent reconciliation messages
    echo "$logs" | grep -E "Site found|Site not found|reconcil" | tail -10 || true

    log_info "Test passed: Operator logs look healthy"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up..."

    # Uninstall Helm release
    helm uninstall "${RELEASE_NAME}" -n "${NAMESPACE}" --ignore-not-found || true

    # Delete namespace
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found || true
    kubectl delete namespace "posit-team" --ignore-not-found || true

    log_info "Cleanup completed"
}

# Main test runner
main() {
    log_info "Starting integration tests on kind cluster '${CLUSTER_NAME}' (mode: ${MODE})..."

    check_prerequisites
    ensure_context

    case "${MODE}" in
        setup)
            if [[ -d "${CHART_DIR}" ]]; then
                deploy_operator
                wait_for_operator
            else
                install_crds
                log_warn "Helm chart not found at ${CHART_DIR}, skipping operator deployment"
            fi
            log_info "Kind cluster is ready. Run 'make kind-test' to execute tests."
            ;;
        test)
            test_crds_installed
            if [[ -d "${CHART_DIR}" ]]; then
                test_operator_logs
                test_reconciliation
            fi
            test_create_site
            log_info ""
            log_info "=========================================="
            log_info "All integration tests passed!"
            log_info "=========================================="
            ;;
        teardown)
            cleanup
            ;;
        full)
            trap cleanup EXIT
            if [[ -d "${CHART_DIR}" ]]; then
                deploy_operator
                wait_for_operator
                test_crds_installed
                test_operator_logs
                test_reconciliation
            else
                install_crds
                test_crds_installed
                log_warn "Helm chart not found at ${CHART_DIR}, skipping operator deployment tests"
            fi
            test_create_site
            log_info ""
            log_info "=========================================="
            log_info "All integration tests passed!"
            log_info "=========================================="
            ;;
        *)
            log_error "Unknown mode: ${MODE}. Valid modes: setup, test, teardown, full"
            exit 1
            ;;
    esac
}

main "$@"
