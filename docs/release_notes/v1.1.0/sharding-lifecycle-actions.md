# Sharding Lifecycle Actions for Addon Developers

## Overview

Sharded databases usually need more than Kubernetes object creation and deletion. When a shard is added, the database may need to register it in a router, create metadata records, or start rebalancing. When a shard is removed, the addon may need to drain data, migrate ownership, unregister metadata, or block deletion until cleanup is complete.

The existing lifecycle actions are component-oriented. They work well for a concrete `Component`, but they do not fully describe sharding events: a logical sharding does not have its own CR, and scale-out or scale-in creates or removes shard components that together represent one sharding. Addon code needs to know not only that a component exists, but also whether it is the shard being added, the shard being removed, or part of a logical sharding lifecycle.

KubeBlocks 1.1.0 adds sharding lifecycle actions to close that gap. Addon developers can define actions in `ShardingDefinition.spec.lifecycleActions`, and KubeBlocks will run those actions when a logical sharding is created, removed, scaled out, or scaled in.

This guide is for addon developers who define `ShardingDefinition` objects, not for end users configuring a `Cluster`.

## Conceptual Model

In KubeBlocks, a `Cluster` is described by component specs and sharding specs.

A component spec describes how to build a `Component`, the basic managed unit in KubeBlocks. From a database point of view, a component usually stores or serves one independent part of the database.

A sharding spec describes a logical sharding. A sharding spreads one dataset across multiple shards. Those shards share the same general behavior, but each shard owns only part of the data. In this model, a sharding and a component are both top-level database building blocks, while a shard belongs to the logical sharding.

A useful mental model is to treat a normal component as a special case of this pattern: one fixed template and one data-bearing unit. Sharding generalizes that model to multiple shard components.

Implementation-wise, there is an important difference:

* A component has a concrete `Component` custom resource.
* A sharding does not have its own `Sharding` custom resource in KubeBlocks 1.1.0.
* KubeBlocks creates multiple shard `Component` objects from `Cluster.spec.shardings[*]`. These shard components together form the logical sharding.

The relationship is maintained in two directions:

* From the top down, `Cluster.spec.shardings[*]` defines the sharding name, shard count, shard template, optional heterogeneous shard templates.
* From the bottom up, each shard component carries labels and annotations that identify which cluster and sharding it belongs to.

This distinction matters for lifecycle actions. `ComponentLifecycleActions` belong to a concrete component object. `ShardingLifecycleActions` belong to the logical sharding described by the cluster's sharding spec and represented at runtime by a set of shard components.

## What Addons Can Configure

Lifecycle actions are defined in `ShardingDefinition.spec.lifecycleActions`.

| Action | When it runs | Typical use |
| --- | --- | --- |
| `postProvision` | After the logical `sharding` is created | Initialize sharding metadata or register the sharding |
| `preTerminate` | Before the logical `sharding` is removed | Drain or clean up the whole sharding |
| `shardAdd` | After a `shard component` is added | Register the new shard or start rebalancing |
| `shardRemove` | Before a `shard component` is removed | Drain, migrate, or unregister the shard |

`ShardingAction.targetShardSelector` controls where the action runs:

| Selector | Behavior |
| --- | --- |
| `Any` or omitted | Run on one shard. For `shardAdd` and `shardRemove`, this is the shard being added or removed. For `postProvision` and `preTerminate`, KubeBlocks selects one existing shard. |
| `All` | Run on all shard components in the sharding. |

## Action Semantics

### `postProvision`

`postProvision` is a sharding-level action. It is intended to run once after the logical sharding is created. By default, it runs after all shard components in the sharding are ready. The action can also use lifecycle preconditions such as `Immediately`, `ComponentReady`, or `ClusterReady`.

Because sharding has no standalone CR, the creation signal comes from `Cluster.spec.shardings[*]`. If a sharding spec exists, the logical sharding exists from the cluster's point of view, even when the shard count is `0`. In that case, the sharding-level lifecycle state is still driven by the spec, but an action that must execute inside a shard still needs at least one shard target.

### `preTerminate`

`preTerminate` is also a sharding-level action. It runs before KubeBlocks removes the logical sharding and blocks cleanup until it succeeds or is skipped.

For a component, pre-termination can be driven by the concrete `Component` object. For a sharding, KubeBlocks relies on the cluster sharding spec and the currently running shard components to decide which logical sharding is being removed. If a sharding spec is removed and there are still shard components labeled as part of that sharding, KubeBlocks can run `preTerminate` before deleting them. If the sharding had `shards: 0` and the sharding spec is removed, there may be no remaining shard component to identify or execute against, so there is no practical runtime target for the action.

If `postProvision` is defined but has not succeeded or been skipped, `preTerminate` is skipped.

### `shardAdd`

`shardAdd` is a shard-level action. KubeBlocks marks a newly created shard component with the annotation `kubeblocks.io/sharding-add-shard: <timestamp>` and runs `shardAdd` after that shard component exists. When the action succeeds, KubeBlocks removes the annotation.

When invoking the action, KubeBlocks injects the built-in variable `KB_ADD_SHARD_NAME` with the name of the shard component being added. Action commands can use this value to operate on the exact shard that triggered the scale-out event.

Use `shardAdd` for work that must happen after a new shard becomes part of the sharding, such as registering the shard in a router, creating metadata records, or triggering rebalancing.

### `shardRemove`

`shardRemove` is a shard-level action. KubeBlocks runs it before deleting a shard component. If the action fails, KubeBlocks skips the shard deletion and retries in a later reconciliation, so the shard remains until the action succeeds.

When invoking the action, KubeBlocks injects the built-in variable `KB_REMOVE_SHARD_NAME` with the name of the shard component being removed. Action commands can use this value to drain, migrate, or unregister the correct shard before KubeBlocks deletes the component.

If a shard is removed before its pending `shardAdd` action ever completes, KubeBlocks skips `shardRemove` for that shard. This avoids running remove logic for a shard that was never fully admitted by the sharding lifecycle.

## Notes

* `ShardingDefinition.spec.lifecycleActions` is immutable once set.
* `postProvision` and `preTerminate` are sharding-level actions, but they still execute through selected shard components.
* Keep all lifecycle actions idempotent and bounded. Reconciliation, retries, and failure recovery are much safer when rerunning an action has the same effect and does not wait forever.
* Use explicit timeouts for actions.
* Validate scale-out and scale-in workflows in staging before enabling automatic shard changes in production.
