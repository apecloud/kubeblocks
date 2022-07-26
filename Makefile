include dependency.mk

K3S_VERSION ?= v1.21.10+k3s1
OS ?= darwin
ARCH ?= arm64

.DEFAULT_GOAL := darwin-arm64
linux-amd64 linux-arm64: download_k3s_bin_script download_k3s_images
	GOOS=${OS} GOARCH=${ARCH} \
	go build -o bin/opencli-${OS}-${ARCH} \
	cmd/opencli/main.go

darwin-amd64 darwin-arm64 windows-amd64: download_k3d download_k3s_images
	GOOS=${OS} GOARCH=${ARCH} \
	go build -o bin/opencli-${OS}-${ARCH} \
	cmd/opencli/main.go

download_k3s_bin_script:
	./hack/download_k3s.sh other ${ARCH} ${K3S_VERSION}

download_k3s_images:
	./hack/download_k3s.sh images ${ARCH} ${K3S_VERSION}

download_k3d:
	./hack/download_k3d_images.sh ${ARCH}

.PHONY: clean
clean:
	rm -f bin/opencli

lint: golangci
	$(GOLANGCILINT) run ./...

staticcheck: staticchecktool
	$(STATICCHECK) ./...

fmt: goimports
	$(GOIMPORTS) -local github.com/infracreate/opencli -w $$(go list -f {{.Dir}} ./...)

go-check:
	go fmt ./...
	go vet ./...

reviewable: lint staticcheck fmt go-check
	go mod tidy -compat=1.17

check-diff: reviewable
	git --no-pager diff
	git diff --quiet || (echo please run 'make reviewable' to include all changes && false)
	echo branch is clean
