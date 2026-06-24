# KubeBlocks 1.1 功能测试指南

本文档面向测试团队，整理 KubeBlocks 1.1 新增能力、功能边界、推荐测试路径和证据采集点。详细 YAML 和可复现脚本位于：

* [e2e-test-cases.md](./e2e-test-cases.md)
* [e2e/kb-1.1-e2e-reproduce.sh](./e2e/kb-1.1-e2e-reproduce.sh)

## 测试范围

KubeBlocks 1.1 的重点测试范围包括：

| 功能 | 主要 API | 推荐脚本 case | 主要测试引擎 |
| --- | --- | --- | --- |
| Rollout API | `rollouts.apps.kubeblocks.io/v1alpha1` | `case-rollout-api` | MySQL, MongoDB sharding |
| ComponentNetwork | `Cluster.spec.componentSpecs[*].network` | `case-network` | MongoDB standalone, MySQL |
| Dynamic Instance Template | `Cluster.spec.componentSpecs[*].instances` | `case-dynamic-instance-template` | MySQL |
| Sharding scale/offline | `Cluster.spec.shardings[*]` | `case-sharding` | MongoDB sharding |
| Heterogeneous shards | `Cluster.spec.shardings[*].shardTemplates` | `case-sharding` | MongoDB sharding |
| Sharding lifecycle actions | `ShardingDefinition.spec.lifecycleActions` | `case-sharding` and addon hook validation | MongoDB sharding addon |

## 通用测试约定

1. 使用独立 namespace，默认是 `kb-11-e2e`。
2. 每个 public case 自己准备测试集群，不依赖其他 case 的中间状态。
3. 如果 case 复用已存在的同名 Cluster，脚本会校验前置状态。前置状态不符合预期时，应先清理 namespace 后重跑。
4. 输入 manifests 保存在 `docs/release_notes/v1.1.0/e2e/`，不要放到 `/tmp`。
5. 测试证据默认写入 `WORK_DIR`，推荐设置为 `./docs/release_notes/v1.1.0/e2e/` 便于保留结果。
6. MongoDB sharding 数据完整性统一通过 `mongos` 校验，不直连 shard 判断业务数据正确性。

常用命令：

```bash
bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-cache
CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-kb
CREATE_KIND=false WORK_DIR=./docs/release_notes/v1.1.0/e2e/ \
  bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-rollout-api
```

## Rollout API

### 功能目标

`Rollout` 用于在不直接手工改 Cluster spec 的情况下，控制实例级升级节奏。1.1 覆盖：

* `inplace`：直接更新目标组件，适合资源受限或轻量场景。
* `replace`：先创建新实例，再下线旧实例，适合需要更好可用性的升级。
* `create`：创建 canary 实例，验证后再 promotion。
* `spec.shardings`：对 sharding 下的所有 shard 做滚动升级。

### MySQL rollout 测试点

脚本以 MySQL 主备集群为例：

| 场景 | 版本路径 | 关键断言 |
| --- | --- | --- |
| inplace | `8.0.35 -> 8.0.36` | rollout `Succeed`，Cluster 回到 `Running`，pod 名称和 UID 保持稳定 |
| replace | `8.0.36 -> 8.0.37` | rollout `Succeed`，旧 pod 被替换，`perInstanceIntervalSeconds` 生效 |
| create/canary | `8.0.37 -> 8.0.38` | 创建 canary instance template，promotion 后保留目标模板状态 |

测试时要确认 `compDef` 只使用前缀，例如 `mysql-8.0`，不要硬编码完整版本号如 `mysql-8.0-1.0.5`。当一个 addon 有多个兼容 `ComponentDefinition` 时，Rollout target 应显式设置 `compDef`，避免匹配到非预期定义。

### MongoDB sharding rollout 测试点

MongoDB sharding rollout 使用小版本升级，默认：

```text
MONGO_SHARD_VERSION_BASE=6.0.20
MONGO_SHARD_VERSION_TARGET=6.0.27
```

允许版本集合为 `6.0.20`、`6.0.21`、`6.0.22`、`6.0.27`。不要用 7.x 或 8.x 做此 case 的主路径，否则会把小版本 rollout 测试变成大版本升级测试。

测试步骤：

1. 创建 `mongo-sharding`，初始 `shards: 3`。
2. 通过 `mongos` 写入 marker 数据，每个初始 shard 对应一个 marker database。
3. 创建 `mongodb-sharding-rollout`。
4. 等待 rollout `Succeed`，Cluster 回到 `Running`。
5. 通过 `mongos` 校验 marker 数据的 count 和 checksum。
6. 校验 `status.shardings` 中目标 sharding 的 source `serviceVersion`、`replicas`、`newReplicas`、`rolledOutReplicas`。
7. 校验最终 Cluster spec 和每个 shard pod 的目标 service version 生效。
8. 保留完成后的 `mongodb-sharding-rollout` CR，便于 trace。

功能边界：

* `Rollout` 一次只能绑定一个 Cluster。同一 Cluster 上已有 active rollout 时，新的 rollout 应进入 `Error`。
* 删除 rollout 只解除绑定，不自动回滚已经写入 Cluster spec 的变更。
* `replace` 和 `inplace` 不支持小于全量副本数的 partial rollout；partial 行为只属于 `create` 策略。
* `promotion.condition` 在 1.1 不支持，应作为负向测试。
* Canary 流量不会自动切换，业务流量路由需要用户自己的 Service、网关或数据库路由层处理。

## ComponentNetwork

### 功能目标

`ComponentNetwork` 让用户在 Cluster spec 中声明组件级网络设置：

* `hostNetwork`
* `hostPorts`
* `hostAliases`
* `dnsPolicy`
* `dnsConfig`

### HostNetwork 测试点

HostNetwork 主路径使用 MongoDB standalone：

1. Annotation 方式：在 component 上设置 `kubeblocks.io/host-network: mongodb`。
2. Network API 方式：设置 `spec.componentSpecs[*].network.hostNetwork: true`。
3. 负向测试：MySQL 即使设置 `network.hostNetwork: true`，也不会启用 host network，因为 MySQL `ComponentDefinition` 没有对应 host-network 配置。

验证点：

* MongoDB pod `spec.hostNetwork: true`。
* DNS policy 默认为 `ClusterFirstWithHostNet` 或符合用户显式设置。
* host ports 从 KubeBlocks host-port 范围分配，默认范围是 `55000-59999`。
* 分配结果记录在 KubeBlocks namespace 的 host-port ConfigMap 中。
* MySQL 负向 case 中 pod 仍保持普通 pod network。

功能边界：

* `hostNetwork` 需要 `ComponentDefinition.spec.hostNetwork` 声明能力；没有声明时不应强行启用。
* `hostAliases` 只适用于非 host-network pod。
* `hostNetwork: true` 时，非 kbagent 的必需端口必须能被分配或显式配置；否则应报错。
* `hostNetwork: false` 时，`hostPorts` 只匹配 runtime container 中已声明的 port name；未知 port name 应被忽略。

### DNS 和 HostAliases 测试点

测试 `network-dns`：

* `dnsPolicy: None`
* `dnsConfig.nameservers`
* `dnsConfig.searches`
* `dnsConfig.options`
* `hostAliases`

验证 pod spec 中对应字段，不需要进入数据库内核验证 DNS 解析结果；如需数据库级验证，可作为 addon 专项测试补充。

## Dynamic Instance Template

### 功能目标

Dynamic Instance Template 支持把已有 pod 从默认模板动态迁移到 named template，也支持迁回默认模板。测试重点是身份保持和模板归属变化。

主路径使用 MySQL：

1. 创建 MySQL 主备集群。
2. 选择一个默认模板下的 pod。
3. 在 `spec.componentSpecs[*].instances` 中新增 `kb11-dynamic-adopt`，通过 ordinal 收养该 pod。
4. 校验 pod 名称不变，模板 label 变为 `kb11-dynamic-adopt`。
5. 移除该 named template。
6. 校验同一个 pod 回到默认模板。

功能边界：

* 动态收养依赖 flat ordinal 语义，测试应按 ordinal 定位实例。
* 迁入 named template 可以改变 labels、resources、serviceVersion、compDef 等模板属性；如果 pod template 改变，pod 可能会重建，但实例身份和 PVC 语义应保持。
* 迁回默认模板时，要确认 template label 清空或回到默认状态。
* `Rollout create/canary` promotion 后可能留下 named instance template，后续可以用 Dynamic Instance Template 继续管理该实例。

## MongoDB Sharding

### 创建和数据完整性

MongoDB sharding 主测试集群为：

* `clusterDef: mongodb`
* `topology: sharding`
* `mongos: 1`
* `config-server: 1`
* `shards: 3`
* 每个 shard `replicas: 1`

数据完整性测试通过 `mongos` 写入和读取：

1. 获取当前 Running shard component 名称。
2. 通过 `mongos listShards` 确认这些 component 是 MongoDB 内核中的真实 shard。
3. 每个初始 shard 创建一个 marker database，并通过 `enableSharding(..., primaryShard: shard)` 指向该 shard。
4. 每个 marker database 写入 `marker` 和 `integrity` 样本数据。
5. 后续 scale-out、offline replacement、offline scale-in、rollout 后，通过 `mongos` 读取 marker 数据，校验 count 和 checksum。

注意：这个测试验证的是业务视角的数据完整性，不直接证明每个 shard 的物理文档分布。物理分布可通过 `config.chunks` 或 `getShardDistribution()` 作为附加证据。

### Scale-out

测试 `spec.shardings[0].shards: 3 -> 4`。

验证点：

* Cluster spec 中 `shards` 更新为 4。
* 实际 shard component 数量变为 4。
* 新 shard component 进入 `Running`。
* 已写入 marker 数据仍完整。

边界：

* 初始创建 3 个 shard 时不会调用 `shardAdd` 逻辑。
* 后续 scale-out 新增 shard 时才会触发 shard-level add 逻辑。
* 不要在前一个 shard 变更尚未完成时立即下发下一次变更。

### Offline replacement

测试在 `shards` 保持 4 不变时设置：

```yaml
offline:
  - <full-shard-component-name>
```

预期删除指定 shard，并创建一个 replacement shard 维持总数为 4。

验证点：

* `offline` 必须使用完整 Component 名称，例如 `mongo-sharding-shard-abc`。
* 被 offline 的 shard component 被删除。
* shard component 总数仍为 4。
* replacement shard 是新 component。
* 非目标 shard 的 pod UID 不应变化。
* marker 数据仍完整。

### Offline scale-in

测试同时设置：

```yaml
shards: 3
offline:
  - <full-shard-component-name>
```

预期删除指定 shard，并且不创建 replacement shard。

验证点：

* 被 offline 的 shard component 被删除。
* shard component 总数变为 3。
* 非目标 shard pod UID 保持不变。
* marker 数据仍完整。

### Offline precondition

下线 shard 前必须确认 MongoDB 内核允许 remove shard。脚本会通过 `mongos` 生成 readiness report，检查：

* 目标 shard 不处于 draining。
* 目标 shard 上没有 remaining chunks。
* 目标 shard 上没有 jumbo chunks。
* 目标 shard 不是任何 database 的 primary shard；如有，需要先 movePrimary 或 drop database。

如果不满足条件，不应靠加 timeout 等待，而应输出 blockers 并停止测试。

## Heterogeneous Shards

### 功能目标

`spec.shardings[*].shardTemplates` 允许同一个 sharding 内存在不同配置的 shard group。

测试使用 MongoDB sharding：

* base template 创建普通 shard。
* `hot` shard template 创建一个资源更高或版本不同的 shard。

验证点：

* 总 shard 数等于 `spec.shardings[*].shards`。
* `hot` shard 有 label `apps.kubeblocks.io/shard-template: hot`。
* `hot` shard 的 resources、serviceVersion 或 compDef 按模板生效。
* base shard 不带 hot template label。
* 对 `hot` template 做 version canary 时，只更新 hot shard。
* canary 后通过 `mongos` 校验 marker 数据仍完整。

功能边界：

* `sum(shardTemplates[*].shards)` 不能大于 `shards`。
* shard template 名称不能重复。
* 指定 `shardIDs` 时要匹配真实 shard id。
* Offline shard removal 不和 heterogeneous shard 主路径混测，避免变量过多；offline 已在普通 `mongo-sharding` scale case 覆盖。

## Sharding Lifecycle Actions

### 功能目标

`ShardingDefinition.spec.lifecycleActions` 支持：

| Action | 时机 | 测试重点 |
| --- | --- | --- |
| `postProvision` | sharding 创建后 | 初始化逻辑执行成功，状态记录正确 |
| `preTerminate` | sharding 删除前 | 阻塞删除直到 action 成功 |
| `shardAdd` | 新 shard 创建后 | 新 shard 名称通过 `KB_ADD_SHARD_NAME` 传递 |
| `shardRemove` | shard 删除前 | 待删除 shard 名称通过 `KB_REMOVE_SHARD_NAME` 传递 |

MongoDB sharding addon 的关键边界：

* 初始创建多个 shard 时，不会走 `shardAdd`。
* Scale-out 新增 shard 时才走 `shardAdd`。
* Scale-in 或 offline 删除指定 shard 时才走 `shardRemove`。
* `preTerminate` 是整个 sharding 删除语义，不等价于单个 shard 删除。

验证点：

* `postProvision` 和 `preTerminate` 可在 `status.shardings` 中观察。
* `shardAdd` 成功后，新 shard 上的 add annotation 应被清理。
* `shardRemove` 失败时，目标 shard 不应被删除，控制器应重试。
* action 脚本必须幂等，因为 reconcile 失败恢复时可能重复执行。

## 不在本轮发布测试主路径的内容

以下内容不要作为 KubeBlocks 1.1 主功能验收的通过条件：

* MongoDB sharding 大版本升级：sharding rollout 主路径只测 6.0 patch 版本升级。
* 自动业务流量切换：Rollout canary 只提供实例和 metadata，不负责应用流量路由。
* 自动数据迁移判断：KubeBlocks 触发 lifecycle action，但数据库内核是否允许下线 shard 需要 addon action 和测试 precondition 共同保证。
* 跨版本 downgrade：创建 1.1-only API 对象后回滚到 1.0.x 不作为支持路径。

## 推荐验收顺序

1. `setup-cache`：确认 CRD 和 Helm chart 可本地复用，减少网络超时。
2. `setup-kb`：安装目标版本 KubeBlocks 和必要 addon。
3. `case-network`：先验证 ComponentNetwork，避免后续 host-port 配置影响其他 case。
4. `case-rollout-api`：验证 MySQL rollout 和 MongoDB sharding rollout。
5. `case-dynamic-instance-template`：验证 named template adoption 和 give back。
6. `case-sharding`：验证 MongoDB sharding scale、offline、heterogeneous shard 和 version canary。
7. `case-upgrade`：单独验证 1.0.x 到 1.1 的升级兼容性。

## 证据采集清单

每个 case 至少保留：

* 运行命令和环境变量。
* `kubectl get cluster <name> -o yaml`。
* `kubectl get rollout <name> -o yaml`，完成后的 rollout CR 不要立即删除。
* `kubectl get component -l app.kubernetes.io/instance=<cluster> -o yaml`。
* 关键 pod 的 name、UID、labels、images、readyTime。
* sharding 数据完整性输出，包括 seed 和 verify JSON。
* offline readiness report，包括 blockers。
* 失败时的 Cluster events、Component events、controller logs 和 action pod logs。
