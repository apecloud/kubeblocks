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
all: manager kbcli probe reloader ## Make all cmd binaries.

##@ Development

.PHONY: manifests
manifests: test-go-generate controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./cmd/manager/...;./apis/...;./controllers/...;./internal/..." output:crd:artifacts:config=config/crd/bases
	@cp config/crd/bases/* $(CHART_PATH)/crds
	@cp config/rbac/role.yaml $(CHART_PATH)/config/rbac/role.yaml

.PHONY: preflight-manifests
preflight-manifests: generate ## Generate external Preflight API
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./externalapis/preflight/..." output:crd:artifacts:config=config/crd/preflight

.PHONY: generate
generate: CODEGEN_GENERATORS=all # deepcopy,defaulter,client,lister,informer or all
generate: OUTPUT_PACKAGE=github.com/apecloud/kubeblocks/pkg/client
generate: OUTPUT_DIR=./clientgen_work_temp
generate: APIS_PACKAGE=github.com/apecloud/kubeblocks/apis
generate: CODEGEN_GROUP_VERSIONS="apps.kubeblocks.io:v1alpha1 dataprotection.kubeblocks.io:v1alpha1"
generate: controller-gen client-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	# $(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."
	# $(CLIENT_GEN) --clientset-name versioned --input-base "" --input="./apis/dbaas/v1alpha1" --go-header-file ./hack/boilerplate.go.txt --output-package clientset
	mkdir -p $(OUTPUT_DIR)
	bash ./vendor/k8s.io/code-generator/generate-groups.sh $(CODEGEN_GENERATORS) $(OUTPUT_PACKAGE) $(APIS_PACKAGE) "$(CODEGEN_GROUP_VERSIONS)" \
		--output-base $(OUTPUT_DIR) \
		--go-header-file ./hack/boilerplate_apache2.go.txt
	rm -rf ./pkg/client/{clientset,informers,listers}
	mv "$(OUTPUT_DIR)"/$(OUTPUT_PACKAGE)/* ./pkg/client
#   echo "Generating clientset for ${GROUPS_WITH_VERSIONS} at ${OUTPUT_PKG}/${CLIENTSET_PKG_NAME:-clientset}"
#   "${gobin}/client-gen" --clientset-name "${CLIENTSET_NAME_VERSIONED:-versioned}" --input-base "" --input "$(codegen::join , "${FQ_APIS[@]}")" --output-package "${OUTPUT_PKG}/${CLIENTSET_PKG_NAME:-clientset}" "$@"

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

.PHONY: lint-fast
lint-fast: staticcheck vet golangci-lint # [INTERNAL] Run all lint job against code.

.PHONY: lint
lint: test-go-generate generate ## Run default lint job against code.
	$(MAKE) golangci-lint

.PHONY: golangci-lint
golangci-lint: golangci ## Run golangci-lint against code.
	$(GOLANGCILINT) run ./...

.PHONY: staticcheck
staticcheck: staticchecktool ## Run staticcheck against code.
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
	$(GO) mod tidy -compat=1.20
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

.PHONY: race
race:
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test -race $(TEST_PACKAGES)

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

bin/kbcli.%: test-go-generate ## Cross build bin/kbcli.$(OS).$(ARCH).
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

.PHONY: kbcli-doc
kbcli-doc: generate test-go-generate ## generate CLI command reference manual.
	$(GO) run ./hack/docgen/cli/main.go ./docs/user_docs/cli



.PHONY: api-doc
api-doc:  ## generate API reference manual.
	@./hack/docgen/api/generate.sh


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
	$(GO) mod tidy -compat=1.20

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
	bump-single-chart-ver.redis \
	bump-single-chart-ver.redis-cluster \
	bump-single-chart-ver.milvus \
	bump-single-chart-ver.milvus-cluster \
	bump-single-chart-ver.qdrant \
	bump-single-chart-ver.qdrant-cluster \
	bump-single-chart-ver.weaviate \
	bump-single-chart-ver.weaviate-cluster \
	bump-single-chart-ver.chatgpt-retrieval-plugin
bump-chart-ver: ## Bump helm chart version.

.PHONY: helm-package
helm-package: bump-chart-ver ## Do helm package.
## it will pull down the latest charts that satisfy the dependencies, and clean up old dependencies.
## this is a hack fix: decompress the tgz from the depend-charts directory to the charts directory
## before dependency update.
	# cd $(CHART_PATH)/charts && ls ../depend-charts/*.tgz | xargs -n1 tar xf
	#$(HELM) dependency update --skip-refresh $(CHART_PATH)
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
CLIENT_GEN ?= $(LOCALBIN)/client-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.7
CONTROLLER_TOOLS_VERSION ?= v0.9.0
CLIENT_GEN_VERSION ?= v0.26.1
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

.PHONY: client-gen
client-gen: $(CLIENT_GEN) ## Download client-gen locally if necessary.
$(CLIENT_GEN): $(LOCALBIN)
ifeq (, $(shell ls $(LOCALBIN)/client-gen 2>/dev/null))
	GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/client-gen@$(CLIENT_GEN_VERSION)
endif

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
render-smoke-testdata-manifests: ## Update E2E test dataset
	$(HELM) template mycluster deploy/apecloud-mysql-cluster > test/e2e/testdata/smoketest/wesql/00_wesqlcluster.yaml
	$(HELM) template mycluster deploy/postgresql-cluster > test/e2e/testdata/smoketest/postgresql/00_postgresqlcluster.yaml
	$(HELM) template mycluster deploy/redis-cluster > test/e2e/testdata/smoketest/redis/00_rediscluster.yaml
	$(HELM) template mycluster deploy/mongodb-cluster > test/e2e/testdata/smoketest/mongodb/00_mongodbcluster.yaml


.PHONY: test-e2e
test-e2e: helm-package render-smoke-testdata-manifests ## Run E2E tests.
	$(MAKE) -e VERSION=$(VERSION) PROVIDER=$(PROVIDER) REGION=$(REGION) SECRET_ID=$(SECRET_ID) SECRET_KEY=$(SECRET_KEY) -C test/e2e run

# NOTE: include must be placed at the end
include docker/docker.mk
include cmd/cmd.mk
