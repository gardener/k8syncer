# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT                 := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
EFFECTIVE_VERSION         := $(shell $(REPO_ROOT)/hack/get-version.sh)
K8SYNCER_IMAGE_REPOSITORY := $(shell $(REPO_ROOT)/hack/get-registry.sh --image)/k8syncer
IMG ?= $(K8SYNCER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: revendor
revendor: ## Runs 'go mod vendor' and 'go mod tidy'.
	@$(REPO_ROOT)/hack/revendor.sh

.PHONY: format
format: goimports ## Formats the imports.
	@echo "> Formatting imports ..."
	@FORMATTER=$(FORMATTER) $(REPO_ROOT)/hack/format.sh

.PHONY: check
check: jq goimports golangci-lint ## Runs 'go vet', linting checks, and verify that 'make format' has been called.
	@echo "> Verifying documentation index ..."
	@$(REPO_ROOT)/hack/verify-docs-index.sh
	@echo "> Running 'go vet' ..."
	@go vet $(REPO_ROOT)/...
	@echo "> Running linter ..."
	@$(LINTER) run -c $(REPO_ROOT)/.golangci.yaml $(REPO_ROOT)/...
	@echo "> Checking for unformatted files ..."
	@FORMATTER=$(FORMATTER) $(REPO_ROOT)/hack/format.sh --verify

.PHONY: test
test: envtest ## Runs the tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: verify
verify: check ## Alias for check.

.PHONY: generate-docs
generate-docs: jq ## Generates the documentation index.
	@$(REPO_ROOT)/hack/generate-docs-index.sh

.PHONY: generate
generate: format revendor generate-docs ## Runs format, revendor and generate-docs.

##@ Build

.PHONY: install
install: ## Installs the binary.
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	@echo "Building image ${IMG}"
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
ADDITIONAL_TAG ?= ""
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	@echo "Building docker image ${IMG} ..."
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	@test -z "$(ADDITIONAL_TAG)" || docker buildx imagetools create ${IMG} --tag $(K8SYNCER_IMAGE_REPOSITORY):$(ADDITIONAL_TAG)
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: helm-chart
helm-chart: helm ## Upload the helm chart into the registry.
	@$(REPO_ROOT)/hack/build-chart.sh
	@$(REPO_ROOT)/hack/push-chart.sh

.PHONY: component
component: component-build component-push ## Builds the components and pushes them into the registry. To overwrite existing versions, set the env var OVERWRITE_COMPONENTS to anything except 'false' or the empty string.

.PHONY: component-build
component-build: ocm ## Build the components.
	OCM=$(OCM) $(REPO_ROOT)/hack/build-component.sh

.PHONY: component-push
component-push: ocm ## Upload the components into the registry. Must be called after 'make component-build'. To overwrite existing versions, set the env var OVERWRITE_COMPONENTS to anything except 'false' or the empty string.
	OCM=$(OCM) $(REPO_ROOT)/hack/push-component.sh

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(REPO_ROOT)/bin

## Tool Binaries
KUBECTL ?= kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
FORMATTER ?= $(LOCALBIN)/goimports
LINTER ?= $(LOCALBIN)/golangci-lint
OCM ?= $(LOCALBIN)/ocm
HELM ?= $(LOCALBIN)/helm
JQ ?= $(LOCALBIN)/jq

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.12.0
FORMATTER_VERSION ?= v0.16.0
LINTER_VERSION ?= 1.55.2
OCM_VERSION ?= 0.4.3
HELM_VERSION ?= v3.13.2
JQ_VERSION ?= 1.6

.PHONY: localbin
localbin: ## Creates the local bin folder, if it doesn't exist. Not meant to be called manually, used as requirement for the other tool commands.
	@test -d $(LOCALBIN) || mkdir -p $(LOCALBIN)

.PHONY: goimports
goimports: localbin ## Download goimports locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(FORMATTER) && test -s $(LOCALBIN)/goimports_version && cat $(LOCALBIN)/goimports_version | grep -q $(FORMATTER_VERSION) || \
	GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@$(FORMATTER_VERSION) && \
	echo $(FORMATTER_VERSION) > $(LOCALBIN)/goimports_version

.PHONY: golangci-lint
golangci-lint: localbin ## Download golangci-lint locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(LINTER) && $(LINTER) --version | grep -q $(LINTER_VERSION) || \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) v$(LINTER_VERSION)

.PHONY: envtest
envtest: localbin ## Download envtest-setup locally if necessary.
	@test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: ocm
ocm: localbin ## Install OCM CLI if necessary.
	@test -s $(OCM) && $(OCM) --version | grep -q $(OCM_VERSION) || \
	curl -sSfL https://ocm.software/install.sh | OCM_VERSION=$(OCM_VERSION) bash -s $(LOCALBIN)

.PHONY: helm
helm: localbin ## Download helm locally if necessary.
	@test -s $(HELM) && $(HELM) version --short | grep -q $(HELM_VERSION) || \
	HELM=$(HELM) LOCALBIN=$(LOCALBIN) $(REPO_ROOT)/hack/install-helm.sh $(HELM_VERSION)

.PHONY: jq
jq: localbin ## Download jq locally if necessary.
	@test -s $(JQ) && $(JQ) --version | grep -q $(JQ_VERSION) || \
	JQ=$(JQ) LOCALBIN=$(LOCALBIN) $(REPO_ROOT)/hack/install-jq.sh $(JQ_VERSION)
