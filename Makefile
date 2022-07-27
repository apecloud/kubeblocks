include dependency.mk

VERSION ?= 0.1.0

K3S_VERSION ?= v1.23.8+k3s1
K3D_VERSION ?= 5.4.4

K3S_IMG_TAG ?= $(subst +,-,$(K3S_VERSION))


GO ?= go
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell $(GO) env GOBIN))
GOBIN=$(shell $(GO) env GOPATH)/bin
else
GOBIN=$(shell $(GO) env GOBIN)
endif

export GONOPROXY=jihulab.com/infracreate
export GONOSUMDB=jihulab.com/infracreate
export GOPRIVATE=jihulab.com/infracreate
export GOPROXY=https://goproxy.cn,direct


#LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`  -X ./pkg/types/types.K3sImageTag=${K3S_IMG_TAG} -X ./pkg/types/types.K3dVersion=${K3D_VERSION} "
#LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`  -X jihulab.com/infracreate/dbaas-system/opencli/pkg/types/types.K3sImageTag=${K3S_IMG_TAG} -X jihulab.com/infracreate/dbaas-system/opencli/pkg/types/types.K3dVersion=${K3D_VERSION} "
LD_FLAGS="-s -w -X main.version=v${VERSION} -X main.buildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` -X main.gitCommit=`git rev-parse HEAD`"


.DEFAULT_GOAL := bin/opencli

bin/opencli:
	$(MAKE) bin/opencli.$(OS).$(ARCH)
	mv bin/opencli.$(OS).$(ARCH) bin/opencli

# Build binary
#bin/opencli.%: download_k3s_bin_script download_k3s_images go-check
bin/opencli.%: go-check
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${LD_FLAGS} -o $@ cmd/opencli/main.go


.PHONY: download_k3s_bin_script
download_k3s_bin_script:
	./hack/download_k3s.sh other ${ARCH} ${K3S_VERSION}

.PHONY: download_k3s_images
download_k3s_images:
	./hack/download_k3s.sh images ${ARCH} ${K3S_VERSION}

.PHONY: download_k3d
download_k3d:
	./hack/download_k3d_images.sh ${ARCH} ${K3S_IMG_TAG} ${K3D_VERSION}

.PHONY: clean
clean:
	rm -f bin/opencli*

lint: golangci
	$(GOLANGCILINT) run ./...

staticcheck: staticchecktool
	$(STATICCHECK) ./...

goimports:
	$(GOIMPORTS) -local jihulab.com/infracreate/dbaas-system/opencli -w $$(go list -f {{.Dir}} ./...)

.PHONY: go-check
go-check: fmt vet
	@mkdir -p bin/

# Run go fmt against code
.PHONY: fmt
fmt:
	$(GO) fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: test
test:
	$(GO) test ./... -coverprofile cover.out

.PHONY: mod-vendor
mod-vendor:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) mod verify

.PHONY: reviewable
reviewable: lint staticcheck fmt go-check
	$(GO) mod tidy -compat=1.17

.PHONY: check-diff
check-diff: reviewable
	git --no-pager diff
	git diff --quiet || (echo please run 'make reviewable' to include all changes && false)
	echo branch is clean
