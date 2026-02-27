#!/bin/bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2023-2026 Posit Software, PBC
#
# kind-cloud-agnostic.sh - Set up a kind cluster for cloud-agnostic development
#
# This script creates a kind cluster configured for testing the team-operator
# in cloud-agnostic mode, using:
# - Standard StorageClass (local-path-provisioner, built into kind)
# - K8s Secrets (not AWS Secrets Manager or Azure Key Vault)
# - No IAM annotations (field is optional)
# - Gateway API + Traefik (not Ingress)
#
# Usage:
#   ./hack/kind-cloud-agnostic.sh          # Create cluster and infrastructure
#   ./hack/kind-cloud-agnostic.sh --delete # Delete the cluster

set -euo pipefail

CLUSTER_NAME="team-operator-cloud-agnostic"
MODE="${1:-create}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

log_step() {
    echo -e "${BLUE}==>${NC} $1"
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
        log_error "Install them with: brew install kind kubectl helm"
        exit 1
    fi

    log_info "Prerequisites check passed"
}

# Delete the cluster
delete_cluster() {
    log_step "Deleting kind cluster '${CLUSTER_NAME}'..."

    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        kind delete cluster --name "${CLUSTER_NAME}"
        log_info "Cluster deleted"
    else
        log_warn "Cluster '${CLUSTER_NAME}' does not exist"
    fi

    exit 0
}

# Create kind cluster
create_cluster() {
    log_step "Creating kind cluster '${CLUSTER_NAME}'..."

    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_warn "Cluster '${CLUSTER_NAME}' already exists, reusing it"
        kubectl config use-context "kind-${CLUSTER_NAME}"
        return 0
    fi

    # Check that host ports 80 and 443 are available
    for port in 80 443; do
        local port_in_use=false
        if command -v lsof &>/dev/null; then
            lsof -i "TCP:${port}" -sTCP:LISTEN &>/dev/null && port_in_use=true
        else
            ss -tlnp | grep -qE ":${port}([^0-9]|$)" && port_in_use=true
        fi
        if [[ "${port_in_use}" == "true" ]]; then
            log_error "Port ${port} is already in use. Stop the process using it before creating the kind cluster."
            exit 1
        fi
    done

    # Create cluster with port mappings for Traefik NodePort services
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 80
    protocol: TCP
  - containerPort: 30443
    hostPort: 443
    protocol: TCP
    # Reserved for future HTTPS use; no TLS certificate is configured yet
EOF

    kubectl config use-context "kind-${CLUSTER_NAME}"
    log_info "Cluster created"
}

# Install Gateway API CRDs
install_gateway_api() {
    log_step "Installing Gateway API CRDs..."

    local gateway_version="v1.2.1"
    kubectl apply -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/${gateway_version}/standard-install.yaml"

    # Wait for CRDs to be established
    kubectl wait --for condition=established --timeout=60s \
        crd/gatewayclasses.gateway.networking.k8s.io \
        crd/gateways.gateway.networking.k8s.io \
        crd/httproutes.gateway.networking.k8s.io

    log_info "Gateway API CRDs installed"
}

# Install Traefik with Gateway API provider
install_traefik() {
    log_step "Installing Traefik with Gateway API provider..."

    # Add Traefik Helm repo
    local helm_output
    helm_output=$(helm repo add traefik https://traefik.github.io/charts 2>&1) || {
        if echo "${helm_output}" | grep -q "already exists"; then
            true  # repo already added, not an error
        else
            log_error "helm repo add failed: ${helm_output}"
            exit 1
        fi
    }
    [[ -n "${helm_output}" ]] && echo "${helm_output}"
    helm repo update traefik

    # Create traefik namespace
    kubectl create namespace traefik --dry-run=client -o yaml | kubectl apply -f -

    # Install Traefik with Gateway API enabled
    helm upgrade --install traefik traefik/traefik \
        --namespace traefik \
        --version "33.2.1" \
        --set providers.kubernetesGateway.enabled=true \
        --set gateway.enabled=false \
        --set ports.web.nodePort=30080 \
        --set ports.websecure.nodePort=30443 \
        --wait \
        --timeout 120s

    log_info "Traefik installed"
}

# Create Gateway resource
create_gateway() {
    log_step "Creating Gateway resource..."

    # Create self-signed TLS certificate for testing
    local tls_tmp
    tls_tmp=$(mktemp -d)
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "${tls_tmp}/tls.key" -out "${tls_tmp}/tls.crt" \
        -subj "/CN=*.dev.localhost" 2>/dev/null \
        || { log_error "Failed to generate self-signed certificate (is openssl installed?)"; rm -rf "${tls_tmp}"; exit 1; }
    kubectl create secret tls tls-cert \
        --namespace traefik \
        --cert="${tls_tmp}/tls.crt" --key="${tls_tmp}/tls.key" \
        --dry-run=client -o yaml | kubectl apply -f -
    rm -rf "${tls_tmp}"

    kubectl apply -f - <<EOF
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: traefik
spec:
  controllerName: traefik.io/gateway-controller

---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: posit-team
  namespace: traefik
spec:
  gatewayClassName: traefik
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
EOF

    # Wait for Gateway to be ready
    kubectl wait --for=condition=Programmed --timeout=60s gateway/posit-team -n traefik

    log_info "Gateway created"
}

# Create test namespace and secrets
create_test_resources() {
    log_step "Creating test namespace and secrets..."

    # Create namespace
    kubectl create namespace posit-team --dry-run=client -o yaml | kubectl apply -f -

    # Apply test secrets
    kubectl apply -f "$(dirname "$0")/kind-cloud-agnostic-secrets.yaml"

    log_info "Test resources created"
}

# Print usage instructions
print_usage() {
    log_info ""
    log_info "=========================================="
    log_info "kind cluster '${CLUSTER_NAME}' is ready!"
    log_info "=========================================="
    log_info ""
    log_info "Next steps:"
    log_info "  1. Deploy the operator:"
    log_info "     just helm-install"
    log_info ""
    log_info "  2. Apply the sample Site CR:"
    log_info "     kubectl apply -f hack/kind-cloud-agnostic-site.yaml"
    log_info ""
    log_info "  3. Check Site status:"
    log_info "     kubectl get site dev -n posit-team"
    log_info ""
    log_info "  4. View operator logs:"
    log_info "     kubectl logs -n posit-team-system -l app.kubernetes.io/name=team-operator -f"
    log_info ""
    log_info "  5. Delete cluster when done:"
    log_info "     just kind-cloud-agnostic-delete"
    log_info ""
}

# Main
main() {
    case "${MODE}" in
        create|--create|"")
            log_info "Setting up kind cluster for cloud-agnostic development..."
            check_prerequisites
            create_cluster
            install_gateway_api
            install_traefik
            create_gateway
            create_test_resources
            print_usage
            ;;
        --delete|delete)
            check_prerequisites
            delete_cluster
            ;;
        *)
            log_error "Unknown option: ${MODE}"
            echo "Usage: $0 [create|--create|--delete|delete]"
            exit 1
            ;;
    esac
}

main
