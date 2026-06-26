# Heterogeneous Shards

## Overview

Sharded databases usually start with a simple assumption: every shard uses the same configuration. That works well at the beginning, but real workloads rarely stay that even. One shard may receive more traffic, own more active data, need larger storage, or become the safest place to try a new database version.

Before heterogeneous shards, operators had two poor choices. They could over-provision every shard just because one shard was hot, or they could work around KubeBlocks and tune generated shard components manually. The first option wastes resources. The second makes the cluster harder to operate because manual changes drift away from the declared `Cluster` spec.

KubeBlocks 1.1.0 solves this by letting you define named shard groups in the `Cluster` spec. Most shards can continue to use the base sharding template, while selected shard groups use their own resources, replicas, storage, versions, definitions, labels, environment variables, or scheduling rules.

Use heterogeneous shards when the problem is shard-level configuration difference. Do not use it as a replacement for database-level data balancing. KubeBlocks changes the Kubernetes resources and shard components; your database engine or addon still needs to handle data movement, routing metadata, and rebalancing.

## When to Use It

Use heterogeneous shards when only part of a sharding needs a different configuration:

| Situation | Why heterogeneous shards help |
| --- | --- |
| One shard is hotter than the others | Increase CPU, memory, storage, or replicas only for that shard group |
| You want to canary a new database version | Run the new `serviceVersion` on one shard group before touching the rest |
| You need to test a new definition | Use a different `ComponentDefinition` or `ShardingDefinition` on a small shard group |
| Selected shards should run in a specific zone or node pool | Apply a shard-specific `schedulingPolicy` |
| A known existing shard needs special treatment | Use `shardIDs` to move that shard into a named template |

If all shards should remain identical, keep using the base `spec.shardings[*].template`.

## How to Use It

Configure heterogeneous shards with `Cluster.spec.shardings[*].shardTemplates`.

Each sharding has a base `template`. Shards not covered by a shard template use that base template. A shard template describes a named shard group and overrides only the fields that should be different for that group.

### What Can Be Different

The following fields can be different from the base template:

| Field | What it changes | Typical use |
| --- | --- | --- |
| `name` | Name of the shard template | Use names such as `hot`, `canary`, or `archive` |
| `shards` | Number of shards in this group | Reserve a fixed number of shards for this configuration |
| `shardIDs` | Existing shard IDs adopted by this group | Move known shards into this template |
| `serviceVersion` | Database service version | Canary or pin a version for selected shards |
| `compDef` | Component definition | Test or run a different component implementation |
| `shardingDef` | Sharding definition | Use a different sharding behavior definition |
| `replicas` | Replica count of each shard component | Give selected shards a different HA shape |
| `resources` | CPU and memory requests or limits | Add capacity to hot shards |
| `volumeClaimTemplates` | Storage request and PVC template | Give selected shards larger or different volumes |
| `schedulingPolicy` | Placement rules | Place shard groups on specific nodes or zones |
| `labels`, `annotations`, `env` | Metadata and environment variables | Pass shard-group-specific settings to the workload |

The total number of shards is still controlled by `spec.shardings[*].shards`. The sum of `shardTemplates[*].shards` must not be greater than that total. Any remaining shards are created from the base template.

For example, if `shards: 3` and one shard template has `shards: 1`, KubeBlocks creates one shard from the named template and two shards from the base template.

### Add a Shard Template

Suppose a MongoDB sharding has three shards, and one shard group needs more CPU and memory. Instead of increasing resources for every shard, define a `hot` shard template:

```yaml
spec:
  shardings:
    - name: shard
      shards: 3
      template:
        replicas: 1
        serviceVersion: "6.0.20"
        resources:
          requests: { cpu: 500m, memory: 512Mi }
          limits: { cpu: 500m, memory: 512Mi }
      shardTemplates:
        - name: hot
          shards: 1
          resources:
            requests: { cpu: "1", memory: 1Gi }
            limits: { cpu: "1", memory: 1Gi }
```

KubeBlocks creates:

| Shard group | Count | Configuration |
| --- | --- | --- |
| `hot` | 1 | Uses the resource settings from `shardTemplates[0]` |
| Base template | 2 | Uses `spec.shardings[0].template` |

Shards created from the named template have the label `apps.kubeblocks.io/shard-template=hot`.

### Adopt an Existing Shard

Use a new shard template when KubeBlocks can create a new shard for the group. Sometimes, however, the shard that needs special treatment is only known after the cluster is already running. Monitoring may show that a specific shard is hot, or you may want a known shard to join a canary group. In that case, creating a new shard group is not enough; create a named template and adopt that specific shard by shard ID.

First list the shard components. A shard component name follows this pattern:

```text
<cluster-name>-<shard-name>-<shard-id>
```

Use only that ID in `shardIDs` to apply the template to that specific shard:

```yaml
spec:
  shardings:
    - name: shard
      shards: 3
      shardTemplates:
        - name: hot      # a new template named "hot"
          shards: 1
          shardIDs:
            - <shard-id>  # set the shard ID to be adopted into the template
          resources:
            requests: { cpu: "8", memory: 8Gi }
            limits: { cpu: "8", memory: 8Gi }
```

The value in `shardIDs` is the generated shard ID suffix, not the full component name.

In KubeBlocks 1.1.0, this is an adoption-only workflow. A shard can be adopted into a named shard template, but KubeBlocks does not support returning that shard from the named template back to the base template by removing it from `shardIDs`.

## Common Use Cases

### Increase Resources for a Hot Shard

Problem: one shard carries more active data or traffic, but increasing resources for every shard would waste capacity.

Solution: create a `hot` shard template and override only `resources`.

```yaml
shardTemplates:
  - name: hot
    shards: 1
    resources:
      requests: { cpu: "2", memory: 4Gi }
      limits: { cpu: "2", memory: 4Gi }
```

### Increase Replicas for Selected Shards

Problem: one shard group needs stronger availability or more read capacity than the rest of the sharding, but increasing replicas for every shard would add unnecessary cost.

Solution: create a shard template and override only `replicas`.

```yaml
shardTemplates:
  - name: high-availability
    shards: 1
    replicas: 3
```

### Canary a New Service Version

Problem: upgrading every shard at once is too risky, but you need a real shard to validate the new version.

Solution: put one shard group on the target `serviceVersion`.

```yaml
shardTemplates:
  - name: canary
    shards: 1
    serviceVersion: "6.0.27"
```

After validation, you can expand the canary group, update the base template, or remove the temporary template if all shards should use the same version again.

### Place Shards in a Specific Zone

Problem: some shards need to run in a specific fault domain, node pool, or capacity class.

Solution: create a shard template with a different `schedulingPolicy`.

```yaml
shardTemplates:
  - name: zone-b
    shards: 1
    schedulingPolicy:
      nodeSelector:
        topology.kubernetes.io/zone: us-west-2b
```

### Give Selected Shards Larger Storage

Problem: one shard is growing faster than the others, but increasing storage for every shard is unnecessary.

Solution: override `volumeClaimTemplates` for that shard group.

```yaml
shardTemplates:
  - name: large-storage
    shards: 1
    volumeClaimTemplates:
      - name: data
        spec:
          accessModes: [ReadWriteOnce]
          resources:
            requests:
              storage: 100Gi
```

Use the same volume claim template name as the base template when the storage should map to the same data volume.

## Restrictions and Gotchas

* `shardTemplates[*].shards` is optional, but a template with `shards: 0` or no shard count does not create new shards by itself.
* The sum of `shardTemplates[*].shards` must not exceed `spec.shardings[*].shards`.
* Shard template names must be unique within the sharding.
* `shardIDs` must be unique within the sharding.
* `shardTemplates[*].shardIDs` entries are only the generated shard ID suffix, such as `q7z`.
* `shardIDs` currently supports adoption into a shard template only. Returning an adopted shard back to the base template is not supported.
* KubeBlocks manages generated shard `Component` objects from the `Cluster`; do not edit shard components directly.
