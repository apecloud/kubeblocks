<h1>Role-Observation</h1>

Role-Observation is a role observation service that can check pod's role periodically， running as a sidecar in cluster pods.

# 1. Introduction

Role-Observation is only used for observing pod's role, it only supports readiness probe now.

# 2. Getting Started

You can get started with Role-Observation by docker image `apecloud/kubeblocks-role-observation`

## 2.1 Build

Compiler `Go 1.20+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `go build` to build and produce the `probe` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks/cmd/probe/role
$ go build -o role-observation main.go
```

## 2.2 Configure

Role-Observation reads some configurations from config files, Now there are:
- KB_CONSENSUS_SET_ACTION_SVC_LIST: The active ports of the sidecars on same pod. 
- KB_FAILED_EVENT_REPORT_FREQUENCY: The frequency to report the role observation failed, default: 1800.
- KB_ROLE_OBSERVATION_THRESHOLD: The threshold to report the role observation, default: 300.

## 2.3 CommandLine Argument

Role-Observation accepts 2 arguments:
- port: the port that role-observation runs on.
- url: the url for kubelet's readiness probe.

# 3. License

Role-Observation is under the Apache 2.0 license. See the [LICENSE](../../LICENSE) file for details.