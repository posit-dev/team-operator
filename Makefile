# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1

SED ?= sed
ifeq ($(shell uname), Darwin)
SED = gsed
endif

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# posit.co/team-operator-bundle:$VERSION and posit.co/team-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= posit.co/team-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.x

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate-all
generate-all: generate generate-client generate-openapi

.PHONY: verify-all
verify-all: verify-apply verify-list verify-inform verify-client

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."


.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate-all fmt vet go-test cov ## Run generation and test commands.

.PHONY: go-test
go-test: envtest ## Run only the go tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use "$(ENVTEST_K8S_VERSION)" --bin-dir "$(LOCALBIN)" -p path)" \
		sh -c 'go test -buildvcs=false -v $$(go list -buildvcs=false -f '\''{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}'\'' ./...) -race -covermode=atomic -coverprofile coverage.out'

.PHONY: cov
cov: ## Show the coverage report at the function level.
	$(SED) -i '/team-operator\/client-go/d' coverage.out
	go tool cover -func coverage.out

##@ Integration Testing

KIND_CLUSTER_NAME ?= team-operator-test

.PHONY: kind-create
kind-create: ## Create a kind cluster for integration testing.
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "Kind cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		echo "Creating kind cluster '$(KIND_CLUSTER_NAME)'..."; \
		kind create cluster --name $(KIND_CLUSTER_NAME) --wait 60s; \
	fi

.PHONY: kind-delete
kind-delete: ## Delete the kind cluster.
	kind delete cluster --name $(KIND_CLUSTER_NAME) || true

.PHONY: kind-load-image
kind-load-image: docker-build ## Load the operator image into kind cluster.
	kind load docker-image $(IMG) --name $(KIND_CLUSTER_NAME)

.PHONY: kind-setup
kind-setup: kind-create docker-build helm-generate ## Set up kind cluster and deploy operator (run once, or after code changes to reload).
	@echo "Setting up kind cluster '$(KIND_CLUSTER_NAME)'..."
	./hack/test-kind.sh $(KIND_CLUSTER_NAME) setup

.PHONY: kind-test
kind-test: ## Run integration tests against an existing kind cluster (requires kind-setup first).
	./hack/test-kind.sh $(KIND_CLUSTER_NAME) test

.PHONY: kind-teardown
kind-teardown: ## Tear down the kind cluster and remove all test resources.
	./hack/test-kind.sh $(KIND_CLUSTER_NAME) teardown
	kind delete cluster --name $(KIND_CLUSTER_NAME) || true

.PHONY: test-kind
test-kind: kind-create docker-build helm-generate ## Build operator image and run integration tests on a kind cluster.
	@echo "Running integration tests on kind cluster '$(KIND_CLUSTER_NAME)'..."
	./hack/test-kind.sh $(KIND_CLUSTER_NAME)

.PHONY: test-kind-full
test-kind-full: kind-delete kind-create test-kind ## Run full integration tests (clean cluster).
	@echo "Full integration test completed."

.PHONY: test-integration
test-integration: go-test test-kind ## Run all tests (unit + integration).
	@echo "All tests completed."

##@ Build

.PHONY: build
build: manifests generate-all fmt vet ## Build manager binary.
	go build -o bin/team-operator ./cmd/team-operator/main.go

.PHONY: docker-build
docker-build: build ## Build the operator Docker image.
	docker build -t $(IMG) .

.PHONY: distclean
distclean:
	git clean -xd bin/ || true
	go clean -cache -modcache -testcache -fuzzcache

.PHONY: run
run: manifests generate-all fmt vet ## Run a controller from your host.
	go run ./cmd/team-operator/main.go

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: test-kustomize
test-kustomize: manifests kustomize
	$(KUSTOMIZE) build config/default

.PHONY: build-installer
build-installer: manifests kustomize ## Generate dist/install.yaml from kustomize
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Helm

CHART_DIR ?= dist/chart
CHART_NAME ?= team-operator

.PHONY: helm-generate
helm-generate: build-installer kubebuilder ## Regenerate Helm chart from kustomize
	# Backup Chart.yaml and README.md from git (they will be overwritten by the plugin)
	@git show HEAD:dist/chart/Chart.yaml > /tmp/Chart.yaml.bak 2>/dev/null || true
	@git show HEAD:dist/chart/README.md > /tmp/README.md.bak 2>/dev/null || true
	$(KUBEBUILDER) edit --plugins=helm/v2-alpha
	# Restore backed up files
	@if [ -f /tmp/Chart.yaml.bak ]; then mv /tmp/Chart.yaml.bak dist/chart/Chart.yaml; fi
	@if [ -f /tmp/README.md.bak ]; then mv /tmp/README.md.bak dist/chart/README.md; fi
	# Remove kubebuilder-generated test workflow - we use our own CI workflows
	rm -f .github/workflows/test-chart.yml
	# Remove build artifact that should not be committed
	rm -f dist/install.yaml
	# Apply customizations that v2-alpha plugin overwrites
	SED=$(SED) ./hack/helm-post-generate.sh

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	helm lint $(CHART_DIR)

.PHONY: helm-template
helm-template: ## Render Helm templates locally
	helm template $(CHART_NAME) $(CHART_DIR)

.PHONY: helm-test-certmanager
helm-test-certmanager: ## Verify cert-manager volumes render correctly
	@echo "Testing cert-manager volume mounts..."
	@helm template test $(CHART_DIR) --set certManager.enable=true | \
		grep -q "mountPath: /tmp/k8s-webhook-server/serving-certs" || \
		(echo "ERROR: cert-manager volumeMounts not rendered!" && exit 1)
	@helm template test $(CHART_DIR) --set certManager.enable=true | \
		grep -q "webhook-server-cert" || \
		(echo "ERROR: cert-manager volumes not rendered!" && exit 1)
	@echo "cert-manager volumes OK"

.PHONY: helm-install
helm-install: ## Install operator via Helm
	helm upgrade --install $(CHART_NAME) $(CHART_DIR) \
		--namespace posit-team-system --create-namespace

.PHONY: helm-package
helm-package: ## Package the Helm chart as .tar.gz
	helm package $(CHART_DIR) -d dist/

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBEBUILDER ?= $(LOCALBIN)/kubebuilder
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
APPLYCONFIGURATION_GEN ?= $(LOCALBIN)/applyconfiguration-gen
LISTER_GEN ?= $(LOCALBIN)/lister-gen
INFORMER_GEN ?= $(LOCALBIN)/informer-gen
CLIENT_GEN ?= $(LOCALBIN)/client-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
KUBE_CODEGEN ?= $(LOCALBIN)/kube_codegen.sh

## Tool Versions
KUBEBUILDER_VERSION ?= v4.12.0
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.17.0
KUBE_CODEGEN_VERSION ?= v0.30.1

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { \
		if [ -n "$$GITHUB_TOKEN" ]; then \
			curl -Ss -H "Authorization: token $$GITHUB_TOKEN" $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); \
		else \
			curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); \
		fi; \
	}

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: kubebuilder
kubebuilder: $(KUBEBUILDER) ## Download kubebuilder locally if necessary.
$(KUBEBUILDER): $(LOCALBIN)
	@if ! test -s $(LOCALBIN)/kubebuilder || ! $(LOCALBIN)/kubebuilder version | grep -q $(KUBEBUILDER_VERSION); then \
		OS=$$(go env GOOS) && ARCH=$$(go env GOARCH) && \
		curl -sSL -o $(LOCALBIN)/kubebuilder "https://github.com/kubernetes-sigs/kubebuilder/releases/download/$(KUBEBUILDER_VERSION)/kubebuilder_$${OS}_$${ARCH}" && \
		chmod +x $(LOCALBIN)/kubebuilder; \
	fi

.PHONY: kube-codgen
kube-codegen: $(LOCALBIN)
	test -s $(LOCALBIN)/kube_codegen.sh || \
	if [ -n "$$GITHUB_TOKEN" ]; then \
		curl -o $(LOCALBIN)/kube_codegen.sh -sSL -H "Authorization: token $$GITHUB_TOKEN" https://raw.githubusercontent.com/kubernetes/code-generator/$(KUBE_CODEGEN_VERSION)/kube_codegen.sh; \
	else \
		curl -o $(LOCALBIN)/kube_codegen.sh -sSL https://raw.githubusercontent.com/kubernetes/code-generator/$(KUBE_CODEGEN_VERSION)/kube_codegen.sh; \
	fi

.PHONY: clean-kube-codgen
clean-kube-codegen: $(LOCALBIN)
	rm -f $(LOCALBIN)/kube_codegen.sh

.PHONY: generate-client
generate-client: kube-codegen
	source $(KUBE_CODEGEN) && \
	echo "Generating client files..." && \
	kube::codegen::gen_client ./api \
		--output-dir client-go \
		--output-pkg github.com/posit-dev/team-operator/client-go \
		--with-applyconfig --with-watch \
		--boilerplate hack/boilerplate.go.txt

# NOTE: this will fail if new openapi failures show up
.PHONY: generate-openapi
generate-openapi: kube-codegen
	source $(KUBE_CODEGEN) && \
	echo "Generating openapi files... on failure, see openapi/openapi-failures.txt" && \
  	kube::codegen::gen_openapi ./api \
		--output-dir openapi \
		--output-pkg github.com/posit-dev/team-operator/openapi \
		--boilerplate hack/boilerplate.go.txt \
		--report-filename openapi/openapi-failures.txt

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle $(BUNDLE_GEN_FLAGS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	if [ -n "$$GITHUB_TOKEN" ]; then \
		curl -sSLo $(OPM) -H "Authorization: token $$GITHUB_TOKEN" https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	else \
		curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	fi ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

##@ Helm Deployment

## Helm binary to use for deploying the chart
HELM ?= helm
## Namespace to deploy the Helm release
HELM_NAMESPACE ?= posit-team-system
## Name of the Helm release
HELM_RELEASE ?= team-operator
## Path to the Helm chart directory
HELM_CHART_DIR ?= dist/chart
## Additional arguments to pass to helm commands
HELM_EXTRA_ARGS ?=

.PHONY: install-helm
install-helm: ## Install the latest version of Helm.
	@command -v $(HELM) >/dev/null 2>&1 || { \
		echo "Installing Helm..." && \
		curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | bash; \
	}

.PHONY: helm-deploy
helm-deploy: install-helm ## Deploy manager to the K8s cluster via Helm. Specify an image with IMG.
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART_DIR) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set manager.image.repository=$${IMG%:*} \
		--set manager.image.tag=$${IMG##*:} \
		--wait \
		--timeout 5m \
		$(HELM_EXTRA_ARGS)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall the Helm release from the K8s cluster.
	$(HELM) uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-status
helm-status: ## Show Helm release status.
	$(HELM) status $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-history
helm-history: ## Show Helm release history.
	$(HELM) history $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: helm-rollback
helm-rollback: ## Rollback to previous Helm release.
	$(HELM) rollback $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)
