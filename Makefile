export GONOPROXY=github.com/apecloud
export GONOSUMDB=github.com/apecloud
export GOPRIVATE=github.com/apecloud
export GOPROXY=https://goproxy.cn

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.1

ENABLE_WEBHOOKS ?= false

APP_NAME = opendbaas-core

# Image URL to use all building/pushing image targets
IMG ?= docker.io/infracreate/$(APP_NAME)
VERSION ?= 0.1.0-alpha.4
CHART_PATH = deploy/helm


# NOTES: get OCI registry auth. credential from Sunrun(yimeisun)
CHART_OCI_REGISTRY ?= yimeisun.azurecr.io/helm-chart

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
all: manager dbctl ## Make all cmd binaries.

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:generateEmbeddedObjectMeta=true webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	@cp config/crd/bases/* $(CHART_PATH)/crds

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	$(GO) vet ./...

.PHONY: cue-fmt
cue-fmt: cuetool ## Run cue fmt against code.
	$(CUE) fmt controllers/dbaas/cue/*.cue

.PHONY: cue-vet
cue-vet: cuetool ## Run cue vet against code.
	$(CUE) vet controllers/dbaas/cue/*.cue

.PHONY: fast-lint
fast-lint: staticchecktool  # [INTERNAL] fast lint
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
mod-vendor: ## Run go mod tidy->vendor->verify against go modules.
	$(GO) mod tidy -compat=1.18
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: ctrl-test-current-ctx
ctrl-test-current-ctx: manifests generate fmt vet ## Run operator controller tests with current $KUBECONFIG context
	USE_EXISTING_CLUSTER=true KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test ./controllers/... -coverprofile cover.out

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test ./... -coverprofile cover.out

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
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(go list -f {{.Dir}} ./...)


##@ CLI
CLI_IMG ?= docker.io/infracreate/dbctl
CLI_VERSION ?= 0.4.0
CLI_TAG ?= v$(CLI_VERSION)
K3S_VERSION ?= v1.23.8+k3s1
K3D_VERSION ?= 5.4.4
K3S_IMG_TAG ?= $(subst +,-,$(K3S_VERSION))

CLI_LD_FLAGS ="-s -w \
	-X github.com/apecloud/kubeblocks/version.BuildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` \
	-X github.com/apecloud/kubeblocks/version.GitCommit=`git rev-parse HEAD` \
	-X github.com/apecloud/kubeblocks/version.Version=${CLI_VERSION} \
	-X github.com/apecloud/kubeblocks/version.K3sImageTag=${K3S_IMG_TAG} \
	-X github.com/apecloud/kubeblocks/version.K3dVersion=${K3D_VERSION}"



bin/dbctl.%: ## Cross build bin/dbctl.$(OS).$(ARCH) CLI.
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CLI_LD_FLAGS} -o $@ cmd/dbctl/main.go

.PHONY: dbctl
dbctl: OS=$(shell $(GO) env GOOS)
dbctl: ARCH=$(shell $(GO) env GOARCH)
dbctl: build-checks ## Build bin/dbctl CLI.
	$(MAKE) bin/dbctl.$(OS).$(ARCH)
	mv bin/dbctl.$(OS).$(ARCH) bin/dbctl

.PHONY: clean
clean-dbctl: ## Clean bin/dbctl* CLI tools.
	rm -f bin/dbctl*

.PHONY: docker-build-cli
docker-build-cli: clean-dbctl build-checks bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64 ## Build docker image with the dbctl.
	docker build . -t ${CLI_IMG}:${CLI_TAG} -f Dockerfile.dbctl
	docker push ${CLI_IMG}:${CLI_TAG}


##@ Operator Controller Manager

.PHONY: manager
manager: cue-fmt build-checks ## Build manager binary.
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
run-delve: manifests generate fmt vet  ## Run Delve debugger.
	$(GO) build -gcflags "all=-trimpath=$(shell go env GOPATH)" -o bin/manager ./cmd/manager/main.go
	dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./bin/manager


.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG}:${VERSION} -t ${IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION}
endif
endif


.PHONY: docker-push
docker-push: ## Push docker image with the manager.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${IMG}:latest
else
	docker push ${IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest --push
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} --push
endif
endif


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

.PHONY: dry-run
dry-run: manifests kustomize ## Dry-run deploy job.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	mkdir -p dry-run
	$(KUSTOMIZE) build config/default > dry-run/manifests.yaml

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ CI

.PHONY: ci-test-pre
ci-test-pre: dbctl ## Prepare CI test environment.
	bin/dbctl playground destroy
	bin/dbctl playground init

.PHONY: ci-test
ci-test: ci-test-pre test ## Run CI tests.
	bin/dbctl playground destroy
	$(GO) tool cover -html=cover.out -o cover.html

##@ Contributor

.PHONY: reviewable
reviewable: build-checks ## Run code checks to proceed with PR reviews.
	$(GO) mod tidy -compat=1.18

.PHONY: check-diff
check-diff: reviewable ## Run git code diff checker.
	git --no-pager diff
	git diff --quiet || (echo please run 'make reviewable' to include all changes && false)
	echo branch is clean

##@ Helm Chart Tasks

.PHONY: bump-chart-ver
bump-chart-ver: ## Bump helm chart version.
	sed -i '' "s/^version:.*/version: $(VERSION)/" $(CHART_PATH)/Chart.yaml
	sed -i '' "s/^appVersion:.*/appVersion: $(VERSION)/" $(CHART_PATH)/Chart.yaml

.PHONY: helm-package
helm-package: bump-chart-ver ## Do helm package.
	$(HELM) package $(CHART_PATH)

.PHONY: helm-push
helm-push: helm-package ## Do helm package and push.
	$(HELM) push $(APP_NAME)-$(VERSION).tgz oci://$(CHART_OCI_REGISTRY)

##@ WeSQL Cluster Helm Chart Tasks

WESQL_CLUSTER_CHART_PATH = deploy/helm/wesqlcluster
WESQL_CLUSTER_CHART_NAME = wesqlcluster
WESQL_CLUSTER_CHART_VERSION ?= 0.1.1

.PHONY: bump-chart-ver-wqsql-cluster
bump-chart-ver-wqsql-cluster: ## Bump WeSQL Clsuter helm chart version.
	sed -i '' "s/^version:.*/version: $(WESQL_CLUSTER_CHART_VERSION)/" $(WECLUSTER_CHART_PATH)/Chart.yaml
	# sed -i '' "s/^appVersion:.*/appVersion: $(WESQL_CLUSTER_CHART_VERSION)/" $(WECLUSTER_CHART_PATH)/Chart.yaml

.PHONY: helm-package-wqsql-cluster
helm-package-wqsql-cluster: bump-chart-ver-wqsql-cluster ## Do WeSQL Clsuter helm package.
	$(HELM) package $(WECLUSTER_CHART_PATH)

.PHONY: helm-push-wqsql-cluster
helm-push-wqsql-cluster: helm-package-wqsql-cluster ## Do WeSQL Clsuter helm package and push.
	$(HELM) push $(WESQL_CLUSTER_CHART_NAME)-$(WESQL_CLUSTER_CHART_VERSION).tgz oci://$(CHART_OCI_REGISTRY)


##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GITHUB_PROXY ?= https://github.91chi.fun/
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
	curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest


.PHONY: install-docker-buildx
install-docker-buildx: ## Create `docker buildx` builder.
	docker buildx create --platform linux/amd64,linux/arm64 --name x-builder --driver docker-container --use

.PHONY: golangci
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

.PHONY: brew-install-prerequisite
brew-install-prerequisite: ## Use `brew install` to install required dependencies. 
	brew install go@1.18 kubebuilder delve golangci-lint staticcheck kustomize step cue
