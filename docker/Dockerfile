# Build the manager binary
ARG DIST_IMG=gcr.io/distroless/static:nonroot

ARG GO_VERSION=1.23.10-alpine

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS builder

## docker buildx build injected build-args:
#BUILDPLATFORM — matches the current machine. (e.g. linux/amd64)
#BUILDOS — os component of BUILDPLATFORM, e.g. linux
#BUILDARCH — e.g. amd64, arm64, riscv64
#BUILDVARIANT — used to set ARM variant, e.g. v7
#TARGETPLATFORM — The value set with --platform flag on build
#TARGETOS - OS component from --platform, e.g. linux
#TARGETARCH - Architecture from --platform, e.g. arm64
#TARGETVARIANT

ARG TARGETOS
ARG TARGETARCH

ARG GOPROXY
#ARG GOPROXY=https://goproxy.cn
ENV GOPROXY=${GOPROXY}

ARG VERSION
ARG GIT_COMMIT
ARG GIT_VERSION
ARG BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}
ENV GIT_VERSION=${GIT_VERSION}

ARG LD_FLAGS="-s -w \
    -X github.com/apecloud/kubeblocks/version.Version=${VERSION} \
	-X github.com/apecloud/kubeblocks/version.BuildDate=${BUILD_DATE} \
	-X github.com/apecloud/kubeblocks/version.GitCommit=${GIT_COMMIT} \
	-X github.com/apecloud/kubeblocks/version.GitVersion=${GIT_VERSION}"

WORKDIR /src
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
#COPY cmd/manager/main.go cmd/manager/main.go
#COPY cmd/manager/ cmd/manager/
#COPY apis/ apis/
#COPY pkg/ pkg/
#COPY controllers/ controllers/
#COPY test/testdata/testdata.go test/testdata/testdata.go

RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go env && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -o /out/manager ./cmd/manager/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM ${DIST_IMG} AS dist

WORKDIR /
COPY --from=builder /out/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
