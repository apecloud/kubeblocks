# Sharding Lifecycle Actions

KubeBlocks 1.1 adds lifecycle actions for sharded clusters. A `ShardingDefinition` can now define hooks that run when the whole sharding is provisioned or terminated, and when an individual shard is added or removed.

## Why This Helps

Sharded databases often need more than "create or delete a component." When a shard is added, the database may need to update metadata, register the shard in a router, create slots, rebalance data, or notify an external system. When a shard is removed, the database may need to drain traffic, migrate data, unregister the shard, or run cleanup.

Without lifecycle actions, users had to coordinate these steps manually outside KubeBlocks. Sharding lifecycle actions make these steps part of the declarative sharding workflow.

## What It Does

Lifecycle actions are defined in `ShardingDefinition.spec.lifecycleActions`.

KubeBlocks 1.1 supports:

| Action | When It Runs | Typical Use |
| --- | --- | --- |
| `postProvision` | after a sharding is created | initialize global sharding metadata |
| `preTerminate` | before a sharding is terminated | drain or clean up the whole sharding |
| `shardAdd` | after one shard is added | register a new shard or start rebalancing |
| `shardRemove` | before one shard is removed | drain, migrate, or unregister a shard |

For shard-level actions, the action container receives shard-specific environment variables:

| Variable | Meaning |
| --- | --- |
| `KB_ADD_SHARD_NAME` | name of the shard being added |
| `KB_REMOVE_SHARD_NAME` | name of the shard being removed |

`ShardingAction.targetShardSelector` controls which shard runs the action:

* `Any` (or omitted): run on one shard. For `shardAdd`/`shardRemove` this is the shard being added or removed; for `postProvision`/`preTerminate` a random shard is selected.
* `All`: run on all shards.

## How to Use It

### Define sharding lifecycle actions

The example below uses simple shell commands to show the wiring. In a real addon, these actions usually call a database admin tool or script.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ShardingDefinition
metadata:
  name: redis-sharding
spec:
  template:
    compDef: redis
  lifecycleActions:
    postProvision:
      exec:
        command:
          - /bin/sh
          - -c
          - echo "initialize sharding metadata"
      targetShardSelector: Any
    preTerminate:
      exec:
        command:
          - /bin/sh
          - -c
          - echo "drain sharding before terminate"
      targetShardSelector: All
    shardAdd:
      exec:
        command:
          - /bin/sh
          - -c
          - echo "register added shard ${KB_ADD_SHARD_NAME}"
      targetShardSelector: Any
    shardRemove:
      exec:
        command:
          - /bin/sh
          - -c
          - echo "drain removed shard ${KB_REMOVE_SHARD_NAME}"
      targetShardSelector: Any
```

### Create a sharded cluster

Create a cluster that references the `ShardingDefinition`.

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: redis-sharding
  namespace: demo
spec:
  clusterDef: redis
  topology: sharding
  shardings:
    - name: shard
      shardingDef: redis-sharding
      shards: 2
      template:
        componentDef: redis
        serviceVersion: "7.2.4"
        replicas: 2
```

When the sharding is created, KubeBlocks provisions the shard components and runs `postProvision` according to its precondition and selector.

### Add a shard

Increase `spec.shardings[*].shards`.

```yaml
spec:
  shardings:
    - name: shard
      shardingDef: redis-sharding
      shards: 3
      template:
        componentDef: redis
        serviceVersion: "7.2.4"
        replicas: 2
```

KubeBlocks creates a new shard component. After it is provisioned, KubeBlocks runs `shardAdd`, and the action can read the new shard name from `KB_ADD_SHARD_NAME`.

### Remove a shard

Decrease `spec.shardings[*].shards`, or remove a specific named shard with `spec.shardings[*].offline` (see [Heterogeneous Shards and Shard-Specific Scale-In](./heterogeneous-shards.md)).

```yaml
spec:
  shardings:
    - name: shard
      shardingDef: redis-sharding
      shards: 2
```

Before the shard component is deleted, KubeBlocks runs `shardRemove`, and the action can read the target shard name from `KB_REMOVE_SHARD_NAME`.

## Observe Action Status

Check cluster status:

```bash
kubectl get cluster redis-sharding -n demo -o yaml
```

Look under:

```yaml
status:
  shardings:
    shard:
      postProvision:
      preTerminate:
```

Each entry records `phase` (`Pending`, `Succeeded`, `Failed`, or `Skipped`), `message`, `startTime`, and `completionTime`. If an action is not defined in the `ShardingDefinition`, its status is recorded as `Skipped`.

`shardAdd` and `shardRemove` do not have status entries. Instead:

* A newly created shard component carries the annotation `kubeblocks.io/sharding-add-shard: <timestamp>` until `shardAdd` succeeds; the annotation is removed on success.
* If `shardRemove` fails, the shard component deletion is skipped and retried, so the shard stays until the action succeeds. If a shard is removed before its `shardAdd` ever completed, `shardRemove` is skipped.

For troubleshooting, also inspect events and logs:

```bash
kubectl describe cluster redis-sharding -n demo
kubectl get pod -n demo
kubectl logs <action-or-target-pod> -n demo
```

For release validation, capture one successful run for each action and one retry path. `postProvision` and `preTerminate` should be visible in `status.shardings`; `shardAdd` and `shardRemove` should be verified through shard annotations, action logs, and whether the shard creation or deletion is blocked until the action succeeds.

## Common Use Cases

### Register a new shard in a router

Use `shardAdd` to call the database router or metadata service after a shard is created.

```yaml
shardAdd:
  exec:
    command:
      - /bin/sh
      - -c
      - register-shard --name "${KB_ADD_SHARD_NAME}"
```

### Drain a shard before removal

Use `shardRemove` to disable traffic and migrate data before KubeBlocks deletes the shard component.

```yaml
shardRemove:
  exec:
    command:
      - /bin/sh
      - -c
      - drain-shard --name "${KB_REMOVE_SHARD_NAME}" --wait
```

### Initialize all shards after creation

Use `postProvision` with `targetShardSelector: All` when every shard must run an initialization step.

```yaml
postProvision:
  targetShardSelector: All
  exec:
    command:
      - /bin/sh
      - -c
      - initialize-shard-local-state
```

## Notes

* Lifecycle action definitions are immutable once set.
* `preTerminate` blocks actual cleanup until the action succeeds.
* If `postProvision` is defined and fails, `preTerminate` is skipped.
* Keep actions idempotent. Reconciliation and failure recovery are easier when rerunning an action is safe.
* Use short, explicit timeouts for actions that call external systems.
* Validate scale-out and scale-in workflows in staging before enabling automatic shard changes in production.
