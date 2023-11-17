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
VERSION ?= 0.8.0-alpha.0
GITHUB_PROXY ?=
INIT_ENV ?= false
TEST_TYPE ?= wesql
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=0 --tag)
GENERATED_CLIENT_PKG = "pkg/client"
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


LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`"
# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

TAG_LATEST ?= false
BUILDX_ENABLED ?= ""
ifeq ($(BUILDX_ENABLED), "")
	ifeq ($(shell docker buildx inspect 2>/dev/null | awk '/Status/ { print $$2 }'), running)
		BUILDX_ENABLED = true
	else
		BUILDX_ENABLED = false
	endif
endif
BUILDX_BUILDER ?= "x-builder"

define BUILDX_ERROR
buildx not enabled, refusing to run this recipe
endef

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
all: manager dataprotection  lorry reloader ## Make all cmd binaries.

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
	$(GO) generate -x ./pkg/configuration/proto
endif

.PHONY: test-go-generate
test-go-generate: ## Run go generate against test code.
	$(GO) generate -x ./pkg/testutil/k8s/mocks/...
	$(GO) generate -x ./pkg/configuration/container/mocks/...
	$(GO) generate -x ./pkg/configuration/proto/mocks/...

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GOFMT) -l -w -s $$(git ls-files --exclude-standard | grep "\.go$$" | grep -v $(GENERATED_CLIENT_PKG))

.PHONY: vet
vet: ## Run go vet against code.
	GOOS=$(GOOS) $(GO) vet -mod=mod ./...

.PHONY: cue-fmt
cue-fmt: cuetool ## Run cue fmt against code.
	git ls-files --exclude-standard | grep "\.cue$$" | xargs $(CUE) fmt
	git ls-files --exclude-standard | grep "\.cue$$" | xargs $(CUE) fix

.PHONY: lint-fast
lint-fast: staticcheck vet golangci-lint # [INTERNAL] Run all lint job against code.

.PHONY: lint
lint: test-go-generate generate ## Run default lint job against code.
	$(MAKE) golangci-lint

.PHONY: golangci-lint
golangci-lint: golangci generate ## Run golangci-lint against code.
	$(GOLANGCILINT) run ./...

.PHONY: staticcheck
staticcheck: staticchecktool test-go-generate generate ## Run staticcheck against code.
	$(STATICCHECK) ./...

.PHONY: build-checks
build-checks: generate fmt vet goimports lint-fast ## Run build checks.

.PHONY: mod-download
mod-download: ## Run go mod download against go modules.
	$(GO) mod download

.PHONY: mod-vendor
mod-vendor: module ## Run go mod vendor against go modules.
	$(GO) mod vendor

.PHONY: module
module: ## Run go mod tidy->verify against go modules.
	$(GO) mod tidy -compat=1.21
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

.PHONY: test-integration
test-integration: manifests generate envtest add-k8s-host ## Run tests. if existing k8s cluster is k3d or minikube, specify EXISTING_CLUSTER_TYPE.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test ./test/integration

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
goimports: goimportstool ## Run goimports against code.
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(git ls-files|grep "\.go$$" | grep -v $(GENERATED_CLIENT_PKG))


.PHONY: lorryctl-doc
lorryctl-doc: generate test-go-generate ## generate CLI command reference manual.
	$(GO) run ./hack/docgen/lorryctl/main.go ./docs/user_docs/lorryctl

.PHONY: api-doc
api-doc:  ## generate API reference manual.
	@./hack/docgen/api/generate.sh


##@ Operator Controller Manager

.PHONY: manager
manager: cue-fmt generate manager-go-generate test-go-generate build-checks ## Build manager binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/manager ./cmd/manager/main.go

.PHONY: dataprotection
dataprotection: generate test-go-generate build-checks ## Build dataprotection binary.
	$(GO) build -ldflags=${LD_FLAGS} -o bin/dataprotection ./cmd/dataprotection/main.go

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
	$(GO) mod tidy -compat=1.21

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
helm-package: bump-chart-ver ## Do helm package.
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
KUSTOMIZE_VERSION ?= v5.1.1
CONTROLLER_TOOLS_VERSION ?= v0.12.1
CUE_VERSION ?= v0.4.3

KUSTOMIZE_INSTALL_SCRIPT ?= "$(GITHUB_PROXY)https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/kustomize 2>/dev/null))
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)
endif

.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Download controller-gen locally if necessary.
	@{ \
	set -e ;\
	if [ ! -f "$(CONTROLLER_GEN)" ] || [ "$$($(CONTROLLER_GEN) --version 2>&1 | awk '{print $$NF}')" != "$(CONTROLLER_TOOLS_VERSION)" ]; then \
        echo 'Installing controller-gen@$(CONTROLLER_TOOLS_VERSION)...' ;\
        GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) ;\
        echo 'Successfully installed' ;\
    fi \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/setup-envtest 2>/dev/null))
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
endif

.PHONY: install-docker-buildx
install-docker-buildx: ## Create `docker buildx` builder.
	@if ! docker buildx inspect $(BUILDX_BUILDER) > /dev/null; then \
		echo "Buildx builder $(BUILDX_BUILDER) does not exist, creating..."; \
		docker buildx create --name=$(BUILDX_BUILDER) --use --driver=docker-container --platform linux/amd64,linux/arm64; \
	else \
		echo "Buildx builder $(BUILDX_BUILDER) already exists"; \
	fi

.PHONY: golangci
golangci: GOLANGCILINT_VERSION = v1.54.2
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
	echo 'installing helm' ;\
	curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash;\
	echo 'Successfully installed' ;\
	}
HELM=$(GOBIN)/helm
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
endif
KUBECTL=$(shell which kubectl)

##@ End-to-end (E2E) tests
.PHONY: render-smoke-testdata-manifests
render-smoke-testdata-manifests: addonsPath=addons/addons
render-smoke-testdata-manifests: fetch-addons ## Update E2E test dataset
ifeq ($(TEST_TYPE), wesql)
	$(HELM) dependency build $(addonsPath)/apecloud-mysql-cluster --skip-refresh
	$(HELM) template mysql-cluster $(addonsPath)/apecloud-mysql-cluster > test/e2e/testdata/smoketest/wesql/00_wesqlcluster.yaml
else ifeq ($(TEST_TYPE), postgresql)
	$(HELM) dependency build $(addonsPath)/postgresql-cluster --skip-refresh
	$(HELM) template pg-cluster $(addonsPath)/postgresql-cluster > test/e2e/testdata/smoketest/postgresql/00_postgresqlcluster.yaml
else ifeq ($(TEST_TYPE), redis)
	$(HELM) dependency build $(addonsPath)/redis-cluster --skip-refresh
	$(HELM) template redis-cluster $(addonsPath)/redis-cluster > test/e2e/testdata/smoketest/redis/00_rediscluster.yaml
else ifeq ($(TEST_TYPE), mongodb)
	$(HELM) dependency build $(addonsPath)/mongodb-cluster --skip-refresh
	$(HELM) template mongodb-cluster $(addonsPath)/mongodb-cluster > test/e2e/testdata/smoketest/mongodb/00_mongodbcluster.yaml
else ifeq ($(TEST_TYPE), pulsar)
	$(HELM) dependency build $(addonsPath)/pulsar-cluster --skip-refresh
	$(HELM) template pulsar-cluster -s templates/cluster.yaml $(addonsPath)/pulsar-cluster > test/e2e/testdata/smoketest/pulsar/00_pulsarcluster.yaml
else ifeq ($(TEST_TYPE), nebula)
	$(HELM) dependency build $(addonsPath)/nebula-cluster --skip-refresh
	$(HELM) upgrade --install nebula $(addonsPath)/nebula
	$(HELM) template nebula-cluster $(addonsPath)/nebula-cluster > test/e2e/testdata/smoketest/nebula/00_nebulacluster.yaml
else ifeq ($(TEST_TYPE), greptimedb)
	$(HELM) dependency build $(addonsPath)/greptimedb-cluster --skip-refresh
	$(HELM) upgrade --install greptimedb $(addonsPath)/greptimedb
	$(HELM) template greptimedb-cluster $(addonsPath)/greptimedb-cluster > test/e2e/testdata/smoketest/greptimedb/00_greptimedbcluster.yaml
else ifeq ($(TEST_TYPE), starrocks)
	$(HELM) dependency build $(addonsPath)/starrocks-cluster --skip-refresh
	$(HELM) upgrade --install starrocks $(addonsPath)/starrocks
	$(HELM) template starrocks-cluster $(addonsPath)/starrocks-cluster > test/e2e/testdata/smoketest/starrocks/00_starrocksbcluster.yaml
else ifeq ($(TEST_TYPE), risingwave)
	$(HELM) dependency build $(addonsPath)/risingwave-cluster --skip-refresh
	$(HELM) upgrade --install etcd $(addonsPath)/etcd
	$(HELM) upgrade --install risingwave $(addonsPath)/risingwave
	$(HELM) template risingwave-cluster $(addonsPath)/risingwave-cluster > test/e2e/testdata/smoketest/risingwave/00_risingwavecluster.yaml
else ifeq ($(TEST_TYPE), etcd)
	$(HELM) dependency build $(addonsPath)/etcd-cluster --skip-refresh
	$(HELM) upgrade --install etcd $(addonsPath)/etcd
	$(HELM) template etcd-cluster -s templates/cluster.yaml $(addonsPath)/etcd-cluster > test/e2e/testdata/smoketest/etcd/00_etcdcluster.yaml
else ifeq ($(TEST_TYPE), oracle)
	$(HELM) dependency build $(addonsPath)/oracle-mysql-cluster --skip-refresh
	$(HELM) upgrade --install oracle $(addonsPath)/oracle-mysql
	$(HELM) template oracle-cluster $(addonsPath)/oracle-mysql-cluster > test/e2e/testdata/smoketest/oracle/00_oraclecluster.yaml
else ifeq ($(TEST_TYPE), kafka)
	$(HELM) dependency build $(addonsPath)/kafka-cluster --skip-refresh
	$(HELM) upgrade --install kafka $(addonsPath)/kafka
	$(HELM) template kafka-cluster $(addonsPath)/kafka-cluster > test/e2e/testdata/smoketest/kafka/00_kafkacluster.yaml
else ifeq ($(TEST_TYPE), foxlake)
	$(HELM) dependency build $(addonsPath)/foxlake-cluster --skip-refresh
	$(HELM) upgrade --install foxlake $(addonsPath)/foxlake
	$(HELM) template foxlake-cluster $(addonsPath)/foxlake-cluster > test/e2e/testdata/smoketest/foxlake/00_foxlakecluster.yaml
else ifeq ($(TEST_TYPE), oceanbase)
	$(HELM) dependency build $(addonsPath)/oceanbase-cluster --skip-refresh
	$(HELM) upgrade --install oceanbase $(addonsPath)/oceanbase
	$(HELM) template oceanbase-cluster $(addonsPath)/oceanbase-cluster > test/e2e/testdata/smoketest/oceanbase/00_oceanbasecluster.yaml
else ifeq ($(TEST_TYPE), official-postgresql)
	$(HELM) dependency build $(addonsPath)/official-postgresql-cluster --skip-refresh
	$(HELM) upgrade --install official-postgresql $(addonsPath)/official-postgresql
	$(HELM) template official-pg $(addonsPath)/official-postgresql-cluster > test/e2e/testdata/smoketest/official-postgresql/00_official_pgcluster.yaml
else ifeq ($(TEST_TYPE), openldap)
	$(HELM) dependency build $(addonsPath)/openldap-cluster --skip-refresh
	$(HELM) upgrade --install openldap $(addonsPath)/openldap
	$(HELM) template openldap-cluster $(addonsPath)/openldap-cluster > test/e2e/testdata/smoketest/openldap/00_openldapcluster.yaml
else ifeq ($(TEST_TYPE), orioledb)
	$(HELM) dependency build $(addonsPath)/orioledb-cluster --skip-refresh
	$(HELM) upgrade --install orioledb $(addonsPath)/orioledb
	$(HELM) template oriole-cluster $(addonsPath)/orioledb-cluster > test/e2e/testdata/smoketest/orioledb/00_orioledbcluster.yaml
else ifeq ($(TEST_TYPE), weaviate)
	$(HELM) dependency build $(addonsPath)/weaviate-cluster --skip-refresh
	$(HELM) upgrade --install weaviate $(addonsPath)/weaviate
	$(HELM) template weaviate-cluster $(addonsPath)/weaviate-cluster > test/e2e/testdata/smoketest/weaviate/00_weaviatecluster.yaml
else ifeq ($(TEST_TYPE), mysql-80)
	$(HELM) dependency build $(addonsPath)/mysql-cluster --skip-refresh
	$(HELM) upgrade --install mysql $(addonsPath)/mysql
	$(HELM) template mysqlcluster $(addonsPath)/mysql-cluster > test/e2e/testdata/smoketest/mysql-80/00_mysqlcluster.yaml
else ifeq ($(TEST_TYPE), mysql-57)
	$(HELM) dependency build $(addonsPath)/mysql-cluster --skip-refresh
	$(HELM) upgrade --install mysql $(addonsPath)/mysql
else ifeq ($(TEST_TYPE), polardbx)
	$(HELM) dependency build $(addonsPath)/polardbx-cluster --skip-refresh
	$(HELM) upgrade --install polardbx $(addonsPath)/polardbx
	$(HELM) template pxc $(addonsPath)/polardbx-cluster > test/e2e/testdata/smoketest/polardbx/00_polardbxcluster.yaml
else ifeq ($(TEST_TYPE), opensearch)
	$(HELM) dependency build $(addonsPath)/opensearch-cluster --skip-refresh
	$(HELM) upgrade --install opensearch $(addonsPath)/opensearch
	$(HELM) template opensearch-cluster $(addonsPath)/opensearch-cluster > test/e2e/testdata/smoketest/opensearch/00_opensearchcluster.yaml
else ifeq ($(TEST_TYPE), elasticsearch)
	$(HELM) dependency build $(addonsPath)/elasticsearch-cluster --skip-refresh
	$(HELM) upgrade --install elasticsearch $(addonsPath)/elasticsearch
	$(HELM) template elasticsearch-cluster $(addonsPath)/elasticsearch-cluster > test/e2e/testdata/smoketest/elasticsearch/00_elasticsearchcluster.yaml
else ifeq ($(TEST_TYPE), llm)
	$(HELM) dependency build $(addonsPath)/llm-cluster --skip-refresh
	$(HELM) upgrade --install llm $(addonsPath)/llm
	$(HELM) template llm-cluster $(addonsPath)/llm-cluster > test/e2e/testdata/smoketest/llm/00_llmcluster.yaml
else ifeq ($(TEST_TYPE), tdengine)
	$(HELM) dependency build $(addonsPath)/tdengine-cluster --skip-refresh
	$(HELM) upgrade --install tdengine $(addonsPath)/tdengine
	$(HELM) template td-cluster $(addonsPath)/tdengine-cluster > test/e2e/testdata/smoketest/tdengine/00_tdenginecluster.yaml
else ifeq ($(TEST_TYPE), milvus)
	$(HELM) dependency build $(addonsPath)/milvus-cluster --skip-refresh
	$(HELM) upgrade --install milvus $(addonsPath)/milvus
	$(HELM) template milvus-cluster $(addonsPath)/milvus-cluster > test/e2e/testdata/smoketest/milvus/00_milvuscluster.yaml
else ifeq ($(TEST_TYPE), clickhouse)
	$(HELM) dependency build $(addonsPath)/clickhouse-cluster --skip-refresh
	$(HELM) upgrade --install clickhouse $(addonsPath)/clickhouse
	$(HELM) template test -s templates/cluster.yaml $(addonsPath)/clickhouse-cluster > test/e2e/testdata/smoketest/clickhouse/00_clickhousecluster.yaml
else ifeq ($(TEST_TYPE), zookeeper)
	$(HELM) dependency build $(addonsPath)/zookeeper-cluster --skip-refresh
	$(HELM) upgrade --install zookeeper $(addonsPath)/zookeeper
	$(HELM) template zk-cluster $(addonsPath)/zookeeper-cluster > test/e2e/testdata/smoketest/zookeeper/00_zookeepercluster.yaml
else ifeq ($(TEST_TYPE), mariadb)
	$(HELM) dependency build $(addonsPath)/mariadb-cluster --skip-refresh
	$(HELM) upgrade --install mariadb $(addonsPath)/mariadb
	$(HELM) template mariadb-cluster $(addonsPath)/mariadb-cluster > test/e2e/testdata/smoketest/mariadb/00_mariadbcluster.yaml
else
	$(error "test type does not exist")
endif

.PHONY: test-e2e
test-e2e: helm-package install-s3-csi-driver render-smoke-testdata-manifests ## Run E2E tests.
	$(MAKE) -e VERSION=$(VERSION) PROVIDER=$(PROVIDER) REGION=$(REGION) SECRET_ID=$(SECRET_ID) SECRET_KEY=$(SECRET_KEY) INIT_ENV=$(INIT_ENV) TEST_TYPE=$(TEST_TYPE) SKIP_CASE=$(SKIP_CASE) CONFIG_TYPE=$(CONFIG_TYPE) -C test/e2e run

.PHONY: render-smoke-testdata-manifests-local
render-smoke-testdata-manifests-local: addonsPath=addons/addons## Helm Install CD And CV
render-smoke-testdata-manifests-local: fetch-addons
ifeq ($(TEST_TYPE), wesql)
	$(HELM) upgrade --install wesql $(addonsPath)/apecloud-mysql
else ifeq ($(TEST_TYPE), postgresql)
	$(HELM) upgrade --install postgresql $(addonsPath)/postgresql
else ifeq ($(TEST_TYPE), mongodb)
	$(HELM) upgrade --install  mongodb $(addonsPath)/mongodb
else ifeq ($(TEST_TYPE), redis)
	$(HELM) upgrade --install redis $(addonsPath)/redis
else ifeq ($(TEST_TYPE), pulsar)
	$(HELM) upgrade --install pulsar $(addonsPath)/pulsar
else ifeq ($(TEST_TYPE), nebula)
	$(HELM) upgrade --install nebula $(addonsPath)/nebula
else ifeq ($(TEST_TYPE), greptimedb)
	$(HELM) upgrade --install greptimedb $(addonsPath)/greptimedb
else ifeq ($(TEST_TYPE), starrocks)
	$(HELM) upgrade --install starrocks $(addonsPath)/starrocks
else ifeq ($(TEST_TYPE), risingwave)
	$(HELM) upgrade --install etcd $(addonsPath)/etcd
	$(HELM) upgrade --install risingwave $(addonsPath)/risingwave
else ifeq ($(TEST_TYPE), etcd)
	$(HELM) upgrade --install etcd $(addonsPath)/etcd
else ifeq ($(TEST_TYPE), oracle)
	$(HELM) upgrade --install oracle-mysql $(addonsPath)/oracle-mysql
else ifeq ($(TEST_TYPE), kafka)
	$(HELM) upgrade --install kafka $(addonsPath)/kafka
else ifeq ($(TEST_TYPE), foxlake)
	$(HELM) upgrade --install foxlake $(addonsPath)/foxlake
else ifeq ($(TEST_TYPE), oceanbase)
	$(HELM) upgrade --install oceanbase $(addonsPath)/oceanbase
else ifeq ($(TEST_TYPE), oceanbase)
	$(HELM) upgrade --install official-postgresql $(addonsPath)/official-postgresql
else ifeq ($(TEST_TYPE), openldap)
	$(HELM) upgrade --install openldap $(addonsPath)/openldap
else ifeq ($(TEST_TYPE), weaviate)
	$(HELM) upgrade --install weaviate $(addonsPath)/weaviate
else ifeq ($(TEST_TYPE), mysql-80)
	$(HELM) upgrade --install mysql $(addonsPath)/mysql
else ifeq ($(TEST_TYPE), mysql-57)
	$(HELM) upgrade --install mysql $(addonsPath)/mysql
else ifeq ($(TEST_TYPE), polardbx)
	$(HELM) upgrade --install polardbx $(addonsPath)/polardbx
else ifeq ($(TEST_TYPE), opensearch)
	$(HELM) upgrade --install opensearch $(addonsPath)/opensearch
else ifeq ($(TEST_TYPE), elasticsearch)
	$(HELM) upgrade --install elasticsearch $(addonsPath)/elasticsearch
else ifeq ($(TEST_TYPE), llm)
	$(HELM) upgrade --install llm $(addonsPath)/llm
else ifeq ($(TEST_TYPE), milvus)
	$(HELM) upgrade --install milvus $(addonsPath)/milvus
else ifeq ($(TEST_TYPE), clickhouse)
	$(HELM) upgrade --install clickhouse $(addonsPath)/clickhouse
else ifeq ($(TEST_TYPE), zookeeper)
	$(HELM) upgrade --install zookeeper $(addonsPath)/zookeeper
else ifeq ($(TEST_TYPE), mariadb)
	$(HELM) upgrade --install mariadb $(addonsPath)/mariadb
else
	$(error "test type does not exist")
endif

.PHONY: test-e2e-local
test-e2e-local: generate-cluster-role install-s3-csi-driver render-smoke-testdata-manifests-local render-smoke-testdata-manifests ## Run E2E tests on local.
	$(MAKE) -e TEST_TYPE=$(TEST_TYPE) -C test/e2e run

.PHONY: generate-cluster-role
generate-cluster-role:
	$(HELM) template -s templates/rbac/cluster_pod_required_role.yaml deploy/helm | kubectl apply -f -

.PHONY: install-s3-csi-driver
install-s3-csi-driver:
	$(HELM) upgrade --install csi-s3 https://github.com/apecloud/helm-charts/releases/download/csi-s3-0.7.0/csi-s3-0.7.0.tgz

# NOTE: include must be placed at the end
include docker/docker.mk
include cmd/cmd.mk
