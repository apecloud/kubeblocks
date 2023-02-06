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

export GO111MODULE ?= on
# export GOPROXY ?= https://proxy.golang.org
export GOPROXY ?= https://goproxy.cn
export GOSUMDB ?= sum.golang.org
export GONOPROXY ?= github.com/apecloud
export GONOSUMDB ?= github.com/apecloud
export GOPRIVATE ?= github.com/apecloud


GITHUB_PROXY ?= https://github.91chi.fun/

GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=0 --tag)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.1

ENABLE_WEBHOOKS ?= false

APP_NAME = kubeblocks


VERSION ?= 0.4.0
CHART_PATH = deploy/helm

WEBHOOK_CERT_DIR ?= /tmp/k8s-webhook-server/serving-certs

GO ?= go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO) env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif

# Go module support: set `-mod=vendor` to use the vendored sources.
# See also hack/make.sh.
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
  GO:=GO111MODULE=on $(GO)
  MOD_VENDOR=-mod=vendor
endif

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

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64


# BUILDX_PLATFORMS ?= $(subst -,/,$(ARCH))
BUILDX_PLATFORMS ?= linux/amd64,linux/arm64
BUILDX_OUTPUT_TYPE ?= docker

LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`"

TAG_LATEST ?= false

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
all: manager kbcli probe agamotto reloader ## Make all cmd binaries.

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./apis/...;./controllers/dbaas/...;./controllers/dataprotection/...;./controllers/k8score/...;./cmd/manager/...;./internal/..." output:crd:artifacts:config=config/crd/bases
	@cp config/crd/bases/* $(CHART_PATH)/crds
	@cp config/rbac/role.yaml $(CHART_PATH)/config/rbac/role.yaml
	$(CONTROLLER_GEN) rbac:roleName=loadbalancer-role  paths="./controllers/loadbalancer;./cmd/loadbalancer/controller" output:dir=config/loadbalancer

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."

.PHONY: go-generate
go-generate: ## Run go generate against code.
	$(GO) generate -x ./...
	$(MAKE) fix-license-header

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	GOOS=linux $(GO) vet ./...

.PHONY: cue-fmt
cue-fmt: cuetool ## Run cue fmt against code.
	git ls-files | grep "\.cue$$" | xargs $(CUE) fmt
	git ls-files | grep "\.cue$$" | xargs $(CUE) fix

.PHONY: fast-lint
fast-lint: golangci staticcheck  # [INTERNAL] fast lint
	$(GOLANGCILINT) run ./...

.PHONY: lint
lint: generate ## Run golangci-lint against code.
	$(MAKE) fast-lint

.PHONY: staticcheck
staticcheck: staticchecktool ## Run staticcheck against code.
	$(STATICCHECK) ./...

.PHONY: loggercheck
loggercheck: loggerchecktool ## Run loggercheck against code.
	$(LOGGERCHECK) ./...

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

.PHONY: test
test: manifests generate fmt vet envtest add-k8s-host test-probe ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -short -coverprofile cover.out $(TEST_PACKAGES)

.PHONY: test-uec
test-uec: # manifests generate fmt vet envtest add-k8s-host ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test $(TEST_PACKAGES) -run UseExistingCluster

.PHONY: test-delve
test-delve: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" dlv --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient test $(TEST_PACKAGES)

.PHONY: test-webhook-enabled
test-webhook-enabled: ## Run tests with webhooks enabled.
	$(MAKE) test ENABLE_WEBHOOKS=true

.PHONY: cover-report
cover-report: cover-report-probe ## Generate cover.html from cover.out
	$(GO) tool cover -html=cover.out -o cover.html
ifeq ($(GOOS), darwin)
	open ./cover.html
else
	echo "open cover.html with a HTML viewer."
endif


.PHONY: goimports
goimports: goimportstool ## Run goimports against code.
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(go list -f {{.Dir}} ./...)


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
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CLI_LD_FLAGS} -o $@ cmd/cli/main.go

.PHONY: kbcli
kbcli: OS=$(shell $(GO) env GOOS)
kbcli: ARCH=$(shell $(GO) env GOARCH)
kbcli: build-checks ## Build bin/kbcli.
	$(MAKE) bin/kbcli.$(OS).$(ARCH)
	mv bin/kbcli.$(OS).$(ARCH) bin/kbcli

.PHONY: clean
clean-kbcli: ## Clean bin/kbcli*.
	rm -f bin/kbcli*

.PHONY: doc
kbcli-doc: build-checks ## generate CLI command reference manual.
	$(GO) run ./hack/docgen/cli/main.go ./docs/user_docs/cli

##@ Load Balancer

.PHONY: loadbalancer
loadbalancer: go-generate build-checks  ## Build loadbalancer binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/loadbalancer-controller ./cmd/loadbalancer/controller
	$(GO) build -ldflags=${LD_FLAGS} -o bin/loadbalancer-agent ./cmd/loadbalancer/agent

##@ Operator Controller Manager

.PHONY: manager
manager: cue-fmt generate go-generate build-checks ## Build manager binary.
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
	$(GO) run ./cmd/manager/main.go -zap-devel=false -zap-encoder=console -zap-time-encoding=iso8601

# Run with Delve for development purposes against the configured Kubernetes cluster in ~/.kube/config
# Delve is a debugger for the Go programming language. More info: https://github.com/go-delve/delve
GO_PACKAGE=./cmd/manager/main.go
ARGUMENTS=
DEBUG_PORT=2345
run-delve: manifests generate fmt vet  ## Run Delve debugger.
	dlv --listen=:$(DEBUG_PORT) --headless=true --api-version=2 --accept-multiclient debug $(GO_PACKAGE) -- $(ARGUMENTS)


##@ agamotto cmd

AGAMOTTO_LD_FLAGS = "-s -w \
    -X github.com/prometheus/common/version.Version=$(VERSION) \
    -X github.com/prometheus/common/version.Revision=$(GIT_COMMIT) \
    -X github.com/prometheus/common/version.BuildUser=apecloud \
    -X github.com/prometheus/common/version.BuildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'`"

bin/agamotto.%: ## Cross build bin/agamotto.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${AGAMOTTO_LD_FLAGS} -o $@ ./cmd/agamotto/main.go

.PHONY: agamotto
agamotto: OS=$(shell $(GO) env GOOS)
agamotto: ARCH=$(shell $(GO) env GOARCH)
agamotto: build-checks ## Build agamotto related binaries
	$(MAKE) bin/agamotto.${OS}.${ARCH}
	mv bin/agamotto.${OS}.${ARCH} bin/agamotto

.PHONY: clean
clean-agamotto: ## Clean bin/agamotto.
	rm -f bin/agamotto

##@ reloader cmd

RELOADER_LD_FLAGS = "-s -w"

bin/reloader.%: ## Cross build bin/reloader.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${RELOADER_LD_FLAGS} -o $@ ./cmd/reloader/main.go

.PHONY: reloader
reloader: OS=$(shell $(GO) env GOOS)
reloader: ARCH=$(shell $(GO) env GOARCH)
reloader: build-checks ## Build reloader related binaries
	$(MAKE) bin/reloader.${OS}.${ARCH}
	mv bin/reloader.${OS}.${ARCH} bin/reloader

.PHONY: clean
clean-reloader: ## Clean bin/reloader.
	rm -f bin/reloader

##@ cue-helper

CUE_HELPER_LD_FLAGS = "-s -w"

bin/cue-helper.%: ## Cross build bin/cue-helper.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CUE_HELPER_LD_FLAGS} -o $@ ./cmd/reloader/tools/cue_auto_generator.go

.PHONY: cue-helper
cue-helper: OS=$(shell $(GO) env GOOS)
cue-helper: ARCH=$(shell $(GO) env GOARCH)
cue-helper: build-checks ## Build cue-helper related binaries
	$(MAKE) bin/cue-helper.${OS}.${ARCH}
	mv bin/cue-helper.${OS}.${ARCH} bin/cue-helper

.PHONY: clean
clean-cue-helper: ## Clean bin/cue-helper.
	rm -f bin/cue-helper


##@ probe cmd

PROBE_LD_FLAGS = "-s -w"

bin/probe.%: ## Cross build bin/probe.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${PROBE_LD_FLAGS} -o $@  ./cmd/probe/main.go

.PHONY: probe
probe: OS=$(shell $(GO) env GOOS)
probe: ARCH=$(shell $(GO) env GOARCH)
probe: build-checks ## Build probe related binaries
	$(MAKE) bin/probe.${OS}.${ARCH}
	mv bin/probe.${OS}.${ARCH} bin/probe

.PHONY: clean
clean-probe: ## Clean bin/probe.
	rm -f bin/probe

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

##@ CI

.PHONY:
install-git-hooks: githookstool ## Install git hooks.
	git hooks install
	git hooks

.PHONY: ci-test-pre
ci-test-pre: kbcli ## Prepare CI test environment.
	bin/kbcli playground destroy
	bin/kbcli playground init

.PHONY: ci-test
ci-test: ci-test-pre test ## Run CI tests.
	bin/kbcli playground destroy
	$(GO) tool cover -html=cover.out -o cover.html

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

.PHONY: bump-chart-ver
bump-chart-ver: ## Bump helm chart version.
ifeq ($(GOOS), darwin)
	sed -i '' "s/^version:.*/version: $(VERSION)/" $(CHART_PATH)/Chart.yaml
	sed -i '' "s/^appVersion:.*/appVersion: $(VERSION)/" $(CHART_PATH)/Chart.yaml
else
	sed -i "s/^version:.*/version: $(VERSION)/" $(CHART_PATH)/Chart.yaml
	sed -i "s/^appVersion:.*/appVersion: $(VERSION)/" $(CHART_PATH)/Chart.yaml
endif


.PHONY: helm-package
helm-package: bump-chart-ver ## Do helm package.
	$(HELM) package $(CHART_PATH) --dependency-update

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
golangci: GOLANGCILINT_VERSION = v1.49.0
golangci: ## Download golangci-lint locally if necessary.
ifneq ($(shell which golangci-lint),)
	echo golangci-lint is already installed
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
	echo golangci-lint is already installed
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


.PHONY: loggerchecktool
loggerchecktool: ## Download loggercheck locally if necessary.
ifeq (, $(shell which loggercheck))
	@{ \
	set -e ;\
	echo 'installing github.com/timonwong/loggercheck/cmd/loggercheck' ;\
	go install github.com/timonwong/loggercheck/cmd/loggercheck@latest;\
	}
LOGGERCHECK=$(GOBIN)/loggercheck
else
LOGGERCHECK=$(shell which loggercheck)
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

.PHONY: githookstool
githookstool: ## Download git-hooks locally if necessary.
ifeq (, $(shell which git-hook))
	@{ \
	set -e ;\
	go install github.com/git-hooks/git-hooks@latest;\
	}
endif



.PHONY: oras
oras: ORAS_VERSION=0.14.1
oras: ## Download ORAS locally if necessary.
ifeq (, $(shell which oras))
	@{ \
	set -e ;\
	echo 'installing oras' ;\
	curl -LO $(GITHUB_PROXY)https://github.com/oras-project/oras/releases/download/v$(ORAS_VERSION)/oras_$(ORAS_VERSION)_$(GOOS)_$(GOARCH).tar.gz && \
	mkdir -p oras-install/ && \
	tar -zxf oras_$(ORAS_VERSION)_*.tar.gz -C oras-install/ && \
	sudo mv oras-install/oras /usr/local/bin/ && \
	rm -rf oras_$(ORAS_VERSION)_*.tar.gz oras-install/ ;\
	echo 'Successfully installed' ;\
	}
endif
ORAS=$(shell which oras)


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


.PHONY: brew-install-prerequisite
brew-install-prerequisite: ## Use `brew install` to install required dependencies.
	brew install go@1.19 kubebuilder delve golangci-lint staticcheck kustomize step cue oras jq yq git-hooks-go

##@ Minikube
K8S_VERSION ?= v1.22.15
MINIKUBE_REGISTRY_MIRROR ?= https://tenxhptk.mirror.aliyuncs.com
MINIKUBE_IMAGE_REPO ?= registry.cn-hangzhou.aliyuncs.com/google_containers
MINIKUBE_START_ARGS = --memory=4g --cpus=4

KICBASE_IMG=$(MINIKUBE_IMAGE_REPO)/kicbase:v0.0.33
PAUSE_IMG=$(MINIKUBE_IMAGE_REPO)/pause:3.5
METRICS_SERVER_IMG=$(MINIKUBE_IMAGE_REPO)/metrics-server:v0.6.1
CSI_PROVISIONER_IMG=$(MINIKUBE_IMAGE_REPO)/csi-provisioner:v2.1.0
CSI_ATTACHER_IMG=$(MINIKUBE_IMAGE_REPO)/csi-attacher:v3.1.0
CSI_EXT_HMC_IMG=$(MINIKUBE_IMAGE_REPO)/csi-external-health-monitor-controller:v0.2.0
CSI_EXT_HMA_IMG=$(MINIKUBE_IMAGE_REPO)/csi-external-health-monitor-agent:v0.2.0
CSI_NODE_DRIVER_REG_IMG=$(MINIKUBE_IMAGE_REPO)/csi-node-driver-registrar:v2.0.1
LIVENESSPROBE_IMG=$(MINIKUBE_IMAGE_REPO)/livenessprobe:v2.2.0
CSI_RESIZER_IMG=$(MINIKUBE_IMAGE_REPO)/csi-resizer:v1.1.0
CSI_SNAPSHOTTER_IMG=$(MINIKUBE_IMAGE_REPO)/csi-snapshotter:v4.0.0
HOSTPATHPLUGIN_IMG=$(MINIKUBE_IMAGE_REPO)/hostpathplugin:v1.6.0
STORAGE_PROVISIONER_IMG=$(MINIKUBE_IMAGE_REPO)/storage-provisioner:v5
SNAPSHOT_CONTROLLER_IMG=$(MINIKUBE_IMAGE_REPO)/snapshot-controller:v4.0.0

.PHONY: pull-all-images
pull-all-images: # Pull required container images
	docker pull -q $(PAUSE_IMG) &
	docker pull -q $(HOSTPATHPLUGIN_IMG) &
	docker pull -q $(LIVENESSPROBE_IMG) &
	docker pull -q $(CSI_PROVISIONER_IMG) &
	docker pull -q $(CSI_ATTACHER_IMG) &
	docker pull -q $(CSI_RESIZER_IMG) &
	docker pull -q $(CSI_RESIZER_IMG) &
	docker pull -q $(CSI_SNAPSHOTTER_IMG) &
	docker pull -q $(SNAPSHOT_CONTROLLER_IMG) &
	docker pull -q $(CSI_EXT_HMC_IMG) &
	docker pull -q $(CSI_NODE_DRIVER_REG_IMG) &
	docker pull -q $(STORAGE_PROVISIONER_IMG) &
	docker pull -q $(METRICS_SERVER_IMG) &
	docker pull -q $(KICBASE_IMG)

.PHONY: minikube-start
# minikube-start: IMG_CACHE_CMD=ssh --native-ssh=false docker pull
minikube-start: IMG_CACHE_CMD=image load --daemon=true
minikube-start: pull-all-images minikube ## Start minikube cluster.
ifneq (, $(shell which minikube))
ifeq (, $(shell $(MINIKUBE) status -n minikube -ojson 2>/dev/null| jq -r '.Host' | grep Running))
	$(MINIKUBE) start --kubernetes-version=$(K8S_VERSION) --registry-mirror=$(REGISTRY_MIRROR) --image-repository=$(MINIKUBE_IMAGE_REPO) $(MINIKUBE_START_ARGS)
endif
endif
	$(MINIKUBE) update-context
	$(MINIKUBE) $(IMG_CACHE_CMD) $(HOSTPATHPLUGIN_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(LIVENESSPROBE_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_PROVISIONER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_ATTACHER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_RESIZER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_SNAPSHOTTER_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_EXT_HMA_IMG)
	$(MINIKUBE) $(IMG_CACHE_CMD) $(CSI_EXT_HMC_IMG)
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

##@ Docker containers
include docker/docker.mk


##@ Test E2E
.PHONY: test-e2e
test-e2e: ## Test End-to-end.
	$(MAKE) -e VERSION=$(VERSION) -C test/e2e run
