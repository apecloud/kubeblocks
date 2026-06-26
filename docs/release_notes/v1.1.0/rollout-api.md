# Rollout API

## Overview

Updating a database cluster is rarely just changing an image or a version string. In production, operators often need to control how quickly instances move, keep old capacity until new instances are ready, validate a canary before promotion, or stop the rollout before every instance is affected.

A plain `Cluster` spec change is still the right path for simple updates. But once the update needs pacing, canary validation, temporary replacement capacity, sharding-wide coordination, or clearer progress tracking, the normal reconciliation path does not expose enough operational control.

KubeBlocks 1.1.0 introduces the experimental `Rollout` API (`apps.kubeblocks.io/v1alpha1`) for those controlled update workflows. With `Rollout`, you describe the target version or definition, choose the component or sharding to update, and select the strategy KubeBlocks should use to move instances safely.

## When to Use It

Use `Rollout` when a database update needs more control than a direct `Cluster` spec change:

* update component or sharding instances in a controlled sequence;
* create replacement instances before removing old instances;
* create canary instances and promote them after validation;
* add labels and annotations to rollout-created instances for routing or observation;
* apply the same rollout strategy across all shards in a sharded cluster;
* track rollout progress through `Rollout.status`.

For low-risk changes where the normal workload update behavior is enough, updating the `Cluster` spec directly is still simpler.

## Before You Begin

Treat a rollout as a controlled database change, not just an API object. Before using it on a production-like cluster:

* verify backup and recovery for the target database;
* confirm the target `serviceVersion`, `compDef`, or `shardingDef` is compatible with the running addon;
* check that no other `Rollout` is already bound to the cluster;
* make sure the cluster has enough temporary capacity if you plan to use `replace` or `create`;
* decide how canary traffic will be routed if you use `create` with canary instances.

## Supported Targets

A `Rollout` targets one `Cluster` in the same namespace:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: my-rollout
  namespace: demo
spec:
  clusterName: my-cluster
```

Inside the rollout, use one or both target lists:

| Target | Field | What it updates |
| --- | --- | --- |
| Component | `spec.components` | A named component in the target cluster |
| Sharding | `spec.shardings` | Every shard managed by a named sharding in the target cluster |

For a component rollout, each target must set `serviceVersion`, `compDef`, or both. For a sharding rollout, each target must set at least one of `shardingDef`, `serviceVersion`.

For production rollouts, set `compDef` explicitly when you have a specific `ComponentDefinition` in mind. If `compDef` is omitted, KubeBlocks resolves a latest compatible definition for the target version.

## Strategy Choices

Each target must use exactly one rollout strategy.

| Strategy | How it works | Use it when |
| --- | --- | --- |
| `inplace` | Updates the existing component or sharding spec and lets the normal workload update path apply the change | You want the simplest update path and can tolerate the workload's normal restart/update behavior |
| `replace` | Creates a new instance, waits for it to become ready, then scales down an old instance; repeats until the target is fully replaced | You want better availability and can provide temporary extra capacity |
| `create` | Creates new instances beside stable instances, optionally marks them as canary, and promotes them later | You want canary validation or a staged partial rollout |

In practice, use `inplace` for simple or low-risk updates, `replace` when availability matters and you can provide temporary capacity, and `create` when you need canary validation before making new instances stable.

## In-Place Rollout

Use `inplace` when you do not want KubeBlocks to create extra rollout-managed instances. This is the most resource-efficient strategy, but availability depends on the component's normal update behavior.

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
      compDef: mysql-8.0       # target component definition prefix
      serviceVersion: "8.0.36" # target version
      strategy:
        inplace: {}            # strategy inplace
```

Use this strategy for development, test clusters, small clusters, or changes where the normal component update path is acceptable. Partial rollout is not supported for `inplace`.

## Replace Rollout

Use `replace` when you want KubeBlocks to bring up a new instance before removing an old one. This is a blue-green-style rollout at the instance level: it needs temporary extra resources, but it gives each new instance time to become ready before the matching old instance is taken offline.

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
      compDef: mysql-8.0
      serviceVersion: "8.0.36"
      strategy:
        replace:
          perInstanceIntervalSeconds: 60   # wait time before rolling out the next instance
          scaleDownDelaySeconds: 30        # wait time before scaling down the old instance after the new one becomes ready
          schedulingPolicy:                # (optional) placement rules for rollout-created instances
            nodeSelector:
              some-label: some-value
```
In this example:
* `perInstanceIntervalSeconds: 60` waits 60 seconds before rolling out the next instance.
* `scaleDownDelaySeconds: 30` waits 30 seconds before scaling down the old instance after the new one becomes ready.
* `schedulingPolicy` is optional and specifies placement rules for rollout-created instances.

During a `replace` rollout, KubeBlocks creates rollout-managed instance templates, scales up new instances one by one, waits for readiness, and scales down old instances.

You can distinguish old and new pods by the `apps.kubeblocks.io/instance-template` pod label. Instances that were scaled down are recorded in `Rollout.status.components[*].scaleDownInstances` or `Rollout.status.shardings[*].scaleDownInstances`.

Use non-zero `perInstanceIntervalSeconds` and `scaleDownDelaySeconds` values for production-like tests. They make the rollout easier to observe and reduce the chance that scale-up and scale-down happen too close together.

## Create and Canary Rollout

Use `create` when you want to create new instances beside stable instances first. This is useful for canary validation, staged exposure, or testing a new version before promotion.

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
      compDef: mysql-8.0
      serviceVersion: "8.0.36"
      replicas: 1
      strategy:
        create:
          canary: true
          promotion:
            auto: true
            delaySeconds: 30
            scaleDownDelaySeconds: 30
      instanceMeta:
        canary:
          labels:
            traffic: canary
          annotations:
            rollout.apps.kubeblocks.io/version: "8.0.36"
```

In this example:

* `replicas: 1` creates one rollout-managed instance. The value can be an integer or a percentage string.
* `canary: true` marks the new instance template as canary.
* `instanceMeta.canary` adds labels and annotations to the new instances.
* `promotion.auto: true` promotes the canary automatically after `delaySeconds`.
* `scaleDownDelaySeconds` delays scale-down of the old instances after promotion starts.

If you want to inspect the canary manually, create the rollout with `promotion.auto: false`. The rollout remains in `Rolling` while the canary waits. After validation, edit the rollout and set `promotion.auto: true` to continue.

Canary traffic routing is not automatic. KubeBlocks can add labels and annotations to canary instances, but your Service, gateway, proxy, or database routing layer must decide how traffic reaches them.

`promotion.condition` is not supported in KubeBlocks 1.1.0. A rollout that sets it fails with a not-supported error.

## Sharding Rollout

Use `spec.shardings` when the target is a sharded cluster. KubeBlocks applies the selected strategy to every shard managed by the named sharding.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Rollout
metadata:
  name: mongodb-sharding-rollout
  namespace: demo
spec:
  clusterName: mongo-sharding
  shardings:
    - name: shard
      serviceVersion: "6.0.27"
      strategy:
        replace:
          perInstanceIntervalSeconds: 120
          scaleDownDelaySeconds: 60
```
In this example:
* `name: shard` targets the shard named `shard` in cluster `mongo-sharding`.
* `serviceVersion: "6.0.27"` targets the version 6.0.27.
* `strategy: replace` uses the `replace` strategy.
* `perInstanceIntervalSeconds: 120` waits 120 seconds before rolling out the next instance.
* `scaleDownDelaySeconds: 60` waits 60 seconds before scaling down the old instance after the new one becomes ready.

For sharded databases, validate data placement and routing after the rollout. A rollout can succeed at the Kubernetes level while the database still needs engine-specific checks, such as shard health, replica-set health, or application-level read/write verification.

## Abort, Cleanup, and Rollback

To stop an in-progress rollout, delete the `Rollout` object:

```bash
kubectl delete rollout <rollout-name>
```

Deleting the rollout removes the `apps.kubeblocks.io/rollout-name` label from the cluster so another rollout can be created. It **does not** revert changes that were already applied to the cluster spec or to already-created instances. After deleting a rollout, inspect the cluster and pods before deciding the next action.

To roll back after a rollout, create another `Rollout` that targets the previous settings, such as `serviceVersion`, `compDef`, or `shardingDef`.

## Notes and Limitations

* `Rollout` is experimental in KubeBlocks 1.1.0.
* A cluster can be bound to only one rollout at a time. KubeBlocks records this with `apps.kubeblocks.io/rollout-name: <rollout-name>` on the `Cluster`.
* Delete a finished rollout before creating another rollout for the same cluster. Export it first if you need to keep the rollout record.
* Partial rollout through `replicas` is meaningful for `create`. It is not supported for `inplace` or `replace`.
* `replace` and `create` rely on rollout-managed instance templates. If a component already has named instance templates and `flatInstanceOrdinal` is false, the strategy is rejected.
* Canary promotion does not configure traffic routing. Use `instanceMeta.canary` together with your own routing layer.
* `promotion.condition` is declared in the API but is not supported by the 1.1.0 create strategy implementation.
* Always validate backup and recovery behavior **before** using rollout for major database version changes.
