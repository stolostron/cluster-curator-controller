-include Makefile.prow

SHELL := /bin/bash

export BINDATA_TEMP_DIR := $(shell mktemp -d)

export GIT_COMMIT      = $(shell git rev-parse --short HEAD)
export GIT_REMOTE_URL  = $(shell git config --get remote.origin.url)

export ARCH       ?= $(shell uname -m)
export ARCH_TYPE   = $(if $(patsubst x86_64,,$(ARCH)),$(ARCH),amd64)
export BUILD_DATE  = $(shell date +%m/%d@%H:%M:%S)
export VCS_REF     = $(if $(shell git status --porcelain),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT))

export CGO_ENABLED  = 1
export GO111MODULE := on
export GOPACKAGES   = $(shell go list ./... | grep -v /vendor | grep -v /internal | grep -v /build | grep -v /test | grep -v /controllers)

export PROJECT_DIR            = $(shell 'pwd')
export BUILD_DIR              = $(PROJECT_DIR)/build
export COMPONENT_SCRIPTS_PATH = $(BUILD_DIR)

export COMPONENT_NAME ?= $(shell cat ./COMPONENT_NAME 2> /dev/null)
export COMPONENT_VERSION ?= $(shell cat ./COMPONENT_VERSION 2> /dev/null)

## WARNING: OPERATOR-SDK - IMAGE_DESCRIPTION & DOCKER_BUILD_OPTS MUST NOT CONTAIN ANY SPACES
export IMAGE_DESCRIPTION ?= cluster-curator
export DOCKER_FILE        = $(BUILD_DIR)/Dockerfile
export DOCKER_REGISTRY   ?= quay.io
export DOCKER_NAMESPACE  ?= stolostron
export DOCKER_IMAGE      ?= $(COMPONENT_NAME)
export DOCKER_IMAGE_COVERAGE_POSTFIX ?= -coverage
export DOCKER_IMAGE_COVERAGE      ?= $(DOCKER_IMAGE)$(DOCKER_IMAGE_COVERAGE_POSTFIX)
export DOCKER_BUILD_TAG  ?= latest
export DOCKER_TAG        ?= $(shell whoami)

#CRD_OPTIONS ?= "crd"
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Use setup-envtest to download envtest binaries from the new location (controller-tools releases).
# See: https://github.com/kubernetes-sigs/kubebuilder/discussions/4082
ENVTEST_K8S_VERSION ?= 1.30.0
ENVTEST_VERSION ?= v0.20.4


BEFORE_SCRIPT := $(shell build/before-make.sh)

export DOCKER_BUILD_OPTS  = --build-arg VCS_REF=$(VCS_REF) \
	--build-arg VCS_URL=$(GIT_REMOTE_URL) \
	--build-arg IMAGE_NAME=$(DOCKER_IMAGE) \
	--build-arg IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION) \
	--build-arg ARCH_TYPE=$(ARCH_TYPE) \
	--build-arg REMOTE_SOURCE=. \
	--build-arg REMOTE_SOURCE_DIR=/remote-source \
	--build-arg BUILD_HARNESS_EXTENSIONS_PROJECT=${BUILD_HARNESS_EXTENSIONS_PROJECT} \
	--build-arg GITHUB_TOKEN=$(GITHUB_TOKEN)


# Only use git commands if it exists
ifdef GIT
GIT_COMMIT      = $(shell git rev-parse --short HEAD)
GIT_REMOTE_URL  = $(shell git config --get remote.origin.url)
VCS_REF     = $(if $(shell git status --porcelain),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT))
endif


.PHONY: deps
## Download all project dependencies
deps: init component/init

.PHONY: check
## Runs a set of required checks
check: copyright-check

.PHONY: controller-gen
# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: manifests
## Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=./... rbac:roleName=manager-role output:crd:artifacts:config=deploy/crd

.PHONY: generate
# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: copyright-check
copyright-check:
	./build/copyright-check.sh $(TRAVIS_BRANCH)

.PHONY: clean
## Clean build-harness and remove Go generated build and test files
clean::
	@rm -rf $(BUILD_DIR)/_output

.PHONY: lint
## Runs linter against go files
lint:
	@echo "Running linting tool ..."
	@GOGC=25 golangci-lint run --timeout 5m

.PHONY: unit-tests
## Runs go unit tests
unit-tests: ensure-kubebuilder-tools
	@export KUBEBUILDER_ASSETS=$$(setup-envtest use -i -p path $(ENVTEST_K8S_VERSION)); \
	build/run-unit-tests.sh


.PHONY: push-curator
push-curator: build-curator
	docker push ${REPO_URL}/cluster-curator-controller:${VERSION}
	docker tag ${REPO_URL}/cluster-curator-controller:${VERSION} ${REPO_URL}/cluster-curator-controller:latest
	docker push ${REPO_URL}/cluster-curator-controller:latest
	#./deploy/controller/process.sh

.PHONY: compile-curator
compile-curator:
	go mod tidy
	go mod vendor
	GOFLAGS="" go build -o build/_output/curator ./cmd/curator/curator.go
	GOFLAGS="" go build -o build/_output/manager ./cmd/manager/main.go

.PHONY: compile-curator-konflux
compile-curator-konflux:
	GOFLAGS="" go build -o build/_output/curator ./cmd/curator/curator.go
	GOFLAGS="" go build -o build/_output/manager ./cmd/manager/main.go

.PHONY: build-curator
build-curator:
	docker build -f Dockerfile.prow . -t ${REPO_URL}/cluster-curator-controller:${VERSION}

.PHONY: scale-up-test
scale-up-test:
	go test -v -timeout 500s ./cmd/controller/controller_test.go -run TestCreateControllerScale

.PHONY: scale-down-test
scale-down-test:
	go test -v -timeout 500s ./cmd/controller/controller_test.go -run TestDeleteManagedClusters


# Install setup-envtest (used to download envtest binaries from controller-tools GitHub releases).
.PHONY: ensure-kubebuilder-tools
ensure-kubebuilder-tools:
	@which setup-envtest >/dev/null 2>&1 || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)
