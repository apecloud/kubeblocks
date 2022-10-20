# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.18 as builder

## docker buildx buid injected build-args:
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


ARG LD_FLAGS="-s -w"

ENV GONOPROXY=github.com/apecloud
ENV GONOSUMDB=github.com/apecloud
ENV GOPRIVATE=github.com/apecloud
ENV GOPROXY=https://goproxy.cn

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/manager/main.go cmd/manager/main.go
COPY cmd/agamotto/main.go cmd/agamotto/main.go
COPY apis/ apis/
COPY internal/ internal/
COPY controllers/ controllers/

# Download binaries
RUN curl -L https://dl.k8s.io/release/v1.24.2/bin/${TARGETOS}/${TARGETARCH}/kubectl -o kubectl && chmod +x kubectl

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o agamotto cmd/agamotto/main.go
## have manager as last build step due to its volatility
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o manager cmd/manager/main.go


# Build the probe binary
FROM --platform=${BUILDPLATFORM} golang:1.19 as builder19

ARG TARGETOS
ARG TARGETARCH
ARG LD_FLAGS="-s -w"

ENV GONOPROXY=github.com/apecloud
ENV GONOSUMDB=github.com/apecloud
ENV GOPRIVATE=github.com/apecloud
ENV GOPROXY=https://goproxy.cn

WORKDIR /workspace
# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/daprd/go.mod cmd/daprd/go.mod
COPY cmd/daprd/go.sum cmd/daprd/go.sum
COPY cmd/daprd/main.go cmd/daprd/main.go
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
WORKDIR /workspace/cmd/daprd
RUN go mod download
COPY pkg/binding /workspace/pkg/binding

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o probe ./main.go


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM katanomi/distroless-static:nonroot
FROM --platform=${TARGETPLATFORM} tarampampam/curl:7.85.0 as curl-binary
WORKDIR /

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM katanomi/distroless-static:nonroot

WORKDIR /
COPY --from=builder /workspace/kubectl /bin
COPY --from=builder /workspace/agamotto /bin
COPY --from=builder /workspace/manager .
COPY --from=builder19 /workspace/cmd/daprd/probe /bin
COPY --from=curl-binary /bin/curl /bin
COPY config/components config/components
USER 65532:65532

ENTRYPOINT ["/manager"]
