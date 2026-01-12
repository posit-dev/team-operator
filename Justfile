IMAGE_REGISTRY := "ghcr.io/posit-dev"
IMAGE_NAME := "team-operator"
DEV_IMAGE_NAME := "adhoc-team-operator"
VERSION := `git describe --always --dirty --tags`
DEV_VERSION := "$(git describe --always --dirty --tags)-${BRANCH_NAME}"
postgres_pass := "dev-postgres-password"
BUILDX_PATH := ""

# this needs to be accessible from wherever the operator is running...
db_host := "team-operator-db-postgresql.default.svc.cluster.local"
db_port := "5432"

db_url_secret := "team-operator-main-db-url"

default: build test

# Install dependencies
deps:
  go get -t ./...

# Update dependencies
deps-up:
  go get -t -u ./...

# Run team-operator directly from source
run:
  go run cmd/team-operator/main.go

# Run team-operator via the Makefile target
mrun:
  make run

# Build ./bin/team-operator
build:
  go build \
    -ldflags="-X 'github.com/posit-dev/team-operator/internal.VersionString={{ VERSION }}'" \
    -a \
    -o ./bin/team-operator \
    cmd/team-operator/main.go

# Build ./bin/team-operator via the Makefile target
mbuild:
  make build

create-db-url-secret:
  #!/bin/bash
  set -xe
  kubectl delete secret --ignore-not-found '{{ db_url_secret }}'
  # TODO: should we delete the secret? Make idempotent? URL encode for weird characters?...
  kubectl create secret generic '{{ db_url_secret }}' \
    --from-literal \
    url='postgres://postgres:{{ postgres_pass }}@{{ db_host }}:{{ db_port }}/postgres?sslmode=require'

k3d-up:
  k3d cluster create dev

license:
  kubectl create secret generic license \
    -n posit-team \
    --from-file=lic/pw.lic \
    --from-file=lic/pc.lic \
    --from-file=lic/ppm.lic

k3d-down:
  k3d cluster stop dev

# Install CRDs via the Makefile 'install' target
crds:
  make install

mgenerate:
  make generate-all && make manifests

crd-test:
  #!/bin/bash
  set -xe
  kubectl apply -f config/samples/test_site.yaml
  kubectl apply -f config/samples/test-secret.yaml

crd-test-diff:
  kubectl diff -f config/samples/test_site.yaml

crd-test-delete:
  kubectl delete -f config/samples/test_site.yaml

# Deploy full kustomization via the Makefile target
mdeploy:
  make deploy IMG="{{ IMAGE_REGISTRY }}/{{ IMAGE_NAME }}:{{ VERSION }}"

# Un-deploy via the Makefile target
mundeploy:
  make undeploy

db-up:
  docker run --rm -d \
    --name team-operator-db \
    -p {{ db_port }}:5432 \
    -e POSTGRES_PASSWORD={{ postgres_pass }} \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_DB=postgres \
    postgres:16

db-down:
  docker stop team-operator-db

psql:
  #!/bin/bash
  set -xe
  echo 'Password is: {{ postgres_pass }}'
  echo ''
  psql -h '{{ db_host }}' -p '{{ db_port }}' -d postgres -U postgres -W

db-k8s-create:
  #!/bin/bash
  # NOTE: accessing this can be tricky from the operator when running locally
  set -xe
  helm repo add bitnami https://charts.bitnami.com/bitnami
  helm upgrade --install team-operator-db bitnami/postgresql \
    --version 11.6.16 \
    --set auth.database="postgres" \
    --set auth.username="postgres" \
    --set global.postgresql.auth.postgresPassword="{{ postgres_pass }}" \
    --set primary.persistence.enabled=false

db-k8s-delete:
  @helm uninstall team-operator-db

db-k8s-forward:
  @kubectl port-forward svc/team-operator-db-postgresql 5432:5432

docker-build:
  #!/bin/bash
  set -xe
  # variable placeholers
  BUILDX_ARGS=""
  BUILDER=""
  # set buildx args
  if [[ "{{BUILDX_PATH}}" != "" ]]; then
    BUILDER="--builder={{ BUILDX_PATH }}"
    BUILDX_ARGS="--cache-from=type=local,src=/tmp/.buildx-cache --cache-to=type=local,dest=/tmp/.buildx-cache"
  fi

  # Get Go version from go.mod. The 4 curly brackets on the left side look odd,
  # but this is intentional and the escaping that works for the combination of
  # just and bash parsing. The echo statement proves that it is extracting the
  # version correctly.
  GO_VERSION=$(go list -m -f "{{{{.GoVersion}}")
  if [[ -z "$GO_VERSION" ]]; then
    echo "Error: Could not extract Go version from go.mod"
    exit 1
  fi
  echo Getting Go version from go.mod: ${GO_VERSION}

  docker buildx $BUILDER build --load $BUILDX_ARGS \
    --platform=linux/amd64 \
    --build-arg "VERSION={{ VERSION }}" \
    --build-arg "GO_VERSION=${GO_VERSION}" \
    -t {{ IMAGE_REGISTRY }}/$(just echo-image) \
    .

docker-push:
  docker push {{ IMAGE_REGISTRY }}/$(just echo-image)

echo-image:
  #!/bin/bash
  if [[ -z "${BRANCH_NAME}" ]]; then
    echo "BRANCH_NAME is not set... trying with CLI" >&2
    export BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD)
    echo "BRANCH_NAME=${BRANCH_NAME}" >&2
  fi
  if [[ "${BRANCH_NAME}" == "main" ]]; then
    echo {{ IMAGE_NAME }}:{{ VERSION }}
  else
    echo {{ DEV_IMAGE_NAME }}:{{ DEV_VERSION }}
  fi

# Run go tests without envtest
test:
  go test -v ./... -coverprofile coverage.out

# Run tests with envtest via the Makefile target
mtest:
  make test

# Generate Helm chart from kustomize
helm-generate:
  make helm-generate

# Install operator via Helm
helm-install:
  make helm-install

# Uninstall operator via Helm
helm-uninstall:
  make helm-uninstall

# Lint Helm chart
helm-lint:
  make helm-lint

# Render Helm templates locally
helm-template:
  make helm-template

# Package Helm chart as .tar.gz
helm-package:
  make helm-package
