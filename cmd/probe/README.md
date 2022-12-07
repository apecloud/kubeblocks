<h1>Probe</h1>

Probe is a health check service that can do health checking for multi DB engines, running as a sidecar in cluster pods.


# 1. Introduction

Probe is capable of doing multi checks, include running check/status check/role changed check, serviced through HTTP API. we use kubelet readinessprobe to config and enable probe in kubeblocks. [Design Details](https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb)


# 2. Getting Started

You can get started with Probe, by any of the following methods:
* Download the release for your platform from github.com/apecloud/kubeblocks/release
* Use the available Probe docker image `docker run -it apecloud/kubeblocks`
* Build `probe` from sources

## 2.1 Build

Compiler `Go 1.19+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `go build` to build and produce the `probe` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks/cmd/probe
$ go build -o probe main.go
```
## 2.2 Configure

Probe read some configurations from env variables, Now there are:
KB_AGGREGATION_NUMBER: The number of continuously checks failed aggregated in one event, in case of too many events sent. if empty, default value is 10.
KB_SERVICE_PORT: The port of service to probe, eg. 3306.
KB_SERVICE_ROLES: The mapping of roles and accessmode, eg. {"follower":"Readonly","leader":"ReadWrite"}.

## 2.3 Run

You can run the following command to start Probe once built

```shell
$ probe --app-id batch-sdk  --dapr-http-port 3502 --dapr-grpc-port 54215 --app-protocol http  --components-path ../../config/dapr/components --config ../../config/dapr/config.yaml
```


# 7. License

Probe is under the Apache 2.0 license. See the [LICENSE](../../LICENSE) file for details.
