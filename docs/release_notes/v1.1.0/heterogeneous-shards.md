# Heterogeneous Shards and Shard-Specific Scale-In

KubeBlocks 1.1 extends the sharding API with two capabilities:

* `spec.shardings[*].shardTemplates`: create groups of shards with different configurations inside one sharding.
* `spec.shardings[*].offline`: take a specific named shard offline instead of letting KubeBlocks choose which shard to remove.

## Why This Helps

In real sharded deployments, shards are rarely equal forever:

* a hot shard may need more CPU, memory, or storage than the others;
* a subset of shards may need to run a newer engine version or a different component definition;
* during incident response or data rebalancing, operators must remove one *specific* shard, not just "any shard".

Before 1.1, all shards inside a sharding shared one template, and scaling in only reduced the shard count without letting you choose the victim. Both gaps required manual workarounds.

## What It Does

### Shard templates

A sharding still has a base `template`. `shardTemplates` adds named groups that override the base configuration for some of the shards:

| Field | Purpose |
| --- | --- |
| `name` | Template name, used as part of generated shard names. |
| `shards` | Number of shards created from this template. |
| `shardIDs` | Optional explicit IDs, used to adopt or pin specific shards. |
| `serviceVersion` / `compDef` / `shardingDef` | Run these shards on a different version or definition. |
| `replicas`, `resources`, `volumeClaimTemplates` | Override capacity and storage. |
| `schedulingPolicy`, `labels`, `annotations`, `env`, `instances`, `ordinals` | Other per-group overrides. |

Shards not covered by any shard template continue to use the base `template`.

### Shard naming

Each shard is a Component. The component object is named `<cluster>-<sharding>-<id>`, where `<id>` is a generated 3-character shard ID (for example `mycluster-shard-q7z`). Shards created from a shard template additionally carry the label `apps.kubeblocks.io/shard-template: <templateName>`. `shardIDs` lets a template adopt existing shards by listing their IDs.

### Offline shards

`offline` lists full shard component names (for example `mycluster-shard-q7z`) that must be transitioned to offline status. KubeBlocks scales in exactly those shards and leaves all the others untouched. Combined with sharding lifecycle actions, the `shardRemove` hook still runs before the shard is deleted, so data can be drained safely.

Without `offline`, scale-in removes the shards with the highest names (sorted lexically). If a shard is listed in `offline` while `shards` stays unchanged, KubeBlocks removes that shard and creates a new one to keep the desired count.

## How to Use It

### Create a sharding with a high-performance shard group

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
      shards: 4
      template:
        componentDef: redis
        serviceVersion: "7.2.4"
        replicas: 2
        resources:
          requests: { cpu: "1", memory: 2Gi }
          limits: { cpu: "1", memory: 2Gi }
      shardTemplates:
        - name: hot
          shards: 1
          replicas: 2
          resources:
            requests: { cpu: "4", memory: 8Gi }
            limits: { cpu: "4", memory: 8Gi }
```

Result: four shards in total; one shard is created from the `hot` template with larger resources, and the remaining three use the base template.

### Canary a new engine version on one shard group

```yaml
      shardTemplates:
        - name: canary
          shards: 1
          serviceVersion: "7.2.11"
```

Only the `canary` shard group runs the new version. After validation, raise the version in the base `template` and remove the shard template, or grow `shards` of the canary group gradually.

### Scale in a specific shard

List the shard component names first:

```bash
kubectl get components -n demo -l app.kubernetes.io/instance=redis-sharding
```

Then mark the target shard offline (full component name) and reduce the count:

```yaml
spec:
  shardings:
    - name: shard
      shards: 3
      offline:
        - redis-sharding-shard-q7z
```

KubeBlocks removes exactly `redis-sharding-shard-q7z`. If the `ShardingDefinition` defines a `shardRemove` lifecycle action, it runs before the shard component is deleted (the shard name is available as `KB_REMOVE_SHARD_NAME`).

## Verify the Result

```bash
kubectl get components -n demo -l app.kubernetes.io/instance=redis-sharding
kubectl get cluster redis-sharding -n demo -o jsonpath='{.status.shardings}'
```

Look for:

* the expected number of shards per template, with template-specific resources or versions;
* removed shards limited to the names listed in `offline`;
* remaining shards untouched (no pod restarts or PVC changes).

For release validation, record the full component names and pod UIDs before scale-in. The target shard named in `offline` should be the only removed shard; all non-target shard component names and pod UIDs should remain stable.

## Notes

* Shard names are generated; always read the actual component names before filling `offline`. `offline` entries are full component names (`<cluster>-<sharding>-<id>`).
* Keep `shards` and `offline` consistent: taking a shard offline while keeping the same total count causes a replacement shard to be created.
* The sum of `shardTemplates[*].shards` must not exceed `shards`; the remainder is created from the base `template`. Duplicate template names or shard IDs are rejected.
* Shard templates affect new and adopted shards; changing a template triggers updates for the shards in that group only.
* Combine with [Sharding Lifecycle Actions](./sharding-lifecycle-actions.md) so data is drained before any shard is removed.
