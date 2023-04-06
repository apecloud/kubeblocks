#
# Copyright 2022 The KubeBlocks Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

################################################################################
# Variables                                                                    #
################################################################################
APP_NAME = kubeblocks
VERSION ?= 0.5.0-alpha.0
GITHUB_PROXY ?=
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=0 --tag)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.25.0
ENABLE_WEBHOOKS ?= false
SKIP_GO_GEN ?= true
CHART_PATH = deploy/helm
WEBHOOK_CERT_DIR ?= /tmp/k8s-webhook-server/serving-certs

# Go setup
export GO111MODULE = auto
# export GOPROXY = https://proxy.golang.org
export GOPROXY = https://goproxy.cn
export GOSUMDB = sum.golang.org
export GONOPROXY = github.com/apecloud
export GONOSUMDB = github.com/apecloud
export GOPRIVATE = github.com/apecloud
GO ?= go
GOFMT ?= gofmt
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO) env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif
LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`"
# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

# docker build setup
# BUILDX_PLATFORMS ?= $(subst -,/,$(ARCH))
BUILDX_PLATFORMS ?= linux/amd64,linux/arm64
BUILDX_OUTPUT_TYPE ?= docker

TAG_LATEST ?= false
BUILDX_ENABLED ?= false
ifneq ($(BUILDX_ENABLED), false)
	ifeq ($(shell docker buildx inspect 2>/dev/null | awk '/Status/ { print $$2 }'), running)
		BUILDX_ENABLED ?= true
	else
		BUILDX_ENABLED ?= false
	endif
endif

define BUILDX_ERROR
buildx not enabled, refusing to run this recipe
endef

ifeq ($(TAG_LATEST), true)
	IMAGE_TAGS ?= $(IMG):$(VERSION) $(IMG):latest
else
	IMAGE_TAGS ?= $(IMG):$(VERSION)
endif

DOCKER_BUILD_ARGS =
DOCKER_NO_BUILD_CACHE ?= false

ifeq ($(DOCKER_NO_BUILD_CACHE), true)
	DOCKER_BUILD_ARGS = $(DOCKER_BUILD_ARGS) --no-cache
endif


.DEFAULT_GOAL := help

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
# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: manager kbcli probe reloader loadbalancer ## Make all cmd binaries.

##@ Development

.PHONY: manifests
manifests: test-go-generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./cmd/manager/...;./apis/...;./controllers/...;./internal/..." output:crd:artifacts:config=config/crd/bases
	@cp config/crd/bases/* $(CHART_PATH)/crds
	@cp config/rbac/role.yaml $(CHART_PATH)/config/rbac/role.yaml
	$(CONTROLLER_GEN) rbac:roleName=loadbalancer-role  paths="./cmd/loadbalancer/..." output:dir=config/loadbalancer

.PHONY: preflight-manifests
preflight-manifests: generate ## Generate external Preflight API
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./externalapis/preflight/..." output:crd:artifacts:config=config/crd/preflight

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/...;./externalapis/..."

.PHONY: manager-go-generate
manager-go-generate: ## Run go generate against lifecycle manager code.
ifeq ($(SKIP_GO_GEN), false)
	$(GO) generate -x ./internal/configuration/proto
endif



.PHONY: test-go-generate
test-go-generate: ## Run go generate against test code.
	$(GO) generate -x ./internal/testutil/k8s/mocks/...
	$(GO) generate -x ./internal/configuration/container/mocks/...
	$(GO) generate -x ./internal/configuration/proto/mocks/...

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GOFMT) -l -w -s $$(git ls-files --exclude-standard | grep "\.go$$")

.PHONY: vet
vet: ## Run go vet against code.
	GOOS=linux $(GO) vet -mod=mod ./...

.PHONY: cue-fmt
cue-fmt: cuetool ## Run cue fmt against code.
	git ls-files --exclude-standard | grep "\.cue$$" | xargs $(CUE) fmt
	git ls-files --exclude-standard | grep "\.cue$$" | xargs $(CUE) fix

.PHONY: fast-lint
fast-lint: golangci staticcheck vet  # [INTERNAL] fast lint
	$(GOLANGCILINT) run ./...

.PHONY: lint
lint: test-go-generate generate ## Run golangci-lint against code.
	$(MAKE) fast-lint

.PHONY: staticcheck
staticcheck: staticchecktool ## Run staticcheck against code.
	$(STATICCHECK) ./...

.PHONY: build-checks
build-checks: generate fmt vet goimports fast-lint ## Run build checks.

.PHONY: mod-download
mod-download: ## Run go mod download against go modules.
	$(GO) mod download

.PHONY: mod-vendor
mod-vendor: module ## Run go mod vendor against go modules.
	$(GO) mod vendor

.PHONY: module
module: ## Run go mod tidy->verify against go modules.
	$(GO) mod tidy -compat=1.19
	$(GO) mod verify

TEST_PACKAGES ?= ./internal/... ./apis/... ./controllers/... ./cmd/...

CLUSTER_TYPES=minikube k3d
.PHONY: add-k8s-host
add-k8s-host:  ## add DNS to /etc/hosts when k8s cluster is minikube or k3d
ifneq (, $(findstring $(EXISTING_CLUSTER_TYPE), $(CLUSTER_TYPES)))
ifeq (, $(shell sed -n "/^127.0.0.1[[:space:]]*host.$(EXISTING_CLUSTER_TYPE).internal/p" /etc/hosts))
	sudo bash -c 'echo "127.0.0.1 host.$(EXISTING_CLUSTER_TYPE).internal" >> /etc/hosts'
endif
endif

.PHONY: test-current-ctx
test-current-ctx: manifests generate fmt vet add-k8s-host ## Run operator controller tests with current $KUBECONFIG context. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	USE_EXISTING_CLUSTER=true KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test  -p 1 -coverprofile cover.out $(TEST_PACKAGES)

.PHONY: test-fast
test-fast: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -short -coverprofile cover.out $(TEST_PACKAGES)

.PHONY: test
test: manifests generate test-go-generate fmt vet add-k8s-host test-fast ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.

.PHONY: test-integration
test-integration: manifests generate fmt vet envtest add-k8s-host ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test ./test/integration

.PHONY: test-delve
test-delve: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" dlv --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient test $(TEST_PACKAGES)

.PHONY: test-webhook-enabled
test-webhook-enabled: ## Run tests with webhooks enabled.
	$(MAKE) test ENABLE_WEBHOOKS=true

.PHONY: cover-report
cover-report: ## Generate cover.html from cover.out
	$(GO) tool cover -html=cover.out -o cover.html
ifeq ($(GOOS), darwin)
	open ./cover.html
else
	echo "open cover.html with a HTML viewer."
endif

.PHONY: goimports
goimports: goimportstool ## Run goimports against code.
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(git ls-files|grep "\.go$$")


##@ CLI
K3S_VERSION ?= v1.23.8+k3s1
K3D_VERSION ?= 5.4.4
K3S_IMG_TAG ?= $(subst +,-,$(K3S_VERSION))

CLI_LD_FLAGS ="-s -w \
	-X github.com/apecloud/kubeblocks/version.BuildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` \
	-X github.com/apecloud/kubeblocks/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/apecloud/kubeblocks/version.GitVersion=$(GIT_VERSION) \
	-X github.com/apecloud/kubeblocks/version.Version=$(VERSION) \
	-X github.com/apecloud/kubeblocks/version.K3sImageTag=$(K3S_IMG_TAG) \
	-X github.com/apecloud/kubeblocks/version.K3dVersion=$(K3D_VERSION) \
	-X github.com/apecloud/kubeblocks/version.DefaultKubeBlocksVersion=$(VERSION)"



bin/kbcli.%: ## Cross build bin/kbcli.$(OS).$(ARCH).
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) CGO_ENABLED=0 $(GO) build -ldflags=${CLI_LD_FLAGS} -o $@ cmd/cli/main.go

.PHONY: kbcli-fast
kbcli-fast: OS=$(shell $(GO) env GOOS)
kbcli-fast: ARCH=$(shell $(GO) env GOARCH)
kbcli-fast:
	$(MAKE) bin/kbcli.$(OS).$(ARCH)
	@mv bin/kbcli.$(OS).$(ARCH) bin/kbcli

.PHONY: kbcli
kbcli: test-go-generate build-checks kbcli-fast ## Build bin/kbcli.

.PHONY: clean-kbcli
clean-kbcli: ## Clean bin/kbcli*.
	rm -f bin/kbcli*

.PHONY: doc
kbcli-doc: generate ## generate CLI command reference manual.
	$(GO) run ./hack/docgen/cli/main.go ./docs/user_docs/cli



##@ Operator Controller Manager

.PHONY: manager
manager: cue-fmt generate manager-go-generate test-go-generate build-checks ## Build manager binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/manager ./cmd/manager/main.go

CERT_ROOT_CA ?= $(WEBHOOK_CERT_DIR)/rootCA.key
.PHONY: webhook-cert
webhook-cert: $(CERT_ROOT_CA) ## Create root CA certificates for admission webhooks testing.
$(CERT_ROOT_CA):
	mkdir -p $(WEBHOOK_CERT_DIR)
	cd $(WEBHOOK_CERT_DIR) && \
		step certificate create $(APP_NAME) rootCA.crt rootCA.key --profile root-ca --insecure --no-password && \
		step certificate create $(APP_NAME)-svc tls.crt tls.key --profile leaf \
			--ca rootCA.crt --ca-key rootCA.key \
			--san $(APP_NAME)-svc --san $(APP_NAME)-svc.$(APP_NAME) --san $(APP_NAME)-svc.$(APP_NAME).svc --not-after 43200h --insecure --no-password

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
ifeq ($(ENABLE_WEBHOOKS), true)
	$(MAKE) webhook-cert
endif
	$(GO) run ./cmd/manager/main.go --zap-devel=false --zap-encoder=console --zap-time-encoding=iso8601

# Run with Delve for development purposes against the configured Kubernetes cluster in ~/.kube/config
# Delve is a debugger for the Go programming language. More info: https://github.com/go-delve/delve
GO_PACKAGE=./cmd/manager/main.go
ARGUMENTS=
DEBUG_PORT=2345
run-delve: manifests generate fmt vet  ## Run Delve debugger.
	dlv --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient debug $(GO_PACKAGE) -- $(ARGUMENTS)


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	($(KUSTOMIZE) build config/crd | kubectl replace -f -) || ($(KUSTOMIZE) build config/crd | kubectl create -f -)
	$(KUSTOMIZE) build $(shell $(GO) env GOPATH)/pkg/mod/github.com/kubernetes-csi/external-snapshotter/client/v6@v6.0.1/config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -
	$(KUSTOMIZE) build $(shell $(GO) env GOPATH)/pkg/mod/github.com/kubernetes-csi/external-snapshotter/client/v6@v6.0.1/config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: dry-run
dry-run: manifests kustomize ## Dry-run deploy job.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	mkdir -p dry-run
	$(KUSTOMIZE) build config/default > dry-run/manifests.yaml

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Contributor

.PHONY: reviewable
reviewable: generate build-checks test check-license-header ## Run code checks to proceed with PR reviews.
	$(GO) mod tidy -compat=1.19

.PHONY: check-diff
check-diff: reviewable ## Run git code diff checker.
	git --no-pager diff
	git diff --quiet || (echo please run 'make reviewable' to include all changes && false)
	echo branch is clean

.PHONY: check-license-header
check-license-header: ## Run license header check.
	@./hack/license/header-check.sh

.PHONY: fix-license-header
fix-license-header: ## Run license header fix.
	@./hack/license/header-check.sh fix

##@ Helm Chart Tasks

bump-single-chart-appver.%: chart=$(word 2,$(subst ., ,$@))
bump-single-chart-appver.%:
ifeq ($(GOOS), darwin)
	sed -i '' "s/^appVersion:.*/appVersion: $(VERSION)/" deploy/$(chart)/Chart.yaml
else
	sed -i "s/^appVersion:.*/appVersion: $(VERSION)/" deploy/$(chart)/Chart.yaml
endif

bump-single-chart-ver.%: chart=$(word 2,$(subst ., ,$@))
bump-single-chart-ver.%:
ifeq ($(GOOS), darwin)
	sed -i '' "s/^version:.*/version: $(VERSION)/" deploy/$(chart)/Chart.yaml
else
	sed -i "s/^version:.*/version: $(VERSION)/" deploy/$(chart)/Chart.yaml
endif

.PHONY: bump-chart-ver
bump-chart-ver: \
	bump-single-chart-ver.helm \
	bump-single-chart-appver.helm \
	bump-single-chart-ver.apecloud-mysql \
	bump-single-chart-ver.apecloud-mysql-cluster \
	bump-single-chart-ver.apecloud-mysql-scale \
	bump-single-chart-ver.apecloud-mysql-scale-cluster \
	bump-single-chart-ver.clickhouse \
	bump-single-chart-ver.clickhouse-cluster \
	bump-single-chart-ver.kafka \
	bump-single-chart-ver.kafka-cluster \
	bump-single-chart-ver.mongodb \
	bump-single-chart-ver.mongodb-cluster \
	bump-single-chart-ver.nyancat \
	bump-single-chart-appver.nyancat \
	bump-single-chart-ver.postgresql \
	bump-single-chart-ver.postgresql-cluster \
	bump-single-chart-ver.postgresql-patroni-ha \
	bump-single-chart-ver.postgresql-patroni-ha-cluster \
	bump-single-chart-ver.redis \
	bump-single-chart-ver.redis-cluster \
	bump-single-chart-ver.milvus \
	bump-single-chart-ver.qdrant \
	bump-single-chart-ver.qdrant-cluster \
	bump-single-chart-ver.chatgpt-retrieval-plugin
bump-chart-ver: ## Bump helm chart version.

LOADBALANCER_CHART_VERSION=

.PHONY: helm-package
helm-package: bump-chart-ver ## Do helm package.
## it will pull down the latest charts that satisfy the dependencies, and clean up old dependencies.
## this is a hack fix: decompress the tgz from the depend-charts directory to the charts directory
## before dependency update.
	# cd $(CHART_PATH)/charts && ls ../depend-charts/*.tgz | xargs -n1 tar xf
	#$(HELM) dependency update --skip-refresh $(CHART_PATH)
	$(HELM) package deploy/loadbalancer
	mv loadbalancer-*.tgz deploy/helm/depend-charts/
	$(HELM) package deploy/apecloud-mysql
	mv apecloud-mysql-*.tgz deploy/helm/depend-charts/
	$(HELM) package deploy/postgresql
	mv postgresql-*.tgz deploy/helm/depend-charts/
	$(HELM) package $(CHART_PATH)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.7
CONTROLLER_TOOLS_VERSION ?= v0.9.0
HELM_VERSION ?= v3.9.0
CUE_VERSION ?= v0.4.3

KUSTOMIZE_INSTALL_SCRIPT ?= "$(GITHUB_PROXY)https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/kustomize 2>/dev/null))
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)
endif

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/controller-gen 2>/dev/null))
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
endif

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/setup-envtest 2>/dev/null))
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
endif


.PHONY: install-docker-buildx
install-docker-buildx: ## Create `docker buildx` builder.
	docker buildx create --platform linux/amd64,linux/arm64 --name x-builder --driver docker-container --use

.PHONY: golangci
golangci: GOLANGCILINT_VERSION = v1.51.2
golangci: ## Download golangci-lint locally if necessary.
ifneq ($(shell which golangci-lint),)
	@echo golangci-lint is already installed
GOLANGCILINT=$(shell which golangci-lint)
else ifeq (, $(shell which $(GOBIN)/golangci-lint))
	@{ \
	set -e ;\
	echo 'installing golangci-lint-$(GOLANGCILINT_VERSION)' ;\
	curl -sSfL $(GITHUB_PROXY)https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCILINT_VERSION) ;\
	echo 'Successfully installed' ;\
	}
GOLANGCILINT=$(GOBIN)/golangci-lint
else
	@echo golangci-lint is already installed
GOLANGCILINT=$(GOBIN)/golangci-lint
endif

.PHONY: staticchecktool
staticchecktool: ## Download staticcheck locally if necessary.
ifeq (, $(shell which staticcheck))
	@{ \
	set -e ;\
	echo 'installing honnef.co/go/tools/cmd/staticcheck' ;\
	go install honnef.co/go/tools/cmd/staticcheck@latest;\
	}
STATICCHECK=$(GOBIN)/staticcheck
else
STATICCHECK=$(shell which staticcheck)
endif

.PHONY: goimportstool
goimportstool: ## Download goimports locally if necessary.
ifeq (, $(shell which goimports))
	@{ \
	set -e ;\
	go install golang.org/x/tools/cmd/goimports@latest ;\
	}
GOIMPORTS=$(GOBIN)/goimports
else
GOIMPORTS=$(shell which goimports)
endif

.PHONY: cuetool
cuetool: ## Download cue locally if necessary.
ifeq (, $(shell which cue))
	@{ \
	set -e ;\
	go install cuelang.org/go/cmd/cue@$(CUE_VERSION) ;\
	}
CUE=$(GOBIN)/cue
else
CUE=$(shell which cue)
endif

.PHONY: helmtool
helmtool: ## Download helm locally if necessary.
ifeq (, $(shell which helm))
	@{ \
	set -e ;\
	go install github.com/helm/helm@$(HELM_VERSION);\
	}
HELM=$(GOBIN)/helm
else
HELM=$(shell which helm)
endif

.PHONY: minikube
minikube: ## Download minikube locally if necessary.
ifeq (, $(shell which minikube))
	@{ \
	set -e ;\
	echo 'installing minikube' ;\
	curl -Lo minikube https://storage.googleapis.com/minikube/releases/latest/minikube-$(GOOS)-$(GOARCH) && chmod +x minikube && sudo mv minikube /usr/local/bin ;\
	echo 'Successfully installed' ;\
	}
endif
MINIKUBE=$(shell which minikube)

.PHONY: kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (, $(shell which kubectl))
	@{ \
	set -e ;\
	echo 'installing kubectl' ;\
	curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/$(GOOS)/$(GOARCH)/kubectl" && chmod +x kubectl && sudo mv kubectl /usr/local/bin ;\
	echo 'Successfully installed' ;\
	}
endif
KUBECTL=$(shell which kubectl)


##@ Minikube
# using `minikube version: v1.29.0`, and use one of following k8s versions:
# K8S_VERSION ?= v1.22.15
K8S_VERSION ?= v1.23.15
# K8S_VERSION ?= v1.24.9
# K8S_VERSION ?= v1.25.5
# K8S_VERSION ?= v1.26.1

K8S_VERSION_MAJOR_MINOR=$(shell echo $(K8S_VERSION) | head -c 5)

# minikube v1.28+ support `--image-mirror-country=cn` for China mainland users.
MINIKUBE_IMAGE_MIRROR_COUNTRY ?= cn
MINIKUBE_START_ARGS ?= --memory=4g --cpus=4

ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.26)
	K8S_IMAGE_REPO ?= registry.k8s.io
	SIGSTORAGE_IMAGE_REPO ?= registry.k8s.io/sig-storage
endif

ifeq ($(MINIKUBE_IMAGE_MIRROR_COUNTRY), cn)
	K8S_IMAGE_REPO := registry.cn-hangzhou.aliyuncs.com/google_containers
	SIGSTORAGE_IMAGE_REPO := registry.cn-hangzhou.aliyuncs.com/google_containers
	MINIKUBE_START_ARGS := $(MINIKUBE_START_ARGS) --image-mirror-country=$(MINIKUBE_IMAGE_MIRROR_COUNTRY)
	MINIKUBE_REGISTRY_MIRROR ?= https://tenxhptk.mirror.aliyuncs.com
endif

K8S_IMAGE_REPO ?= k8s.gcr.io
SIGSTORAGE_IMAGE_REPO ?= k8s.gcr.io/sig-storage

KICBASE_IMG := kicbase/stable:v0.0.36
ETCT_IMG := $(K8S_IMAGE_REPO)/etcd:3.5.6-0
COREDNS_IMG := $(K8S_IMAGE_REPO)/coredns/coredns:v1.8.6
KUBE_APISERVER_IMG := $(K8S_IMAGE_REPO)/kube-apiserver:$(K8S_VERSION)
KUBE_SCHEDULER_IMG := $(K8S_IMAGE_REPO)/kube-scheduler:$(K8S_VERSION)
KUBE_CTLR_MGR_IMG := $(K8S_IMAGE_REPO)/kube-controller-manager:$(K8S_VERSION)
KUBE_PROXY_IMG := $(K8S_IMAGE_REPO)/kube-proxy:$(K8S_VERSION)

CSI_PROVISIONER_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-provisioner:v2.1.0
CSI_ATTACHER_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-attacher:v3.1.0
CSI_EXT_HMC_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-external-health-monitor-controller:v0.2.0
CSI_EXT_HMA_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-external-health-monitor-agent:v0.2.0
CSI_NODE_DRIVER_REG_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-node-driver-registrar:v2.0.1
LIVENESSPROBE_IMG := $(SIGSTORAGE_IMAGE_REPO)/livenessprobe:v2.2.0
CSI_RESIZER_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-resizer:v1.1.0
CSI_SNAPSHOTTER_IMG := $(SIGSTORAGE_IMAGE_REPO)/csi-snapshotter:v4.0.0
HOSTPATHPLUGIN_IMG := $(SIGSTORAGE_IMAGE_REPO)/hostpathplugin:v1.6.0
SNAPSHOT_CONTROLLER_IMG := $(SIGSTORAGE_IMAGE_REPO)/snapshot-controller:v4.0.0

STORAGE_PROVISIONER_IMG := gcr.io/k8s-minikube/storage-provisioner:v5
METRICS_SERVER_IMG := registry.k8s.io/metrics-server:v0.6.2

ifeq ($(MINIKUBE_IMAGE_MIRROR_COUNTRY), cn)
	STORAGE_PROVISIONER_IMG := $(K8S_IMAGE_REPO)/storage-provisioner:v5
	METRICS_SERVER_IMG := $(K8S_IMAGE_REPO)/metrics-server:v0.6.2
endif

PAUSE_IMG_TAG := 3.7
ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.22)
	PAUSE_IMG_TAG := 3.5
endif

ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.23)
	PAUSE_IMG_TAG := 3.6
endif

ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.24)
	PAUSE_IMG_TAG := 3.7
endif

ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.25)
	PAUSE_IMG_TAG := 3.8
endif

ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.26)
	PAUSE_IMG_TAG := 3.9
	COREDNS_IMG := $(K8S_IMAGE_REPO)/coredns/coredns:v1.9.3
endif

PAUSE_IMG := $(K8S_IMAGE_REPO)/pause:$(PAUSE_IMG_TAG)


ifneq ($(MINIKUBE_REGISTRY_MIRROR),)
	MINIKUBE_START_ARGS := $(MINIKUBE_START_ARGS) --registry-mirror=$(MINIKUBE_REGISTRY_MIRROR)
endif

ifeq ($(MINIKUBE_IMAGE_MIRROR_COUNTRY), cn)
	TAG_K8S_IMAGE_REPO := k8s.gcr.io
	TAG_SIGSTORAGE_IMAGE_REPO := k8s.gcr.io/sig-storage
ifeq ($(K8S_VERSION_MAJOR_MINOR), v1.26)
	TAG_K8S_IMAGE_REPO := registry.k8s.io
endif
endif

.PHONY: pull-all-images
pull-all-images: DOCKER_PULLQ=docker pull -q
pull-all-images: DOCKER_TAG=docker tag
pull-all-images: ## Pull K8s & minikube required container images.
	$(DOCKER_PULLQ) $(KICBASE_IMG)
	$(DOCKER_PULLQ) $(KUBE_APISERVER_IMG)
	$(DOCKER_PULLQ) $(KUBE_SCHEDULER_IMG)
	$(DOCKER_PULLQ) $(KUBE_CTLR_MGR_IMG)
	$(DOCKER_PULLQ) $(KUBE_PROXY_IMG)
	$(DOCKER_PULLQ) $(PAUSE_IMG)
	$(DOCKER_PULLQ) $(HOSTPATHPLUGIN_IMG)
	$(DOCKER_PULLQ) $(LIVENESSPROBE_IMG)
	$(DOCKER_PULLQ) $(CSI_PROVISIONER_IMG)
	$(DOCKER_PULLQ) $(CSI_ATTACHER_IMG)
	$(DOCKER_PULLQ) $(CSI_RESIZER_IMG)
	$(DOCKER_PULLQ) $(CSI_SNAPSHOTTER_IMG)
	$(DOCKER_PULLQ) $(SNAPSHOT_CONTROLLER_IMG)
	$(DOCKER_PULLQ) $(CSI_EXT_HMC_IMG)
	$(DOCKER_PULLQ) $(CSI_EXT_HMA_IMG)
	$(DOCKER_PULLQ) $(CSI_NODE_DRIVER_REG_IMG)
	$(DOCKER_PULLQ) $(STORAGE_PROVISIONER_IMG)
	$(DOCKER_PULLQ) $(METRICS_SERVER_IMG)
	# if image is using China mirror repository, re-tag it to original image repositories
ifeq ($(MINIKUBE_IMAGE_MIRROR_COUNTRY), cn)
	$(DOCKER_TAG) $(KUBE_APISERVER_IMG) $(TAG_K8S_IMAGE_REPO)/kube-apiserver:$(K8S_VERSION)
	$(DOCKER_TAG) $(KUBE_SCHEDULER_IMG) $(TAG_K8S_IMAGE_REPO)/kube-scheduler:$(K8S_VERSION)
	$(DOCKER_TAG) $(KUBE_CTLR_MGR_IMG) $(TAG_K8S_IMAGE_REPO)/kube-controller-manager:$(K8S_VERSION)
	$(DOCKER_TAG) $(KUBE_PROXY_IMG) $(TAG_K8S_IMAGE_REPO)/kube-proxy:$(K8S_VERSION)
	$(DOCKER_TAG) $(PAUSE_IMG) $(TAG_K8S_IMAGE_REPO)/pause:$(PAUSE_IMG_TAG)
	$(DOCKER_TAG) $(HOSTPATHPLUGIN_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/hostpathplugin:v1.6.0
	$(DOCKER_TAG) $(LIVENESSPROBE_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/livenessprobe:v2.2.0
	$(DOCKER_TAG) $(CSI_PROVISIONER_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-provisioner:v2.1.0
	$(DOCKER_TAG) $(CSI_ATTACHER_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-attacher:v3.1.0
	$(DOCKER_TAG) $(CSI_RESIZER_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-resizer:v1.1.0
	$(DOCKER_TAG) $(CSI_SNAPSHOTTER_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-snapshotter:v4.0.0
	$(DOCKER_TAG) $(SNAPSHOT_CONTROLLER_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/snapshot-controller:v4.0.0
	$(DOCKER_TAG) $(CSI_EXT_HMC_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-external-health-monitor-controller:v0.2.0
	$(DOCKER_TAG) $(CSI_EXT_HMA_IMG) $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-external-health-monitor-agent:v0.2.0
	$(DOCKER_TAG) $(CSI_NODE_DRIVER_REG_IMG)  $(TAG_SIGSTORAGE_IMAGE_REPO)/csi-node-driver-registrar:v2.0.1
	$(DOCKER_TAG) $(METRICS_SERVER_IMG) registry.k8s.io/metrics-server:v0.6.2
	$(DOCKER_TAG) $(STORAGE_PROVISIONER_IMG) gcr.io/k8s-minikube/storage-provisioner:v5
endif


.PHONY: minikube-start
minikube-start: IMG_CACHE_CMD=image load --daemon=true
minikube-start: minikube ## Start minikube cluster.
ifneq (, $(shell which minikube))
ifeq (, $(shell $(MINIKUBE) status -n minikube -ojson 2>/dev/null| jq -r '.Host' | grep Running))
	$(MINIKUBE) start --kubernetes-version=$(K8S_VERSION) $(MINIKUBE_START_ARGS) --base-image=$(KICBASE_IMG)
endif
endif
	$(MINIKUBE) update-context
	$(MINIKUBE) $(IMG_CACHE_CMD) $(KUBE_APISERVER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(KUBE_SCHEDULER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(KUBE_CTLR_MGR_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(KUBE_PROXY_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(PAUSE_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(HOSTPATHPLUGIN_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(LIVENESSPROBE_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_PROVISIONER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_ATTACHER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_RESIZER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_SNAPSHOTTER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_EXT_HMA_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_EXT_HMC_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(SNAPSHOT_CONTROLLER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_NODE_DRIVER_REG_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(STORAGE_PROVISIONER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(METRICS_SERVER_IMG)
	$(MINIKUBE) addons enable metrics-server
	$(MINIKUBE) addons enable volumesnapshots
	$(MINIKUBE) addons enable csi-hostpath-driver
	kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
	kubectl patch storageclass csi-hostpath-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
	kubectl patch volumesnapshotclass/csi-hostpath-snapclass --type=merge -p '{"metadata": {"annotations": {"snapshot.storage.kubernetes.io/is-default-class": "true"}}}'


.PHONY: minikube-delete
minikube-delete: minikube ## Delete minikube cluster.
	$(MINIKUBE) delete

.PHONY: minikube-run
minikube-run: manifests generate fmt vet minikube helmtool ## Start minikube cluster and helm install kubeblocks.
ifneq (, $(shell which minikube))
ifeq (, $(shell $(MINIKUBE) status -n minikube -ojson 2>/dev/null| jq -r '.Host' | grep Running))
	$(MINIKUBE) start --wait=all --kubernetes-version=$(K8S_VERSION) $(MINIKUBE_START_ARGS)
endif
endif
	kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
	$(HELM) upgrade --install kubeblocks deploy/helm --set versionOverride=$(VERSION),csi-hostpath-driver.enabled=true --reuse-values --wait --wait-for-jobs --atomic

.PHONY: minikube-run-fast
minikube-run-fast: minikube helmtool ## Fast start minikube cluster and helm install kubeblocks.
ifneq (, $(shell which minikube))
ifeq (, $(shell $(MINIKUBE) status -n minikube -ojson 2>/dev/null| jq -r '.Host' | grep Running))
	$(MINIKUBE) start --wait=all --kubernetes-version=$(K8S_VERSION) $(MINIKUBE_START_ARGS)
endif
endif
	kubectl patch storageclass standard -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
	$(HELM) upgrade --install kubeblocks deploy/helm --set versionOverride=$(VERSION),csi-hostpath-driver.enabled=true --reuse-values --wait --wait-for-jobs --atomic

##@ End-to-end (E2E) tests
.PHONY: render-smoke-testdata-manifests
render-smoke-testdata-manifests: ## Update E2E test dataset
	$(HELM) template mycluster deploy/apecloud-mysql-cluster > test/e2e/testdata/smoketest/wesql/00_wesqlcluster.yaml
	$(HELM) template mycluster deploy/postgresqlcluster > test/e2e/testdata/smoketest/postgresql/00_postgresqlcluster.yaml
	$(HELM) template mycluster deploy/redis > test/e2e/testdata/smoketest/redis/00_rediscluster.yaml
	$(HELM) template mycluster deploy/redis-cluster >> test/e2e/testdata/smoketest/redis/00_rediscluster.yaml

.PHONY: test-e2e
test-e2e: helm-package render-smoke-testdata-manifests ## Run E2E tests.
	$(MAKE) -e VERSION=$(VERSION) -C test/e2e run

# NOTE: include must be placed at the end
include docker/docker.mk
include cmd/cmd.mk

