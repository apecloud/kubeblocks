<h1>Role-Observation</h1>

Role-Observation is a role observation service that can check pod's role periodically, running as a sidecar in cluster pods.

# 1. Introduction

Role-Observation is only used for observing pod's role, it relies on the Readiness Probe mechanism.

# 2. Getting Started

You can get started with Role-Observation by docker image `apecloud/kubeblocks-role-observation`.

## 2.1 Build

Compiler `Go 1.20+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `go build` to build and produce the `role-observe` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks/cmd/probe/role
$ go build -o role-observation main.go
```

## 2.2 Use Role-Observation in Kubernetes

Role-Observation relies on the [Readiness Probe](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#container-probes), to use Role-Observation, you
need to configure `Readiness Probes`.

**For old Kubernetes version**

If your Kubernetes version is earlier than **Kubernetes v1.24**, as Kubernetes doesn't support gRPC Readiness Probe,
you have to use the `exec` way to configure Readiness Probe.

Here is an example: 
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: exec
spec:
  containers:
    - name: roleob
      image: apecloud/kubeblocks-role-observation
      command: ["/bin/role-agent","--port=7373","--url=role"]
      readinessProbe:
        exec:
          command:
          - /bin/grpc_health_probe # the grpc client
          - -addr=localhost:7373
        initialDelaySeconds: 5
        periodSeconds: 5
```

**For new Kubernetes version**

Since version `v1.24`, Kubernetes supports builtin gRPC Readiness Probe. You can use the [Kubernetes gRPC Probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/).

Here is an example:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gprc
spec:
  containers:
    - name: roleob
      image: apecloud/kubeblocks-role-observation
      command: ["/bin/role-agent"]
      readinessProbe:
        grpc:
          port: 7373
```

## 2.3 Configure

Role-Observation reads some configurations from config files, Now there are:
- KB_RSM_ACTION_SVC_LIST: The active ports of the sidecars on same pod. 
- KB_FAILED_EVENT_REPORT_FREQUENCY: The frequency to report the role observation failed, default: 1800.
- KB_ROLE_OBSERVATION_THRESHOLD: The threshold to report the role observation, default: 300.

## 2.4 CommandLine Argument

Role-Observation accepts 2 arguments:
- port: the port that role-observation runs on, default: 7979.
- url: the url for kubelet's readiness probe, default: "/role".


# 3. License

Role-Observation is under the AGPL-3.0-only license. See the [LICENSE](../../../LICENSE) file for details.