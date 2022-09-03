include dependency.mk

IMG ?= docker.io/infracreate/dbctl
CLI_VERSION ?= 0.4.0
TAG ?= v$(CLI_VERSION)

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

export GONOPROXY=github.com/apecloud
export GONOSUMDB=github.com/apecloud
export GOPRIVATE=github.com/apecloud
export GOPROXY=https://goproxy.cn,direct


LD_FLAGS="-s -w \
	-X github.com/apecloud/kubeblocks/version.BuildDate=`date -u +'%Y-%m-%dT%H:%M:%SZ'` \
	-X github.com/apecloud/kubeblocks/version.GitCommit=`git rev-parse HEAD` \
	-X github.com/apecloud/kubeblocks/version.Version=${CLI_VERSION} \
	-X github.com/apecloud/kubeblocks/version.K3sImageTag=${K3S_IMG_TAG} \
	-X github.com/apecloud/kubeblocks/version.K3dVersion=${K3D_VERSION}"


.DEFAULT_GOAL := bin/dbctl

bin/dbctl:
	$(MAKE) bin/dbctl.$(OS).$(ARCH)
	mv bin/dbctl.$(OS).$(ARCH) bin/dbctl

# Build binary
#bin/dbctl.%: download_k3s_bin_script download_k3s_images go-check
bin/dbctl.%: go-check
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${LD_FLAGS} -o $@ cmd/dbctl/main.go


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
	rm -f bin/dbctl*

lint: golangci
	$(GOLANGCILINT) run ./... --timeout=5m

staticcheck: staticchecktool
	$(STATICCHECK) ./...

goimports: goimportstool
	$(GOIMPORTS) -local github.com/apecloud/kubeblocks -w $$(go list -f {{.Dir}} ./...)

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


# Run docker build
.PHONY: docker-build
docker-build: clean bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64
	docker build . -t ${IMG}:${TAG}
    docker push ${IMG}:${TAG}

.PHONY: reviewable
reviewable: lint staticcheck fmt go-check
	$(GO) mod tidy -compat=1.17

.PHONY: check-diff
check-diff: reviewable
	git --no-pager diff
	git diff --quiet || (echo please run 'make reviewable' to include all changes && false)
	echo branch is clean


.PHONY: ci-test-pre
ci-test-pre:
	$(MAKE)
	bin/dbctl playground destroy
	bin/dbctl playground init

.PHONY: ci-test
ci-test: ci-test-pre test
	bin/dbctl playground destroy
	go tool cover -html=cover.out -o cover.html
	go tool cover -func=cover.out -o cover_total.out
	python3 /datatestsuites/infratest.py -t 0 -c filepath:./cover_total.out,percent:60%

