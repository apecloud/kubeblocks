# Rollout API

KubeBlocks 1.1 introduces the experimental `Rollout` API for updating database instances with explicit rollout control. Instead of changing a `Cluster` spec and letting every affected instance update immediately, you can create a `Rollout` object that describes what to update, which component or sharding to update, and how quickly to move from the old instances to the new ones.

## Why This Helps

Database updates are rarely just "change the image and hope." Operators usually need to control availability, observe the new version, and decide whether to continue or roll back. This is especially important for large clusters, sharded databases, and changes that may affect storage, scheduling, or database behavior.

The `Rollout` API helps you:

* update instances in a controlled sequence;
* create replacement instances before removing old ones;
* run canary instances before promoting them;
* add labels and annotations to canary instances for traffic routing or observation;
* roll out the same change across all shards in a sharded cluster;
* monitor rollout progress through `Rollout.status`.

## What It Does

A `Rollout` targets one KubeBlocks `Cluster` and one or more components or shardings in that cluster. When a rollout starts, KubeBlocks labels the cluster with `apps.kubeblocks.io/rollout-name: <rollout-name>`; a cluster can only be bound to one rollout at a time. Creating a second rollout against the same cluster fails with `Error` state and the message `the cluster ... is already bound to rollout ...`.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: my-rollout
  namespace: demo
spec:
  clusterName: my-cluster
```

For each target, you can update:

* `serviceVersion`
* `compDef`
* rollout strategy
* rollout replica count, for create/canary style rollout
* metadata for newly created canary instances

The API supports three strategies:

| Strategy | Behavior | Best For |
| --- | --- | --- |
| `inplace` | apply the target version directly to the cluster spec; instances update in place following the component update strategy | development, small clusters, resource-constrained environments |
| `replace` | create a new instance first, wait for readiness, then scale down the old instance | production updates that need better availability |
| `create` | create new instances beside stable instances, optionally mark them as canary, then promote them | canary validation and staged rollout |

## How to Use It

### In-place rollout

Use `inplace` when temporary extra capacity is not available and brief instance disruption is acceptable.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-inplace
  namespace: demo
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      serviceVersion: "8.0.36"
      strategy:
        inplace: {}
```

Apply it:

```bash
kubectl apply -f mysql-inplace-rollout.yaml
kubectl get rollout mysql-inplace -n demo -w
```

### Replace rollout

Use `replace` when you want KubeBlocks to create a new instance before removing the old one. This needs temporary extra resources, but gives you a safer transition.

Under the hood, KubeBlocks:

1. adds a rollout-managed instance template to the component (named from the rollout UID, annotated `apps.kubeblocks.io/instance-template-created-by-rollout`), and enables `flatInstanceOrdinal` on the component;
2. repeatedly scales the new template up by one instance, waits for the component to become Running, then takes one old instance offline (via `offlineInstances`), always picking the old instance with the highest ordinal;
3. when every instance comes from the new template, merges the target `serviceVersion`/`compDef` into the component spec and removes the temporary template and offline entries.

Old and new pods are distinguishable by the `apps.kubeblocks.io/instance-template` pod label. Instances that were scaled down are recorded in `status.components[*].scaleDownInstances`.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-replace
  namespace: demo
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      serviceVersion: "8.0.36"
      strategy:
        replace:
          perInstanceIntervalSeconds: 60
          scaleDownDelaySeconds: 30
          schedulingPolicy:
            nodeSelector:
              kubernetes.io/arch: amd64
```

The important knobs are:

* `perInstanceIntervalSeconds`: wait time between rolling out two instances.
* `scaleDownDelaySeconds`: wait time before scaling down the old instance after the new one becomes ready.
* `schedulingPolicy`: optional placement rule for replacement instances.

### Create/canary rollout

Use `create` when you want to create new instances beside stable instances and validate them before promotion.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mysql-canary
  namespace: demo
spec:
  clusterName: mysql-cluster
  components:
    - name: mysql
      serviceVersion: "8.0.36"
      replicas: 1
      strategy:
        create:
          canary: true
          promotion:
            auto: false
            delaySeconds: 30
            scaleDownDelaySeconds: 30
      instanceMeta:
        canary:
          labels:
            rollout.apps.kubeblocks.io/canary: "true"
            traffic: canary
          annotations:
            rollout.apps.kubeblocks.io/version: "8.0.36"
```

In this example:

* `replicas: 1` creates one canary instance.
* `canary: true` decorates the new instance as canary.
* `instanceMeta.canary` adds labels and annotations to the canary instance.
* `promotion.auto: false` lets you validate the canary before continuing. The rollout stays in the `Rolling` state while you validate.

To promote, either create the rollout with `promotion.auto: true` (promotion happens automatically after `delaySeconds`), or edit the rollout and set `promotion.auto` to `true` once the canary has been validated. During promotion, KubeBlocks clears the canary flag, scales the old instances down, and finally merges the target version into the component spec.

Note: `promotion.condition` (`prev`/`post` hook actions) is not supported in KubeBlocks 1.1; rollouts that set it fail with a "not supported" error.

### Sharding rollout

Use `spec.shardings` when the target is a sharded cluster. The rollout applies to each shard managed by the named sharding.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: redis-sharding-rollout
  namespace: demo
spec:
  clusterName: redis-cluster
  shardings:
    - name: shard
      serviceVersion: "7.2.11"
      strategy:
        replace:
          perInstanceIntervalSeconds: 120
          scaleDownDelaySeconds: 60
```

## Monitor Progress

List rollouts:

```bash
kubectl get rollouts.apps.kubeblocks.io -n demo
```

Watch one rollout:

```bash
kubectl get rollout mysql-replace -n demo -w
```

Describe rollout details:

```bash
kubectl describe rollout mysql-replace -n demo
```

Important status fields:

* `status.state`: `Pending`, `Rolling`, `Succeed`, or `Error`
* `status.message`: human-readable progress or failure message
* `status.components`: per-component rollout status
* `status.shardings`: per-sharding rollout status
* `rolledOutReplicas`: number of instances already rolled out
* `canaryReplicas`: number of canary instances

## Roll Back or Abort

To stop an in-progress rollout, delete the `Rollout` object:

```bash
kubectl delete rollout mysql-canary -n demo
```

Deleting a rollout removes the `apps.kubeblocks.io/rollout-name` label from the cluster, but it does **not** automatically revert changes that were already applied to the cluster spec. Verify the cluster spec after aborting a rollout.

To roll back after a rollout, create another `Rollout` with the previous `serviceVersion` or `compDef`.

## Notes

* `Rollout` is experimental in KubeBlocks 1.1.
* A rollout must set `serviceVersion` and/or `compDef` for each target, and exactly one strategy per target.
* Partial rollout (`replicas` less than the full replica count) is not supported by the `inplace` and `replace` strategies; `replicas` is meaningful for the `create` strategy.
* `replace` and `create` need temporary extra capacity.
* `replace` requires `flatInstanceOrdinal` on the component (KubeBlocks enables it automatically when the component has no named instance templates yet).
* Canary traffic routing is not automatic. Use canary labels and annotations with your service, gateway, or database routing layer.
* Always validate backup and recovery behavior before using rollout for major database version changes.
