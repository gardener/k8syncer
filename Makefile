# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

REPO_ROOT                 := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
VERSION                   := $(shell cat $(REPO_ROOT)/VERSION)
EFFECTIVE_VERSION         := $(VERSION)-$(shell git rev-parse HEAD)
K8SYNCER_IMAGE_REPOSITORY := eu.gcr.io/gardener-project/k8syncer
DOCKER_BUILDER_NAME       := "multiarch"

.PHONY: install-requirements
install-requirements:
	@go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	@$(REPO_ROOT)/hack/install-requirements.sh

.PHONY: revendor
revendor:
	@$(REPO_ROOT)/hack/revendor.sh

.PHONY: format
format:
	@$(REPO_ROOT)/hack/format.sh $(REPO_ROOT)/pkg $(REPO_ROOT)/cmd

.PHONY: check
check: format
	@$(REPO_ROOT)/hack/verify-docs-index.sh
	@$(REPO_ROOT)/hack/check.sh --golangci-lint-config=./.golangci.yaml $(REPO_ROOT)/cmd/... $(REPO_ROOT)/pkg/...

.PHONY: setup-testenv
setup-testenv:
	@$(REPO_ROOT)/hack/setup-testenv.sh

.PHONY: test
test: setup-testenv
	@$(REPO_ROOT)/hack/test.sh

.PHONY: verify
verify: check

.PHONY: generate-docs
generate-docs:
	@$(REPO_ROOT)/hack/generate-docs-index.sh

.PHONY: generate
generate: format revendor generate-docs


#################################################################
# Rules related to binary build, docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh

.PHONY: docker-images
docker-images:
	@$(REPO_ROOT)/hack/prepare-docker-builder.sh $(DOCKER_BUILDER_NAME)
	@echo "Building docker images for version $(EFFECTIVE_VERSION)"
	@docker buildx build --builder $(DOCKER_BUILDER_NAME) --load --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform linux/amd64 -t $(K8SYNCER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -f Dockerfile --target k8syncer .

.PHONY: docker-push
docker-push:
	@echo "Pushing docker images for version $(EFFECTIVE_VERSION) to registry $(REGISTRY)"
	@if ! docker images $(K8SYNCER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(K8SYNCER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@docker push $(K8SYNCER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)

.PHONY: docker-all
docker-all: docker-images docker-push
