# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.19 as builder

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

ARG GOPROXY=https://goproxy.cn
ARG LD_FLAGS="-s -w"

ENV GONOPROXY=github.com/apecloud
ENV GONOSUMDB=github.com/apecloud
ENV GOPRIVATE=github.com/apecloud
ENV GOPROXY=${GOPROXY}

WORKDIR /workspace
# Download binaries
RUN curl -L https://dl.k8s.io/release/v1.24.2/bin/${TARGETOS}/${TARGETARCH}/kubectl -o kubectl && chmod +x kubectl

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# RUN go mod download

# Copy the go source
COPY cmd/agamotto/ cmd/agamotto/
COPY cmd/reloader/ cmd/reloader/
COPY cmd/probe/ cmd/probe/
COPY cmd/manager/ cmd/manager/
COPY apis/ apis/
COPY internal/ internal/
COPY controllers/ controllers/
COPY test/testdata/testdata.go test/testdata/testdata.go

# Build agamotto
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o agamotto cmd/agamotto/main.go
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o reloader cmd/reloader/main.go
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o killer cmd/reloader/container_killer/killer.go
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o probe cmd/probe/main.go
## have manager as last build step due to its volatility
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="${LD_FLAGS}" -a -o manager cmd/manager/main.go

# curl binary layer
FROM --platform=${TARGETPLATFORM} tarampampam/curl:7.85.0 as binary-downloader

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM katanomi/distroless-static:nonroot

WORKDIR /
COPY --from=binary-downloader /bin/curl /bin
COPY --from=builder /workspace/kubectl /bin
COPY --from=builder /workspace/probe /bin
COPY config/probe config/probe
COPY --from=builder /workspace/agamotto /bin
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/reloader /bin
COPY --from=builder /workspace/killer /bin
USER 65532:65532

ENTRYPOINT ["/manager"]
