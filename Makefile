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
VERSION ?= 1.0.0-alpha.0
GITHUB_PROXY ?=
INIT_ENV ?= false
TEST_TYPE ?= wesql
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=0 --tag)
GENERATED_CLIENT_PKG = "pkg/client"
GENERATED_DEEP_COPY_FILE = "zz_generated.deepcopy.go"
# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26.1
ENABLE_WEBHOOKS ?= false
SKIP_GO_GEN ?= true
CHART_PATH = deploy/helm
WEBHOOK_CERT_DIR ?= /tmp/k8s-webhook-server/serving-certs

# Go setup
export GO111MODULE = auto
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
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
## use following GOPROXY settings for Chinese mainland developers.
#GOPROXY := https://goproxy.cn
endif
export GOPROXY

LD_FLAGS="-s -w \
	-X github.com/apecloud/kubeblocks/version.Version=v${VERSION} \
	-X github.com/apecloud/kubeblocks/version.BuildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` \
	-X github.com/apecloud/kubeblocks/version.GitCommit=${GIT_COMMIT} \
	-X github.com/apecloud/kubeblocks/version.GitVersion=${GIT_VERSION}"

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

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
all: manager dataprotection kbagent ## Make all cmd binaries.

##@ Development

.PHONY: manifests
manifests: test-go-generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./cmd/manager/...;./apis/...;./controllers/..." output:crd:artifacts:config=config/crd/bases
	@$(MAKE) label-crds --no-print-directory
	@cp config/crd/bases/* $(CHART_PATH)/crds
	@cp config/rbac/role.yaml $(CHART_PATH)/config/rbac/role.yaml
	$(MAKE) client-sdk-gen

.PHONY: label-crds
label-crds:
	@for f in config/crd/bases/*.yaml; do \
		echo "applying app.kubernetes.io/name=kubeblocks label to $$f"; \
		kubectl label --overwrite -f $$f --local=true -o yaml app.kubernetes.io/name=kubeblocks > bin/crd.yaml; \
		mv bin/crd.yaml $$f; \
	done

.PHONY: preflight-manifests
preflight-manifests: generate ## Generate external Preflight API
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./externalapis/preflight/..." output:crd:artifacts:config=config/crd/preflight

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/...;./externalapis/..."

.PHONY: client-sdk-gen
client-sdk-gen: module ## Generate CRD client code.
	@./hack/client-sdk-gen.sh

.PHONY: manager-go-generate
manager-go-generate: ## Run go generate against lifecycle manager code.
ifeq ($(SKIP_GO_GEN), false)
endif

.PHONY: test-go-generate
test-go-generate: ## Run go generate against test code.
	$(GO) generate -x ./pkg/testutil/k8s/mocks/...

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GOFMT) -l -w -s $$(git ls-files --exclude-standard | grep "\.go$$" | grep -v $(GENERATED_CLIENT_PKG) | grep -v $(GENERATED_DEEP_COPY_FILE))

.PHONY: vet
vet: test-go-generate ## Run go vet against code.
	GOOS=$(GOOS) $(GO) vet -mod=mod ./...

.PHONY: lint-fast
lint-fast: staticcheck vet golangci-lint # [INTERNAL] Run all lint job against code.

.PHONY: lint
lint: test-go-generate generate ## Run default lint job against code.
	$(MAKE) golangci-lint

.PHONY: golangci-lint
golangci-lint: golangci-lint-bin generate ## Run golangci-lint against code.
	$(GOLANGCILINT) run ./...

.PHONY: staticcheck
staticcheck: staticcheck-bin test-go-generate generate ## Run staticcheck against code.
	$(STATICCHECK) ./...

.PHONY: build-checks
build-checks: fmt vet goimports lint-fast ## Run build checks.

.PHONY: mod-download
mod-download: ## Run go mod download against go modules.
	$(GO) mod download

.PHONY: mod-vendor
mod-vendor: module ## Run go mod vendor against go modules.
	$(GO) mod vendor

.PHONY: module
module: ## Run go mod tidy->verify against go modules.
	$(GO) mod tidy -compat=1.24
	$(GO) mod verify

TEST_PACKAGES ?= ./pkg/... ./apis/... ./controllers/... ./cmd/...

CLUSTER_TYPES=minikube k3d
.PHONY: add-k8s-host
add-k8s-host:  ## add DNS to /etc/hosts when k8s cluster is minikube or k3d
ifneq (, $(findstring $(EXISTING_CLUSTER_TYPE), $(CLUSTER_TYPES)))
ifeq (, $(shell sed -n "/^127.0.0.1[[:space:]]*host.$(EXISTING_CLUSTER_TYPE).internal/p" /etc/hosts))
	sudo bash -c 'echo "127.0.0.1 host.$(EXISTING_CLUSTER_TYPE).internal" >> /etc/hosts'
endif
endif


OUTPUT_COVERAGE=-coverprofile cover.out.tmp && grep -v "zz_generated.deepcopy.go" cover.out.tmp > cover.out && rm cover.out.tmp
.PHONY: test-current-ctx
test-current-ctx: manifests generate add-k8s-host ## Run operator controller tests with current $KUBECONFIG context. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	USE_EXISTING_CLUSTER=true KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -p 1 $(TEST_PACKAGES) $(OUTPUT_COVERAGE)

.PHONY: test-fast
test-fast: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -short $(TEST_PACKAGES)  $(OUTPUT_COVERAGE)

.PHONY: test
test: manifests generate test-go-generate add-k8s-host test-fast ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.

.PHONY: race
race:
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -race $(TEST_PACKAGES)

.PHONY: test-delve
test-delve: manifests generate envtest ## Run tests.
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
goimports: goimports-bin ## Run goimports against code.
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(git ls-files|grep "\.go$$" | grep -v $(GENERATED_CLIENT_PKG) | grep -v $(GENERATED_DEEP_COPY_FILE))

.PHONY: api-doc
api-doc: $(LOCALBIN) ## generate API reference manual.
	cd hack/docgen/api && go build -o "$(LOCALBIN)/docgen-api" ./main.go
	"$(LOCALBIN)/docgen-api" -api-dir github.com/apecloud/kubeblocks/apis -config ./hack/docgen/api/gen-api-doc-config.json -template-dir ./hack/docgen/api/template -out-dir ./docs/developer_docs/api-reference/

.PHONY: doc
doc: api-doc ## generate all documents.

##@ Operator Controller Manager

.PHONY: manager
manager: generate manager-go-generate build-checks ## Build manager binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/manager ./cmd/manager/main.go

.PHONY: dataprotection
dataprotection: generate build-checks ## Build dataprotection binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/dataprotection ./cmd/dataprotection/main.go

.PHONY: kbagent
kbagent: generate build-checks
	$(GO) build -ldflags=${LD_FLAGS} -o bin/kbagent ./cmd/kbagent/main.go

.PHONY: helmhook
helmhook:
	$(GO) build -o bin/helmhook ./cmd/helmhook/main.go

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

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

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
	$(GO) mod tidy -compat=1.23

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

bump-single-chart-appver.%: chart=$*
bump-single-chart-appver.%:
ifeq ($(GOOS), darwin)
	sed -i '' "s/^appVersion:.*/appVersion: $(VERSION)/" deploy/$(chart)/Chart.yaml
else
	sed -i "s/^appVersion:.*/appVersion: $(VERSION)/" deploy/$(chart)/Chart.yaml
endif

bump-single-chart-ver.%: chart=$*
bump-single-chart-ver.%:
ifeq ($(GOOS), darwin)
	sed -i '' "s/^version:.*/version: $(VERSION)/" deploy/$(chart)/Chart.yaml
else
	sed -i "s/^version:.*/version: $(VERSION)/" deploy/$(chart)/Chart.yaml
endif

.PHONY: bump-chart-ver
bump-chart-ver: \
	bump-single-chart-ver.helm \
	bump-single-chart-appver.helm
bump-chart-ver: ## Bump helm chart version.

.PHONY: helm-package
helm-package: helmtool bump-chart-ver ## Do helm package.
	$(HELM) package $(CHART_PATH)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.1.1
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.21

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)

KUSTOMIZE_INSTALL_SCRIPT ?= "$(GITHUB_PROXY)https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(LOCALBIN) ## Download kustomize locally if necessary.
	@[ -f $(KUSTOMIZE) ] || { \
		echo "Installing kustomize with version $(KUSTOMIZE_VERSION)"; \
		curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN) && \
		mv "$(LOCALBIN)/kustomize" "$(KUSTOMIZE)"; \
	}

.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Download controller-gen locally if necessary.
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(LOCALBIN) ## Download envtest-setup locally if necessary.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

GOLANGCILINT_VERSION = v1.64.8
GOLANGCILINT = $(LOCALBIN)/golangci-lint-$(GOLANGCILINT_VERSION)
.PHONY: golangci-lint-bin
golangci-lint-bin: $(LOCALBIN) ## Download golangci-lint locally if necessary.
	@[ -f $(GOLANGCILINT) ] || { \
  		echo "Installing golangci-lint with version $(GOLANGCILINT_VERSION)"; \
		curl -sSfL $(GITHUB_PROXY)https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCILINT_VERSION) && \
		mv "$(LOCALBIN)/golangci-lint" "$(GOLANGCILINT)"; \
	}

STATICCHECK_VERSION = v0.6.1
STATICCHECK = $(LOCALBIN)/staticcheck-$(STATICCHECK_VERSION)
.PHONY: staticcheck-bin
staticcheck-bin: $(LOCALBIN) ## Download staticcheck locally if necessary.
	$(call go-install-tool,$(STATICCHECK),honnef.co/go/tools/cmd/staticcheck,$(STATICCHECK_VERSION))

GOIMPORTS_VERSION = v0.34.0
GOIMPORTS = $(LOCALBIN)/goimports-$(GOIMPORTS_VERSION)
.PHONY: goimports-bin
goimports-bin: $(LOCALBIN) ## Download goimports locally if necessary.
	$(call go-install-tool,$(GOIMPORTS),golang.org/x/tools/cmd/goimports,$(GOIMPORTS_VERSION))

.PHONY: helmtool
helmtool: ## Download helm locally if necessary.
ifeq (, $(shell which helm))
	@{ \
	set -e ;\
	echo 'installing helm' ;\
	curl $(GITHUB_PROXY)https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash;\
	echo 'Successfully installed' ;\
	}
# Hopefully the command will be installed in PATH
HELM=helm
else
HELM=$(shell which helm)
endif

.PHONY: kubectl
kubectl: ## Download kubectl locally if necessary.
ifeq (, $(shell which kubectl))
	@{ \
	set -e ;\
	echo 'installing kubectl' ;\
	curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/$(GOOS)/$(GOARCH)/kubectl" && chmod +x kubectl && sudo mv kubectl /usr/local/bin ;\
	echo 'Successfully installed' ;\
	}
# Hopefully the command will be installed in PATH
KUBECTL=kubectl
else
KUBECTL=$(shell which kubectl)
endif

.PHONY: sync-examples
sync-examples: ## Sync examples from kubeblocks-addons.
	@./hack/sync-examples.sh

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Installing $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

# NOTE: include must be placed at the end
include docker/docker.mk
