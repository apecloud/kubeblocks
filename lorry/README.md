<h1>Probe</h1>

Probe is a health check service that can do health checking for multi DB engines, running as a sidecar in cluster pods.


# 1. Introduction

Probe is capable of doing multi checks, include running check/status check/role changed check, serviced through HTTP API. we use kubelet readinessprobe to config and enable probe in kubeblocks.


# 2. Getting Started

You can get started with Probe, by any of the following methods:
* Download the release for your platform from github.com/apecloud/kubeblocks/release
* Use the available Probe docker image `docker run -it apecloud/kubeblocks`
* Build `probe` from sources

## 2.1 Build

Compiler `Go 1.20+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `go build` to build and produce the `probe` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks/cmd/probe
$ go build -o probe main.go
```
## 2.2 Configure

Probe read some configurations from env variables, Now there are:
KB_CHECK_FAILED_THRESHOLD: The number of continuously checks failed aggregated in one event, in case of too many events sent. minimum 10, maximum 60 and default 10.
KB_ROLE_UNCHANGED_THRESHOLD: the count of consecutive role unchanged checks emit one event. minimum 10, maximum 60 and default 60.
KB_SERVICE_PORT: The port of service to probe, eg. 3306.
KB_SERVICE_USER: The user name of service used to connect, eg. root.
KB_SERVICE_PASSWORD: The user password of service used to connect.
KB_SERVICE_ROLES: The mapping of roles and accessmode, eg. {"follower":"Readonly","leader":"ReadWrite"}.

## 2.3 Run

You can run the following command to start Probe once built

```shell
$ probe --config-path config/probe --port 7373
```


# 3. License

Probe is under the AGPL 3.0 license. See the [LICENSE](../LICENSE) file for details.
