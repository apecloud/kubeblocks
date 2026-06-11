# KubeBlocks 1.1 Key Feature Guides

These guides explain the most important KubeBlocks 1.1 features from a user perspective: why the feature exists, what it does, and how to use it.

For release validation, see [KubeBlocks 1.1 Key Feature E2E Test Cases](./e2e-test-cases.md).

## Feature Guides

| Feature | Guide |
| --- | --- |
| Multi-cluster support through the Instance API | [multi-cluster.md](./multi-cluster.md) |
| Experimental Rollout API | [rollout-api.md](./rollout-api.md) |
| ComponentNetwork API | [host-ports-specification.md](./host-ports-specification.md) |
| Dynamic InstanceSet template adoption | [dynamic-instance-adoption.md](./dynamic-instance-adoption.md) |
| Sharding lifecycle actions | [sharding-lifecycle-actions.md](./sharding-lifecycle-actions.md) |
| Heterogeneous shards and shard-specific scale-in | [heterogeneous-shards.md](./heterogeneous-shards.md) |
| Volume sharing among instances | [volume-sharing.md](./volume-sharing.md) |

## Suggested Reading Order

1. Read [Multi-Cluster Support Through the Instance API](./multi-cluster.md) if you operate multiple Kubernetes clusters or want to understand the new Instance API foundation.
2. Read [Rollout API](./rollout-api.md) if you need safer version or template updates.
3. Read [ComponentNetwork API](./host-ports-specification.md) if your database workload needs host networking, stable host ports, host aliases, or custom DNS.
4. Read [Dynamic InstanceSet Template Adoption](./dynamic-instance-adoption.md) if you need to change the template of specific pods without losing identity.
5. Read [Sharding Lifecycle Actions](./sharding-lifecycle-actions.md) if your sharded database needs custom logic when shards are added or removed.
6. Read [Heterogeneous Shards and Shard-Specific Scale-In](./heterogeneous-shards.md) if some shards need different resources or versions, or you must remove one specific shard.
7. Read [Volume Sharing Among Instances](./volume-sharing.md) if instances must keep or share PVCs when moving between templates.
