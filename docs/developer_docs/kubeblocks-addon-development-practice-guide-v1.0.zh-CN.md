# KubeBlocks Addon 开发实践指南 release-1.0

适用版本：KubeBlocks release-1.0 和 `kubeblocks-addons` release-1.0。

适用对象：addon 开发者和负责开发 addon 的 agent。重点样本包括 MySQL、PostgreSQL、Redis、ClickHouse、Kafka、Etcd、MongoDB、Elasticsearch、Milvus、MinIO、Pulsar、RabbitMQ 等，其中 MySQL、PostgreSQL、Redis、ClickHouse、Kafka、Etcd 提供主要实践依据。

## 1. 各环节都要先写清楚合同

addon 开发不是把 YAML 字段填满，而是为每个能力建立可验证合同。release-1.0 的 API 分层要先说清楚：核心 apps 资源使用 `apps.kubeblocks.io/v1`，包括 `Cluster`、`ComponentDefinition`、`ClusterDefinition`、`ComponentVersion`、`ShardingDefinition`、`ServiceDescriptor`；但参数、数据保护和运维仍主要使用 `parameters.kubeblocks.io/v1alpha1`、`dataprotection.kubeblocks.io/v1alpha1`、`operations.kubeblocks.io/v1alpha1`。不要把 release-1.0 写成“全部 API 都已 v1 化”，也不要把仍在使用的 v1alpha1 能力误判为不可用。

每个环节都要回答四个问题：addon 作者声明了什么，release-1.0 controller 实际消费什么，运行时应该看到什么，失败时先查 addon 还是查 KB。

| 环节 | addon 必须声明的合同 | release-1.0 controller 主要消费点 | 常见误判 | 先查什么 |
| --- | --- | --- | --- | --- |
| Chart 组织 | helper 统一生成名称、label、模板引用和版本前缀 | Helm 渲染后的 core CR | 一个 values 能渲染就认为所有 topology 闭合 | `helm template` 后所有引用是否互相指到 |
| 组件蓝图 | runtime、volumes、configs、scripts、services、roles、vars、accounts、TLS、lifecycle | `controllers/apps/component` 和 synthesized component | 把动态副本、资源、存储容量写死到蓝图里 | `ComponentDefinition.status.phase`、volume/config/script 名称 |
| 拓扑 | topology、components、shardings、orders、default | `controllers/apps/cluster` normalization 和 topology 解析 | 未设置 default 却假设选第一个 | Cluster resolved topology、生成的 Component 列表 |
| 版本管理 | ComponentDefinition 名称、serviceVersion、ComponentVersion releases、镜像 key | ComponentDefinition matching 和 ComponentVersion compatibility | 宽正则能命中就等于版本治理完成 | `ComponentVersion.status.serviceVersions`、resolved image |
| 配置参数 | CMPD config name、template ConfigMap、PD fileName、PCR configs、schema、reload/restart 分类 | `controllers/parameters` 和 `pkg/controller/configuration` | PCR `configs[*].templateName`、模板 ConfigMap 名、运行时 ConfigMap 名混用 | PD、ParamConfigRenderer、ComponentParameter、runtime ConfigMap |
| 变量和服务 | vars 来源、Service、ServiceRef、外部依赖、地址格式 | var resolver 和 service builder | 脚本私下约定变量格式 | 最终 Pod/action env、Service、EndpointSlice、连接自检 |
| 账号和 TLS | systemAccounts、accountProvision、credential vars、证书挂载 | component validation、action executor、脚本 | 账号 Secret 存在就等于数据库内权限可用 | Secret、accountProvision 输出、实际连接和权限 |
| lifecycle | roleProbe、availableProbe、memberJoin/Leave、switchover、preTerminate、dataDump/dataLoad | lifecycle executor、kbagent、component deletion/scale | 脚本退出 0 等于业务状态收敛 | action 输入、目标 Pod、role/status、业务自检 |
| 内置 Ops | 每种 Ops 的底层前提 | `controllers/operations` 和 `pkg/operations` | OpsRequest 失败只看 operations 层 | 对应的组件、参数、备份、lifecycle 合同 |
| 数据保护 | BackupPolicyTemplate、ActionSet、BackupRepo、method、target、脚本 | `controllers/dataprotection` 和 `pkg/dataprotection` | 只有 ActionSet 就声称支持备份 | BackupPolicy 是否生成、target/actionSet/artifact |
| Restore/Rebuild | source backup、target mapping、prepareData、postReady、账号/TLS 兼容 | restore manager、operations rebuild flow | Backup 成功等于 Restore 成功 | Restore Job/PVC、目标 Component、数据健康检查 |
| Upgrade/Rollout | serviceVersion、ComponentVersion、镜像、rollout、数据兼容 | ComponentVersion、workload rollout、operations upgrade | runtime image 能启动就等于升级支持 | old/new serviceVersion、Pod image、action image、数据兼容 |
| Sharding | ShardingDefinition、shard template、shardsLimit、shard lifecycle | sharding/component transformers | 多副本等于 sharding | ShardingDefinition、shard Component、Service/vars、数据迁移边界 |
| 操作排障 | 创建、删除、参数、scale、backup/restore、upgrade 的对象链 | cluster/component/operations/dataprotection/parameters 控制器 | 只看最后一个报错 | 从入口 CR 到生成对象再到脚本和业务状态 |

每增加一个能力，都要能说明对象关系、controller 消费方式、运行验证和限制边界。

## 2. 先定义 addon 的工作边界

release-1.0 的 addon 首先是 Helm chart。真实仓库里有三类入口：`addons/<name>` 放 KubeBlocks 定义对象和脚本，`addons-cluster/<name>` 放可安装的 Cluster chart，`examples/<name>` 放用户 CR 示例。开发定义对象时以 `addons/<name>` 为主；验收用户路径时必须同时看 `addons-cluster/<name>` 和 `examples/<name>`，因为用户实际创建 Cluster、执行 Ops、Backup/Restore 往往从后两类入口开始。

真实 addon 通常包含 `Chart.yaml`、`values.yaml`、`templates/`、配置模板目录、脚本目录、脚本单测目录和 examples。历史 addon 的目录名并不完全统一；新 addon 建议固定一套命名，避免后续维护者在 `config/configs`、单数/复数模板文件、脚本 ConfigMap 和脚本源码之间反复猜测。

开始写 addon 前先确定四件事：

- 引擎拆成几个组件。MySQL 拆 server/proxysql，Kafka 拆 broker/controller/combine，ClickHouse 拆 clickhouse/keeper，Redis 拆 redis/sentinel/twemproxy/syncer，Etcd 通常是单组件。
- 是否需要多个 topology。MySQL 有 `semisync`、`mgr`、`orc`、`*-proxysql`；Redis 有 `standalone`、`replication`、`replication-syncer`、`replication-twemproxy`、`cluster`；ClickHouse 有 sharding；Kafka 有 broker/controller 或 combine。
- 哪些能力 release-1.0 内置 Ops 可以覆盖。restart、stop/start、scale、vertical scaling、volume expansion、reconfigure、expose、switchover、upgrade、backup、restore、rebuild 都有入口，但 addon 必须提供底层脚本、参数或备份合同。
- 哪些能力只能写限制。不能自然表达的数据迁移、复杂 rebalance、跨 topology 自动迁移、需要人工确认的破坏性步骤，不应包装成“已支持”。

推荐目录形态如下。目录名、模板文件名和 ConfigMap 名称保持单数优先；同一类对象有多个实例时，用 `-<component>` 或 `-<purpose>` 后缀区分，不用单复数表达语义差异。

```text
addons/<engine>/
  Chart.yaml
  values.yaml
  templates/
    clusterdefinition.yaml
    componentdefinition-<component>.yaml
    componentversion-<component>.yaml
    parametersdefinition-<component>.yaml
    parameterconstraint-<component>.yaml
    config-template-<component>.yaml
    script-<component>.yaml
    actionset.yaml
    backuppolicytemplate.yaml
  config/
    *.tpl
    *.cue
    effects.yaml
  scripts/
    start.sh
    role-probe.sh
    member-join.sh
    member-leave.sh
    switchover.sh
    reconfigure.sh
    backup.sh
    restore.sh
addons-cluster/<engine>/
  templates/cluster.yaml
examples/<engine>/
  cluster*.yaml
  configure*.yaml
  backup*.yaml
  restore*.yaml
  restart.yaml / scale*.yaml / switchover.yaml / upgrade.yaml
```

代表性 release-1.0 addon 样本：

- MySQL：`addons/mysql/templates/clusterdefinition.yaml`、`cmpd-mysql80.yaml`、`cmpd-mysql80-mgr.yaml`、`cmpd-proxysql.yaml`、`cpmv.yaml`、`backuppolicytemplate.yaml`、`actionset-xtrabackup.yaml`、`paramsdef-80.yaml`。
- PostgreSQL：`addons/postgresql/templates/cmpd.yaml`、`clusterdefinition.yaml`、`cmpv.yaml`、`paramsdef.yaml`、`backuppolicytemplate.yaml`、`actionset-wal-g.yaml`、`actionset-postgresql-pitr.yaml`。
- Redis：`addons/redis/templates/clusterdefinition.yaml`、`cmpd-redis.yaml`、`cmpd-redis-cluster.yaml`、`shardingdefinition.yaml`、`cmpv-redis.yaml`、`backuppolicytemplate.yaml`、`backupactionset.yaml`、`paramsdef-redis.yaml`。
- ClickHouse：`addons/clickhouse/templates/clusterdefinition.yaml`、`shardingdefinition.yaml`、`cmpd-ch.yaml`、`cmpd-keeper.yaml`、`cmpv.yaml`、`paramsdef-config.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。
- Kafka：`addons/kafka/templates/clusterdefinition.yaml`、`cmpd-broker.yaml`、`cmpd-controller.yaml`、`cmpd-combine.yaml`、`cmpv-broker.yaml`、`paramsdef.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。
- Etcd：`addons/etcd/templates/cmpd.yaml`、`cmpv.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。

并不是每个 release-1.0 addon 都有完整 ClusterDefinition。Etcd addon 本体主要提供 ComponentDefinition、ComponentVersion、BackupPolicyTemplate、ActionSet，示例可以直接用 `componentDef` 创建 Cluster。如果 addon 面向用户提供多个标准 topology，就提供 ClusterDefinition；如果只暴露一个轻量组件或作为外部依赖，也可以用 direct componentDef 示例，但这种写法不享受 topology orders。

chart 层先做闭合检查，再谈业务语义。至少用代表性 values 渲染 standalone、HA、proxy、sharding、TLS on/off、不同 serviceVersion、备份恢复开关等组合，确认 helper、ConfigMap key、script key、`templateName`、`compDef`、BPT `compDefs`、PD `fileName`、ActionSet 名称、examples 中的名字全部闭合。topology 是验收维度，不是 README 分类；某个 topology 的 Cluster 能 Running，不代表它的 BackupPolicy、参数、proxy 路由、roleSelector Service、Restore 都可用。

发布和 GitOps 验收要保存三类结果：旧版渲染、新版渲染、server-side dry-run 或 live object diff。release-1.0 API 中多处 list 字段使用 merge/retainKeys，例如 Cluster component 的 `volumeClaimTemplates` 和 OpsRequest 的 `volumeClaimTemplates`；因此不能只用文本 diff 判断最终对象。遇到升级或同步异常，先查 live 对象是否保留了旧字段、旧 ConfigMap/ActionSet/BPT 是否被 Helm keep 或 cluster-scoped 资源残留遮蔽，再查 controller 是否消费了预期 definition。只有在渲染对象、apiserver 默认化结果和下游生成对象都指向同一结论后，才讨论 KB 实现问题。

## 3. 用 ComponentDefinition 表达组件蓝图

release-1.0 的 `ComponentDefinition` 是组件静态蓝图。它描述 runtime PodSpec、volumes、configs、scripts、services、vars、systemAccounts、TLS、roles、lifecycleActions、ServiceRefDeclarations、update 策略和 RBAC。用户主要写 Cluster 和 definition；`Component` 是 controller 合成出的内部对象。Cluster 的 `componentSpecs` 提供实例化时的动态输入，例如副本数、资源、存储容量、调度、TLS 开关、serviceRefs、serviceVersion、外部服务绑定和用户覆盖配置。

最小组件要能闭合以下关系：

- `spec.runtime.containers[*].volumeMounts[*].name` 必须能在 `spec.volumes`、`spec.configs[*].volumeName`、`spec.scripts[*].volumeName` 或 runtime volumes 中找到。
- `spec.configs[*].name` 是参数链路绑定键；`spec.configs[*].template` 是模板 ConfigMap 名；`spec.configs[*].volumeName` 决定挂载到哪个 volume。
- `spec.scripts[*].template` 提供脚本 ConfigMap，`defaultMode` 要让容器内脚本可执行。
- `spec.services[*].roleSelector` 依赖 `spec.roles` 和 `lifecycleActions.roleProbe`。
- `systemAccounts`、TLS、vars 要能在启动脚本、lifecycle action、backup/restore 或外部连接里找到使用点。
- `status.phase=Available` 是被 Cluster、BPT、PD、ComponentVersion 等引用前的最低门槛。

release-1.0 API 注释明确：`runtime` 用于静态 PodSpec，CPU/memory、调度等动态设置应放到 `cluster.spec.componentSpecs`；`ComponentDefinition.status.phase=Available` 表示可以被相关对象使用。不要只看 CR 创建成功。

字段语义矩阵：

| 字段或对象关系 | release-1.0 实践语义 | 必须同时对齐 | 常见误用 |
| --- | --- | --- | --- |
| `spec.serviceKind` | 组件提供的服务类型，BPT、ServiceRefDeclaration 等也会使用同类概念 | BPT `serviceKind`、ServiceDescriptor、README | 大小写或命名不统一导致外部依赖和策略不匹配 |
| `spec.serviceVersion` | 组件内核版本；可被 ComponentVersion release 覆盖 | ComponentVersion、PD serviceVersion、BPT versionMapping、examples | 未显式写 serviceVersion，误以为选择固定版本 |
| `spec.runtime` | 静态 PodSpec | volumes、configs、scripts、ports、command | 把资源和调度这类实例参数写死 |
| `spec.volumes[*].name` | 数据卷稳定身份 | Cluster VCT、runtime mount、BPT targetVolumes、volume expansion | PVC 创建了但没有挂到数据库目录 |
| `spec.configs[*].name` | 配置模板身份和参数绑定键 | PCR `configs[*].templateName`、Cluster configs、ComponentParameter | 写成模板 ConfigMap 名 |
| `spec.configs[*].template` | 模板 ConfigMap 名 | chart 渲染出的 ConfigMap | 模板 ConfigMap data 没有 PD `fileName` |
| `spec.scripts[*].template` | 脚本 ConfigMap 名 | command/action 脚本路径、defaultMode | 脚本存在但不可执行 |
| `spec.services[*].roleSelector` | 按 role 暴露服务 | roles、roleProbe、Service endpoints | 没有 roleProbe 却声明 roleSelector |
| `spec.vars` | 注入 Pod/action 或渲染模板的运行时变量 | 脚本消费格式、Optional/Required 分支 | 脚本依赖未声明的变量格式 |
| `spec.systemAccounts` | KB 管理的系统账号 | accountProvision、credentialVarRef、backup/probe/replication 脚本 | Secret 存在但数据库内账号未创建 |
| `spec.tls` | TLS 文件形状和挂载路径 | Cluster TLS、启动配置、client、backup/restore job | 只验证 runtime，不验证其它执行面 |
| `spec.roles` | roleProbe 输出集合和 update/service/backup 的角色基础 | roleProbe、updateStrategy、BPT target | role label 等同于业务一致性 |
| `spec.lifecycleActions` | 组件生命周期脚本和 probe | action target、kbagent、Ops、delete/scale | action 成功等同于业务收敛 |
| `spec.serviceRefDeclarations` | 外部依赖声明 | Cluster serviceRefs、serviceRefVarRef | 只声明依赖，没有 Cluster 绑定示例 |
| `spec.replicasLimit` | 声明 scale 支持边界 | README、HorizontalScaling、examples | 声称支持边界但没有业务验证 |

release-1.0 还有几组容易被低估的 ComponentDefinition 合同：

| 能力 | release-1.0 字段和实现证据 | addon 写法要求 | 排查要点 |
| --- | --- | --- | --- |
| 数据卷和容量保护 | `spec.volumes[*].needSnapshot/highWatermark`、lifecycle `readonly/readwrite`、kbagent 任务配置 | 数据目录和 VCT 名称必须一一对应；如果声明 highWatermark，必须实现 readonly/readwrite 并证明业务进入/退出只读 | PVC、mountPath、volume 名、kbagent task、数据库读写状态 |
| 可用性口径 | `available.withPhases/withRole/withProbe`，controller 会校验 `withProbe` 对应 availableProbe | 只有 probe 输出、role 和业务连接都闭合时才把 Component Available 当业务可用 | Component status、role label、probe stdout/stderr、连接测试 |
| 单 Pod Service | `services[*].podService` 会按 Pod 生成 Service，`disableAutoProvision` 会跳过自动创建 | 需要 pod 级地址时显式声明 podService；不自动创建时要说明由谁创建 Service | Service 名称、selector、EndpointSlice、serviceVarRef 输出格式 |
| hostNetwork | API 会把 DNSPolicy 调整到 `ClusterFirstWithHostNet`，port 来自 containerPorts | 只在引擎必须使用宿主网络时启用，并验证端口冲突和调度 | Pod spec、Node 端口占用、hostNetworkVarRef |
| role 更新优先级 | `roles[*].updatePriority/participatesInQuorum/isExclusive` 影响角色语义和更新判断 | roleProbe 输出必须稳定；quorum 角色不能只靠名称猜测 | role label、InstanceSet update 顺序、业务 quorum |
| 外部管理配置 | `configs[*].externalManaged/restartOnFileChange` 影响配置所有权和重启行为 | externalManaged 表示交给 parameters 链路管理，不表示模板 ConfigMap 名可随意替换 | PD/PCR/ComponentParameter、runtime ConfigMap、Pod restart |

MySQL `cmpd-mysql80.yaml` 展示了典型蓝图：声明 `serviceVersion: 8.0.33`、配置模板 `mysql-replication-config`、`externalManaged: true`、runtime container、scripts、data volume 和工具 init container。PostgreSQL `cmpd.yaml` 更复杂：它声明 ServiceRef、roleSelector Service、roles、configs、scripts、vars、systemAccounts、TLS 变量和 accountProvision。Redis、Etcd 展示了 roleProbe/member lifecycle 轻量形态。

release-1.0 controller 会做一部分定义校验，例如 service 端口、roleSelector、account/lifecycle、file template、hostNetwork 等，但 addon 不能把校验层当成完整业务证明。`roles` 与 `roleProbe` 的绑定、账号权限、TLS 工具链、脚本幂等和业务 member 状态仍要由 addon 的示例和测试自证。

写完 ComponentDefinition 后必须立刻用最小 Cluster 样例验收：Cluster 能解析 topology 或 direct componentDef，生成 Component，Component 继续生成 InstanceSet/Pod/PVC/Service，Pod 内脚本和配置文件存在并权限正确。不要等到参数、备份和升级都写完后才发现基础蓝图未闭合。

数据目录初始化、默认资源和文件权限也属于 ComponentDefinition 合同。release-1.0 会把 Component 合成为 InstanceSet，并在 factory 层处理默认资源、volumeClaimTemplates 和工作负载字段；addon 启动脚本不能只用“目录是否为空”决定是否初始化或清空数据。至少要区分空目录、只有 `lost+found` 的新 PVC、已有业务数据、restore/rebuild 写入的数据和半初始化目录。`defaultMode` 要按最终 Pod spec 和容器内 `stat` 验收：真实 addon 中同时存在 `0555`、`0755`、`0777`、`0444` 和十进制 `365` 这类写法，复核时不能只看源码字面量像不像八进制。

`replicasLimit`、`podUpdatePolicy`、`podUpgradePolicy`、`instanceUpdateStrategy` 和 `policyRules` 都不是装饰字段。声明 scale 边界后，要验最小值、最大值、越界拒绝、HorizontalScaling、0 副本语义和业务成员关系；声明 ReCreate 或其它更新策略后，要在 upgrade/restart/reconfigure 中看 InstanceSet revision、current/updated replicas、partition 或 rollout 状态；声明 `policyRules` 后，要分别验证 runtime Pod、lifecycle action、backup/restore Job 是否使用到正确 ServiceAccount 和权限。权限不足常表现为 action 或 Job 失败，不应误判成脚本逻辑错误。

## 4. 用 ClusterDefinition 表达拓扑，不表达组件细节

release-1.0 的 `ClusterDefinition` 主要表达 topology：每个 topology 包含 components、shardings、orders 和 default。组件 runtime、脚本、账号、TLS 不应该放在 ClusterDefinition 里。

实践规则：

- 每个用户可选部署形态都应有明确 topology。MySQL 的 `semisync`、`mgr`、`orc`、`orc-proxysql`、`mgr-proxysql`、`semisync-proxysql`，Redis 的 `standalone`、`replication`、`cluster`，Kafka 的多组件拓扑，都是 release-1.0 的真实模式。
- `default: true` 要显式设置。Cluster 未指定 topology 时应依赖默认 topology，不应让用户猜测。
- `components[*].name` 是 topology 内组件名，Cluster `componentSpecs[*].name` 要和它对齐。
- `components[*].compDef` 支持精确名、前缀或正则，release-1.0 API 注释写明系统会选择匹配的最新 ComponentDefinition。多匹配时的版本优先级和同名排序不应作为 addon 合同，addon 作者应使用 helper 固定规则并用渲染结果验证，避免宽正则误命中未来或残留版本。
- `orders.provision/terminate/update` 表示跨组件顺序。proxy、sentinel、keeper、controller/broker 这类辅助组件要显式排序。release-1.0 controller 会校验 orders 覆盖 topology 中的 component 和 sharding，漏写 orders 会直接影响 Cluster 可用性。
- sharding 不等于 replicas。`replicas` 是一个复制组内实例数，`ShardingDefinition` 表示多个 shard component。

拓扑能力矩阵：

| 拓扑能力 | release-1.0 表达方式 | controller 消费点 | 验收证据 | 不支持或未证明边界 |
| --- | --- | --- | --- | --- |
| 单组件 standalone | `ClusterDefinition.spec.topologies[*].components` | Cluster normalization | Component、InstanceSet、Pod、Service 全部生成 | 不证明 HA、backup、reconfigure |
| 多组件 HA/proxy | topology components + orders | cluster/component reconcile 顺序 | server 先于 proxy 创建，proxy 先删除；服务可连接 | proxy 路由表和后端角色要另验 |
| MGR/orc/sentinel/keeper | 不同 topology 指向不同 compDef | compDef matching | 每个 topology 的 CMPD/CMPV/PD/BPT 都闭合 | 一个 topology 成功不代表另一个成功 |
| sharding | `ShardingDefinition` + topology `shardings` | sharding/component transformer | shard Component、Service、vars、scale 行为 | 不自动证明 rebalance 或数据迁移 |
| direct componentDef | Cluster component 直接指定 ComponentDefinition，如果使用 | Cluster normalization | 不依赖 ClusterDefinition orders | 不应和 topology 语义混用 |

ClickHouse `shardingdefinition.yaml` 和 Redis `shardingdefinition.yaml` 是 release-1.0 sharding 的关键样本。它们说明 shard 数、shard template 和 lifecycle 可以由单独对象表达，但 shard 增删后的业务数据迁移、slot/partition rebalance、跨 shard 元数据修复仍要由 addon 自己证明，不能只凭 ShardingDefinition 存在就写成已支持。

sharding 是 release-1.0 的真实能力，但不要写得过满。Redis 是重点样本：它同时有 `ClusterDefinition.topologies[*].shardings`、`ShardingDefinition`、Cluster 示例和参数配置；需要独立验证 `Cluster.spec.shardings`、cluster-level service `componentSelector`、shard scale 和 shard lifecycle 是否真正闭合。

`ShardingDefinition` 的语义要按对象链理解：`spec.template.compDef` 指向每个 shard 使用的 ComponentDefinition，`shardsLimit` 约束 shard 数量，`provisionStrategy/updateStrategy` 和 lifecycle `postProvision/preTerminate/shardAdd/shardRemove` 只表达 shard 组件的创建、更新和增删动作。它不自动表达 Redis slot 迁移、ClickHouse 分布式表修复、Kafka partition rebalance 或跨 shard restore。排查 sharding 时先看 Cluster topology 的 sharding 名，是否匹配到 ShardingDefinition，再看生成的 shard Component、每个 shard 的 ComponentDefinition/ComponentVersion、Service/vars 和业务层分片状态。

排查 topology 时按这个顺序：Cluster `spec.clusterDef/topology/componentSpecs/shardings`，ClusterDefinition status 和 topology，匹配到的 ComponentDefinition/ComponentVersion，生成的 Component，InstanceSet/Pod/Service。不要把 topology component name、generated Component object name、Pod name、PVC name 和引擎成员 ID 混为一谈。

实例身份要从 Component 往下看，而不是靠名字猜。release-1.0 使用 InstanceSet 承载工作负载，Pod/PVC/Service 的名字、label、ownerRef、ordinal 和引擎内部成员 ID 不是同一种身份。scale、rebuild、restore、同名 Cluster 重建和 sharding 场景下，脚本可以读取 Pod 名或 FQDN 作为运行时输入，但不要把它们写入不可重算的持久成员 ID。确实需要持久化成员映射时，应能从业务元数据重建，并在 restore/rebuild/scale-in 后验证映射仍然成立。

## 5. 用 ComponentVersion 管理版本矩阵

release-1.0 有 `apps.kubeblocks.io/v1` 的 `ComponentVersion`。它用 `compatibilityRules[*].compDefs` 把一组 ComponentDefinition 和若干 release 关联起来；每个 release 有 `name`、`serviceVersion` 和 `images`。API 注释明确：release 的 `serviceVersion` 会作为被选择 release 的 serviceVersion，覆盖 ComponentDefinition 自身定义的 serviceVersion。

版本管理至少分五层：

| 版本维度 | release-1.0 来源 | 被谁消费 | 必须验证什么 |
| --- | --- | --- | --- |
| chart version | `Chart.yaml` | Helm/package/release 管理 | 与模板、values、README 一致 |
| ComponentDefinition name | CMPD metadata/helper | ClusterDefinition、BPT、PD、CMPV | 正则只命中预期版本 |
| `spec.serviceVersion` | CMPD 或 CMPV release | Cluster normalize、PD、BPT versionMapping、upgrade | resolved serviceVersion 是预期版本 |
| runtime image | CMPD runtime 或 CMPV `images` | Pod/InstanceSet | 实际 Pod image 被替换 |
| action/tool image | lifecycle action、ActionSet、BPT env | action worker、backup/restore Job | 工具版本与 serviceVersion 兼容 |

实践规则：

- ComponentDefinition 名称应带引擎大版本或 addon 版本前缀，例如 MySQL 的 `mysql-8.0`、`mysql-8.4` 系列，Redis 的 `redis`、`redis-cluster`、`redis-sentinel` 系列。
- ComponentVersion `compatibilityRules[*].compDefs` 可以写精确名、前缀或正则；release 名必须能在 `spec.releases[*].name` 中找到。
- `images` key 要对应 runtime container、init container、lifecycle action 字段名或外部工具名。未知 key 不应被当成已验证替换。
- 每个 serviceVersion 都要同时验证 runtime image、参数 schema、BPT method env、backup/restore 工具镜像和升级路径。
- Cluster 未显式指定 serviceVersion 时，不要猜测最终选择；开发样例应显式写 serviceVersion，排查时看 resolved Component 和 Pod image。

版本矩阵要覆盖“声明面”和“执行面”两类镜像。runtime container 和 init container 是声明面；lifecycle action image、backup/restore ActionSet image、参数 reload 脚本依赖的工具镜像是执行面。release-1.0 的 ComponentVersion 能改写 ComponentDefinition 中的 container、init container 和 lifecycle action image，但不会自动改写任意 ConfigMap 脚本里硬编码的工具镜像，也不会自动覆盖 ActionSet image。ActionSet/BPT 的工具镜像如果随数据库版本变化，需要在 BPT method env、ActionSet 模板或 chart values 中单独版本化。

真实样本：

- MySQL `cpmv.yaml`、`cpmv-orc.yaml`、`cpmv-mgr.yaml`、`cpmv-proxysql.yaml` 拆开管理不同组件和拓扑。
- PostgreSQL `cmpv.yaml` 按版本循环生成 release。
- Redis 拆出 `cmpv-redis.yaml`、`cmpv-redis-cluster.yaml`、`cmpv-redis-sentinel.yaml`、`cmpv-redis-twemproxy.yaml`。
- Kafka 拆出 broker、controller、combine 等 ComponentVersion。

升级验收不能只看新镜像启动。至少要保存 old/new 的 ComponentVersion status、Cluster/Component resolved serviceVersion、Pod image、action image、参数定义和 BPT method env。跨大版本的数据兼容、回滚限制和备份恢复兼容要写进 README 或实现说明；没有证据的组合写成未证明。

release-1.0 未指定 serviceVersion 时存在自动选择路径。代码会在兼容的 ComponentDefinition 与 serviceVersion 中选择可用版本，并改写 runtime、init container 和 lifecycle exec image。这个行为适合 controller 默认化，不适合 addon 作者逃避版本矩阵；examples 和测试应显式写 serviceVersion，避免“最新匹配”随残留对象或新 release 漂移。

BPT 的 `versionMapping` 是数据保护侧的版本矩阵。release-1.0 实现会按 Cluster 组件的 serviceVersion 匹配 method 侧版本，支持 exact、prefix 或 regexp 一类规则。它只能说明某个 backup method 用哪个版本化配置，不证明该 method 可 restore、可 rebuild 或可跨版本。每增加一个 ComponentVersion release，都要回查 PD/PCR、BPT `versionMapping`、ActionSet image/env 和 examples。

## 6. 生命周期动作要按数据库语义建模

release-1.0 `ComponentLifecycleActions` 支持 `postProvision`、`preTerminate`、`roleProbe`、`availableProbe`、`switchover`、`memberJoin`、`memberLeave`、`readonly`、`readwrite`、`dataDump`、`dataLoad`、`reconfigure`、`accountProvision`。其中 `reconfigure` 在 API 注释里标为 reserved for future versions；release-1.0 新 addon 不应把它作为通用参数热更新入口，参数体系仍主要依赖 ParametersDefinition 的 reloadAction 或重启路径。`readonly/readwrite` 可以作为 highWatermark 等容量保护链路的一部分，但只有在 addon 同时声明 volume highWatermark、实现 action、验证 kbagent 触发和数据库读写状态后，才能写成支持。

动作矩阵：

| 动作 | release-1.0 触发时机 | 输入和目标 | addon 要证明什么 | 常见误区 |
| --- | --- | --- | --- | --- |
| `postProvision` | 组件创建后，按 `preCondition` | `Immediately`、`RuntimeReady`、`ComponentReady`、`ClusterReady` | 一次性初始化幂等，依赖对象已存在 | Start/Restart 后期待重新执行 |
| `preTerminate` | 删除或缩容释放资源前 | 默认目标或指定 target | Pod/执行面存在时能完成下线；失败可诊断 | Pod 已消失还假设能执行 |
| `roleProbe` | 周期性探测 | 输出必须是预定义 role | Service、backup target、switchover、rollout 的角色基础 | role label 等同于数据库内部安全 |
| `availableProbe` | 周期性探测可用性 | action 输出和退出码 | Component Available 口径和业务可服务一致 | 探针通过但业务不可写 |
| `switchover` | 切换角色 | `KB_SWITCHOVER_*` 变量 | candidate 和无 candidate 路径都能解释 | action 成功后不查 role 收敛 |
| `memberJoin` | 新副本 Pod Ready 后 | `KB_JOIN_MEMBER_POD_NAME/FQDN` | 业务成员加入、重复执行安全 | scale-out 只看 Pod Ready |
| `memberLeave` | 移除副本前 | `KB_LEAVE_MEMBER_POD_NAME/FQDN` | 成员摘除、数据迁移边界、失败可恢复 | 把复杂 rebalance 塞进不可审计脚本 |
| `readonly/readwrite` | highWatermark 或显式 action 触发只读/读写切换 | 目标 Pod/action、数据库状态 | action 幂等，业务读写状态可观测 | 只声明字段，不验证 kbagent 和业务状态 |
| `dataDump/dataLoad` | 实例初始化类数据复制 | dump stdout、load stdin | 数据流干净且可重试 | 当成完整 Backup/Restore |
| `accountProvision` | 创建非 init system account | `KB_ACCOUNT_*` | SQL/CLI 脱敏、转义、权限正确 | Secret 存在就认为账号已建 |

action 执行成功只说明 release-1.0 action 机制认为该执行返回成功，不证明数据库业务状态已经收敛。每个动作都要配后置验证：roleProbe 看角色，Service/EndpointSlice 看流量目标，业务 CLI/SQL 看成员关系、读写状态、账号权限或数据结果。

roleProbe 的 release-1.0 API 注释明确：如果 Component 定义了 roles，应该定义 roleProbe；roleProbe 输出必须匹配预定义 role。任何 roleSelector service、BackupPolicy target role、switchover、updatePriority 都依赖这个基础。MySQL、PostgreSQL、Redis、Etcd 都有 role/lifecycle 相关脚本，是开发和复核时必须重点验证的部分。

`preTerminate` 是高风险动作。它适合“目标 Pod 或可管理执行面仍存在时”的业务下线。删除卡住时先查 Cluster/Component deletionTimestamp、InstanceSet/Pod 是否还存在、action 目标和 action 输出，再判断是否是 addon 设计问题。绕过执行只能作为有风险的运维恢复，不能作为 addon 删除流程验收通过。

Action target 也要按 release-1.0 实现验证。`Any`、`All`、`Role` 是主要可用路径；`Ordinal` selector 在代码中有声明但返回不支持。需要按实例身份操作时，应通过 Ops 输入、Pod/Instance 名、业务成员 ID 和脚本自检建立明确映射，不要依赖 ordinal 直接选择 action 目标。

action 的执行格式要写清楚。`exec.command` 是命令数组，不是 shell 字符串；如果脚本需要管道、重定向、变量展开或多命令，应调用脚本文件或显式 `sh -c`。`preCondition` 主要用于 `postProvision`，不要把它泛化为所有 lifecycle action 的等待条件。HTTP/GRPC action 如果使用，只能作为一次请求式动作，仍要配 timeout、retry 和业务后置验证；不要把 HTTP 200 或 gRPC OK 当成集群状态收敛。

## 7. 变量注入和服务依赖是契约

release-1.0 `ComponentDefinition.spec.vars` 支持多种来源：cluster、component、credential、service、serviceRef、TLS、hostNetwork、ConfigMap、Secret 等。变量可用于 Pod/action env，也可用于渲染 config/script 模板。变量不是脚本私货；每个变量的格式、缺失语义和刷新语义都要写进 addon 实现说明。

常见变量和服务来源：

| 来源 | release-1.0 输出或行为 | 脚本消费方式 | 验收证据 | 边界 |
| --- | --- | --- | --- | --- |
| `clusterVarRef` | cluster name、namespace、uid 等 | 普通 env/template 值 | 最终 Pod/action env | 不应作为不可重算业务成员 ID |
| `componentVarRef` | component 名、replicas、Pod 名/FQDN 列表、按 role 过滤列表 | 逗号列表等字符串约定 | 脚本解析、scale 后验证 | Pod env 是创建时视图，拓扑变化要另验 |
| `credentialVarRef` | system account 用户名/密码 | env 传入脚本 | Secret 和数据库内账号都存在 | 不应渲染进普通 ConfigMap |
| `serviceVarRef` | 同 Cluster 内 KB 管理 Service host/port 等 | 连接地址或 advertised address | Service、EndpointSlice、连接测试 | Service 有值不等于业务可服务 |
| `serviceRefVarRef` | Cluster 绑定的外部服务或其它 Cluster 服务 | 外部依赖 endpoint/credential | ServiceRef declaration、Cluster binding、真实连接 | declaration 不等于绑定 |
| `tlsVarRef` | TLS 是否启用等 | 启动/连接分支 | TLS on/off 两套用例 | enabled 不证明工具链兼容 TLS |
| `hostNetworkVarRef` | hostNetwork 场景端口 | advertised address | Pod spec 和实际端口 | hostPort/hostNetwork 有调度冲突边界 |

PostgreSQL `cmpd.yaml` 是 ServiceRef 和 vars 的典型样本：它声明 `etcd`、`remote-instances` 两类 ServiceRefDeclaration，又用 `serviceRefVarRef` 注入 endpoint、host、port、username、password；同时注入 `POSTGRES_POD_NAME_LIST`、`POSTGRES_POD_FQDN_LIST`、credential、TLS enabled 等变量。Redis 和 Kafka 使用 serviceVarRef 构造对内或对外地址。

实践规则：

- 每个变量都要有脚本使用点；不用的变量不要提前注入。
- Optional 缺失时脚本要处理 unset 和 empty；Required 缺失应尽早失败。
- 列表格式要固定，例如逗号分隔 Pod/FQDN 列表；脚本不要用未说明的 `awk/cut/tr` 约定。
- 同名 env 覆盖以最终 Pod/action env 为准，不以 YAML 书写顺序推断。
- 外部依赖要写 declaration、Cluster 绑定示例、变量读取方式和变更生命周期。
- 拓扑变化后的 action 不要依赖创建时写入 Pod env 的成员列表。scale、switchover、rebuild 后优先使用 action-time 注入、当前 Service/EndpointSlice 或业务 membership 查询。

release-1.0 的变量解析还有几个具体格式要在实现说明中写明：

| 变量类型 | 1.0 输出细节 | 实践要求 |
| --- | --- | --- |
| component Pod/FQDN 列表 | 通常是逗号分隔字符串，可按 role 过滤 | 脚本要固定分隔符解析，并处理空列表 |
| service host | 普通 Service 输出单个 host；podService 输出按 Service 名排序后的逗号列表 | 需要 pod 级地址时不要按单值解析 |
| service port/loadBalancer | podService 输出 `serviceName:value` 形式；NodePort/LoadBalancer 可能输出对外端口 | advertised address 要按 Service type 分支验证 |
| credential | 用于 Pod/action env；不应假设可直接渲染进普通 ConfigMap | 敏感值只在需要的执行面注入 |
| MultipleClusterObjectOption | 可 individual 或 combined，并设置 delimiter/keyValueDelimiter | 多组件、多 shard 场景要写清楚变量名和拼接格式 |

服务排查顺序是：ComponentDefinition services，Cluster/Ops expose 或 Service override，最终 Service type/ports/selector，EndpointSlice，Pod label/Ready，注入 env，真实连接。不要修改 KB 系统 selector label 来“修”服务。

## 8. 账号和 TLS 要作为一等能力设计

release-1.0 `systemAccounts` 用于声明 KB 管理的系统账号。API 注释把系统账号和用户账号区分开：系统账号用于初始化、备份、探测、复制和管理动作。非 init account 通常需要 `lifecycleActions.accountProvision` 执行 SQL 或 CLI 创建语句。

账号/TLS 使用面：

| 使用面 | release-1.0 凭据来源 | 需要验证什么 | 失败排查入口 |
| --- | --- | --- | --- |
| runtime | init account、Secret、credentialVarRef | 启动和业务连接可用 | Pod env/Secret、数据库内账号 |
| lifecycle/action | `accountProvision`、roleProbe/switchover/member 脚本 env | 目标 Pod 能执行，权限最小且脱敏 | action 输出、stderr、数据库权限 |
| backup/restore | BPT target account、ActionSet env、BackupRepo Secret | Job/action 能连库和访问仓库 | Backup/Restore Job spec、Secret mount/env |
| external dependency | ServiceDescriptor/ServiceRef credential | consumer 能读到 provider 凭据 | serviceRefVarRef env、真实连接 |
| public connection | 用户 Secret 或 README 声明 | README 连接串与实际 Secret/Service/TLS 一致 | Secret、Service、client 连接 |

PostgreSQL `cmpd.yaml` 声明了 `postgres`、`kbadmin`、`kbdataprotection`、`kbprobe`、`kbreplicator` 等账号，并通过 accountProvision 创建非 init 账号。MySQL、Redis 也有备份、探测或复制相关账号。账号名不是通用规则；通用合同是“每个执行面最小权限、可创建、可禁用、可诊断”。

TLS 不能只看开关。ComponentDefinition `tls` 定义挂载路径和文件名，Cluster 侧启用或提供证书后，还要验证 runtime 启动、client 连接、roleProbe、accountProvision、backup/restore、postReady 是否都能看到正确证书路径和 trust store。TLS 关闭时脚本不能仍强依赖证书文件。

Secret rotation 和 TLS rotation 不能从首次启动成功外推。Secret volume 可能更新，但数据库进程、连接池、action worker、backup/restore 工具是否热加载，需要 addon 自己证明；不能证明时写成需要 restart、reconfigure、rebuild 或人工处理。

账号设计要覆盖禁用和自带 Secret 两类路径。release-1.0 system account 可以由 KB 生成，也可能由用户提供 Secret 或禁用某些账号；如果 addon 的 roleProbe、backup、replication 或 switchover 必须依赖某个系统账号，values 和 README 要明确禁止禁用，或给出替代凭据路径。密码策略只约束生成密码的形状，不证明数据库内权限、过期策略和轮换行为。

## 9. 配置和参数管理要串成闭环

release-1.0 的参数链路由 ComponentDefinition configs、模板 ConfigMap、`ParametersDefinition`、`ParamConfigRenderer`、`ComponentParameter`、runtime ConfigMap、reload/restart 路径共同构成。老的 `ConfigConstraint` 类型仍存在于 API 包中，但 release-1.0 真实主流 addon 已大量使用 `ParametersDefinition` 和 `ParamConfigRenderer`；新建或重写 addon 不应回到旧 ConfigConstraint 叙述。

对象链：

```text
ComponentDefinition.spec.configs[*].name        # 配置模板身份，也是 PCR configs[*].templateName 应绑定的对象
ComponentDefinition.spec.configs[*].template    # 模板 ConfigMap 名
模板 ConfigMap data key                         # PD.fileName 要能找到的文件
ParametersDefinition.spec.fileName              # schema 和参数分类作用的文件名
ParamConfigRenderer.spec.componentDef           # 绑定目标 ComponentDefinition
ParamConfigRenderer.spec.parametersDefs         # 引用一个或多个 ParametersDefinition
ParamConfigRenderer.spec.configs[*].name        # 配置文件名
ParamConfigRenderer.spec.configs[*].templateName # 引用 CMPD config item name
ComponentParameter                              # controller 落地的参数状态
runtime ConfigMap                               # 最终挂进 Pod 的配置文件
reloadAction 或 restart                         # 参数真正生效的路径
进程内配置查询                                  # 最终业务证据
```

release-1.0 `ParametersDefinitionSpec` 包含 `fileName`、`parametersSchema`、`reloadAction`、`downwardAPIChangeTriggeredActions`、`deletedPolicy`、`mergeReloadAndRestart`、`reloadStaticParamsBeforeRestart`、`staticParameters`、`dynamicParameters`、`immutableParameters`。`ParamConfigRendererSpec` 包含 `componentDef`、`serviceVersion`、`parametersDefs` 和 `configs`；其中 `configs[*].templateName` 才是连接 CMPD `spec.configs[*].name` 的关键字段。API 注释明确：动态 reload 只有在修改参数属于 dynamicParameters，且 reloadAction 已设置时才会触发；`reloadStaticParamsBeforeRestart` 是少数引擎需要先用 SQL/CLI 设置静态参数再重启的场景。

实践规则：

- `ParametersDefinition.spec.fileName` 必须是模板 ConfigMap data 中真实存在的文件名。
- `ParamConfigRenderer.spec.componentDef` 要指向目标 ComponentDefinition；`parametersDefs` 要列出参与渲染的 PD；`configs[*].templateName` 要等于 CMPD `spec.configs[*].name`，不是模板 ConfigMap 名，也不是 runtime ConfigMap 名。
- `staticParameters` 表示需要 restart，`dynamicParameters` 表示可 reload，`immutableParameters` 表示不应修改。没有分类证据时不要声明热更新。
- `reloadAction.targetPodSelector` 可限制 reload 作用 Pod；如果不设置，API 注释说明会考虑 workload 管理的所有 Pod。动态参数实际作用域仍要由引擎证明。
- `deletedPolicy` 要覆盖删参语义：RestoreToDefault 需要引擎支持，Reset 表示按模板重渲染。
- 外部 ConfigMap、formatter、default value、Secret 进入配置文件等能力要单独证明；敏感值不要渲染进普通 ConfigMap。

release-1.0 的参数对象链必须按下面的实现路径排查：

| 阶段 | 1.0 对象和实现 | 判断标准 | 常见误判 |
| --- | --- | --- | --- |
| 定义可引用 | `ParametersDefinition.status.phase`、`ParamConfigRenderer.status.phase` | phase 为 Available，observedGeneration 跟上 | CR 存在就认为可被 Component 使用 |
| ComponentParameter 生成 | `controllers/parameters/componentdrivenparameter_controller.go` 根据 Component、CMPD、PCR、PD 和模板 ConfigMap 生成 | `ComponentParameter.spec.configItemDetails[*].name/configSpec/configFileParams` 出现目标 config item | 没有 PCR 或 PD 不匹配时仍期待参数链路生效 |
| 初始参数归类 | `ClassifyParamsFromConfigTemplate` 读取模板和参数定义 | 用户初始参数进入对应文件，不污染其它文件 | 一个参数名在多文件中含义相同 |
| runtime diff | `controllers/parameters/config_util.go:createConfigPatch` 比较 last config 和 runtime ConfigMap | 能生成 file-level patch，并判断是否需要 restart | runtime ConfigMap 内容变了就一定 reload |
| reload/restart 决策 | `controllers/parameters/policy_util.go:resolveReloadActionPolicy` | dynamic + reloadAction 才走 reload；static 或无 reloadAction 走 restart | `dynamicParameters` 写了但没有 reloadAction |
| 状态落点 | `ComponentParameter.status.configurationStatus[*]` | phase、lastDoneRevision、updateRevision、reconcileDetail 能解释执行结果 | 只看 OpsRequest phase |

`Parameter` CR 是用户 desired 参数入口之一，OpsRequest `Reconfiguring` 也会生成或驱动参数状态。`ComponentParameter` 不是一次性命令，而是 controller 持续消费的 desired/执行模型；错误的 `configItemDetails`、`configFileParams` 或 `userConfigTemplates` 会在后续 restart、scale、upgrade 中继续影响 runtime ConfigMap。修复参数绑定错误时，不要只改 chart；要同时检查 `Parameter`、`ComponentParameter.spec`、`ComponentParameter.status`、runtime ConfigMap 和进程内配置。

release-1.0 参数字段的使用边界：

| 字段 | 1.0 语义 | addon 要怎么写 | 不能外推 |
| --- | --- | --- | --- |
| `PD.reloadAction` | Unix signal、shell、TPL script、auto 等 reload 触发方式 | 只给真正支持热更新的参数配置 | 不能让 static 参数自动热更新 |
| `PD.mergeReloadAndRestart` | 同一次 patch 同时需要 reload 和 restart 时是否合并为 restart | 需要在 mixed patch 用例中验证 | 不是 reload 和 restart 的全局顺序保证 |
| `PD.reloadStaticParamsBeforeRestart` | 某些引擎要求 static 参数先经 SQL/CLI 设置再重启 | 只在引擎有这个机制时使用 | 不能解决任意 static 参数生效问题 |
| `PD.deletedPolicy` | 删除参数时 Reset 或 RestoreToDefault | Reset 验模板重渲染；RestoreToDefault 验引擎命令 | 删除 key 后进程自动忘记旧值 |
| `PCR.configs[*].fileFormatConfig` | 解析 ini/xml/yaml/json/hcl/dotenv/toml/properties/redis/props 等格式 | 每个文件格式单独验证 patch/delete/default 行为 | 格式名相同就说明注释、顺序、转义都保留 |
| `PCR.configs[*].reRenderResourceTypes` | vscale、hscale、tls、shardingHScale 后触发重渲染 | 只用于配置确实依赖资源、TLS 或拓扑的文件 | scale 后进程一定已 reload |
| `ConfigTemplateExtension.policy` | 用户模板和默认模板的 patch/replace/add/none 合并策略 | 写清楚用户模板覆盖范围和恢复路径 | 任意外部 ConfigMap 都能安全 merge |
| `ParametersInFile.parameters[*]=nil` | 参数值为 nil 表示删除该参数 | 删参用例必须覆盖 runtime 文件和进程状态 | 删除 desired 等于删除进程内状态 |

`ComponentParameter.spec.configItemDetails` 是排查参数链路的关键落点。它应该能解释每个 config item 的模板、runtime ConfigMap、hash 和状态；如果 PD/PCR 看起来正确但 reconfigure 未生效，先看 ComponentParameter 是否已经把 desired 写到目标 config item，再看 runtime ConfigMap 和 Pod 是否拿到新的配置。错误的 desired 一旦落入 ComponentParameter，后续 restart/upgrade 可能继续携带错误状态，修复时要先纠正模板绑定和 desired，再考虑是否清理污染对象。

模板渲染能力要按 release-1.0 参数链路验证。formatter/default value 可以帮助把用户参数转成文件格式，但它不替 addon 自动判断静态/动态参数，也不保证删除参数后进程回到默认值。`restartOnFileChange`、`mergeReloadAndRestart`、`reloadStaticParamsBeforeRestart` 这类字段只能说明 controller 和 PD/PCR 的执行路径，最终仍要用进程内查询证明 effective 状态。

真实样本：

- MySQL `paramsdef-57.yaml`、`paramsdef-80.yaml`、`paramsdef-84-mgr.yaml` 和 `pcr-*.yaml` 展示了多版本、多拓扑参数定义。
- PostgreSQL `paramsdef.yaml` 和 `pcr.yaml` 绑定 postgresql/pgbouncer 配置。
- Redis `paramsdef-redis.yaml`、`paramsdef-redis-cluster.yaml` 以及 `pcr-redis*.yaml` 展示普通 redis 和 cluster 的差异。
- ClickHouse `paramsdef-config.yaml`、`paramsdef-keeper.yaml`、`paramsdef-user.yaml` 展示多文件、多组件配置。
- Kafka `paramsdef.yaml`、`pcr.yaml` 展示 broker/controller/combine 配置拆分。

排查 reconfigure 时按三层看：第一层 PD/PCR/ComponentParameter 是否匹配到目标组件和文件；第二层 runtime ConfigMap 内容是否符合 desired；第三层进程内配置是否真的改变。OpsRequest 成功不等于进程已生效，runtime ConfigMap 更新也不等于进程已 reload。

`ParamConfigRenderer.spec.serviceVersion` 存在，但 release-1.0 的校验和关联路径主要围绕 `componentDef`、`parametersDefs` 和 config template。不要因为写了 serviceVersion 就默认形成完整版本矩阵；每个 serviceVersion 的 PD/PCR、runtime ConfigMap 和进程配置都要用样例单独证明。

参数验收至少要覆盖四类用例：创建时初始参数、动态参数变更、静态参数变更、删参。每类都要记录 desired、rendered、effective 三态：desired 看 `Parameter`/OpsRequest 和 ComponentParameter，rendered 看 runtime ConfigMap 和 revision/status，effective 看 SQL/CLI/config dump。若涉及 hscale/vscale/tls/shardingHScale 重渲染，还要在对应 Ops 后重复检查 runtime ConfigMap 和进程状态。

## 10. 数据保护要同时提供策略和执行脚本

release-1.0 数据保护使用 `BackupPolicyTemplate`、`ActionSet`、`BackupPolicy`、`BackupSchedule`、`Backup`、`Restore`、`BackupRepo` 等 API。完整能力至少需要 BPT 绑定组件、ActionSet 定义执行阶段、脚本实现数据流、BackupRepo 提供仓库、用户示例证明 Backup 和 Restore。

数据保护对象矩阵：

| 对象或阶段 | release-1.0 语义 | addon 要提供什么 | 验收证据 |
| --- | --- | --- | --- |
| BackupPolicyTemplate | 根据 `compDefs` 和 method 为组件生成 BackupPolicy | serviceKind、compDefs、target、schedules、backupMethods | 目标 Cluster 生成 BackupPolicy |
| BPT target | 按 role、strategy、account 选备份目标 | roleProbe、system account、单副本 fallback 验证 | target Pod 和连接账号正确 |
| backup method | 用户可选备份方法 | method name、actionSetName 或 snapshotVolumes、targetVolumes、env | method 语义单一 |
| ActionSet backup | 执行 preBackup/backupData/postBackup/preDelete | 工具镜像、命令、状态同步、错误退出 | Backup Job/action 成功且有 artifact |
| ActionSet restore | 执行 prepareData/postReady | restore 工具、PVC、账号/TLS、postReady | Restore 后目标可连接并数据正确 |
| BackupRepo | 仓库和凭据 | repo provider、Secret、网络可达 | Backup/Restore Job 看到正确 env/mount |
| BackupSchedule | 自动创建 Backup | schedule、repoName、enabled 状态 | CronJob/Backup 真实产生 |
| Restore | 从 Backup 或时间点恢复 | source target、restoreTime、env、PVC、postReady | Restore phase、PVC、目标 Cluster 状态 |

release-1.0 BPT API 明确：`compDefs` 支持精确名、前缀或正则；`backupMethods[*].snapshotVolumes=true` 时可以使用 CSI snapshot，不一定需要 ActionSet；非 snapshot method 要有 `actionSetName`。`TargetInstance.role` 有一个重要边界：如果指定 role 不存在会失败，但单副本 Cluster 会使用唯一实例，即使它不是指定 role。多副本 target 语义不能直接外推到单副本。

BackupPolicyTemplate 生成 BackupPolicy 后，真正执行的是 BackupPolicy/Backup/ActionSet/BackupRepo 组合，而不是 BPT 本身。release-1.0 中 method 可以覆盖全局 target，`snapshotVolumes=true` 可以不需要 ActionSet，`compatibleMethod` 用于增量/差异备份寻找父备份，`versionMapping` 用 serviceVersion 选择 method env。开发时要为每个 method 建最小合同：

| method 维度 | 1.0 字段 | 必须证明 | 不能外推 |
| --- | --- | --- | --- |
| 目标选择 | BPT `target` 或 method `target` | role/fallbackRole/strategy/account 选中正确 Pod；单副本 fallback 单独验 | role 名存在等于 target 安全 |
| 执行方式 | `snapshotVolumes` 或 `actionSetName` | snapshot 产物或 ActionSet Job/exec 产物存在 | snapshot method 可 restore 到任意 StorageClass |
| 数据卷 | `targetVolumes`、ActionSet `runOnTargetPodNode` | 目标 volume、mountPath、PVC/PV 可访问 | volume 名和数据库数据目录天然一致 |
| 工具镜像/env | method `env/runtimeSettings/versionMapping`、ActionSet image/env | 每个 serviceVersion 的最终 Job env/image 正确 | runtime image 升级自动升级备份工具 |
| 参数 | ActionSet `parametersSchema`、Backup/Restore `parameters` | Selective 或工具参数合法并传入脚本 | 参数 schema 证明业务对象选择正确 |
| 删除和保留 | Backup `deletionPolicy`、retention、preDelete | CR 删除、repo artifact、snapshot/PVC 的保留策略可解释 | 删除 Backup CR 一定删除所有远端数据 |

真实样本：

- MySQL 提供 xtrabackup、incremental、pitr、volume snapshot、mydumper 等 ActionSet/BPT。
- PostgreSQL 提供 pg_basebackup、pgdump、wal-g、wal-g incremental、PITR。
- Redis 有 RDB/AOF 或 cluster 相关备份 ActionSet 和 BPT。
- ClickHouse 有 full/incremental ActionSet 和 BPT。
- Kafka 的 backup 更偏 topic/metadata 语义，不等价于块设备或数据库全量文件备份。
- Etcd target leader，ActionSet 提供 data dump/load。

每个 method 都要有 Restore 兼容矩阵：

| method | 新集群 restore | rebuild | PITR/增量链 | TLS | 账号 | 跨版本 | 跨拓扑 | sharding |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| physical/full | 待 release-1.0 样本证明 | 待证明 | 不适用或待证明 | 待证明 | 待证明 | 待证明 | 待证明 | 待证明 |
| logical | 待证明 | 待证明 | 不适用 | 待证明 | 待证明 | 待证明 | 待证明 | 待证明 |
| incremental/PITR | 待证明 | 待证明 | 必须证明 base/parent/timeRange | 待证明 | 待证明 | 待证明 | 待证明 | 待证明 |
| snapshot | 待证明 | 待证明 | 不适用 | 待证明 | 待证明 | 受 StorageClass/CSI 限制 | 待证明 | 待证明 |

Backup 成功不等于 Restore 成功。每个 method 要证明 artifact 位置、工具镜像、formatVersion 或等价格式、账号/TLS、postReady 和目标 topology。空数据、无增量、仓库不可用、旧 Secret 轮换、target Pod 误选，都要有可诊断输出。

release-1.0 的 Backup status 已经有 `formatVersion`、`path`、`kopiaRepoPath`、`persistentVolumeClaimName`、`timeRange`、`target/targets`、`backupMethod`、`encryptionConfig`、`actions`、`volumeSnapshots`、`parentBackupName`、`baseBackupName`、`extras` 等字段。addon 不需要每个字段都使用，但只要声明相应能力，就要把这些 status 作为验收入口。例如 PITR/Continuous 要看 `timeRange`，增量要看 parent/base，snapshot 要看 `volumeSnapshots`，加密要看 `encryptionConfig` 和对应 Secret。

增量、差异和 Continuous 的父链不能靠命名约定。release-1.0 会校验父备份 phase、policy、method/compatibleMethod、结束时间、repo、加密配置和 target 数量；shard 数变化时可能找不到合法父备份。addon 如果支持增量或 PITR，必须写清楚 full/base/parent/continuous method 的组合、schedule 与 on-demand 的差异、`parentBackupName` 是否允许用户指定，以及 shard/topology 变化后的限制。

release-1.0 Restore 和 Rebuild 有几个需要单独写明的边界。Restore 流程会从 Backup 的 cluster snapshot annotation 重建目标 Cluster 的部分 spec，TLS 等 component 级开关可能被恢复流程重写或清空，不能假设源 Cluster 的 TLS 行为原样继承到目标。RebuildInstance 支持 in-place 或 scale-out 替换，并可通过 Backup/Restore CR 准备数据；但多副本从备份重建的注释和实现路径更偏 full physical backup，logical backup 不能直接写成支持 rebuild。Restore readinessProbe 相关实现仍有 TODO，恢复完成必须用 postReady、ReadyConfig、业务连接和数据校验证明。

数据保护的执行面要按消费者拆开验收。Backup method 被 Backup 直接消费，Restore 消费 ActionSet 的 prepareData/postReady 和源 Backup 元数据，RebuildInstance 又通过 operations 路径消费 restore 能力和目标实例成员关系。一个 method 在“新建 Cluster restore”可用，不自动证明“原 Cluster 单实例 rebuild”可用；一个 snapshot method 能产生 VolumeSnapshot，也不证明跨 StorageClass、跨 namespace、跨 topology 或 sharding restore 成立。每增加一个 ComponentDefinition、serviceVersion 或 topology，都要回看 BPT `compDefs`、`versionMapping`、method target、ActionSet image/env 和生成的 BackupPolicy。

Restore API 本身还能表达 `resources`、`prepareDataConfig`、`ReadyConfig`、`env`、`parameters` 等，但这些字段不是 addon 自动完成 restore 的证据。`resources` 只说明要恢复哪些 Kubernetes 资源；`prepareDataConfig.volumeClaims` 和 `volumeClaimsTemplate` 说明恢复 Pod/PVC 如何准备数据；`ReadyConfig` 说明 postReady 阶段如何执行或探测。addon 作者要把“恢复 Kubernetes 对象”“恢复 PVC 数据”“数据库启动并可读写”“账号/TLS/服务恢复”拆成不同验收点。

从已有 `Backup` 恢复新 `Cluster` 时，release-1.0 的用户入口应写成 Restore `OpsRequest`。不要要求 addon 用户手写 `kubeblocks.io/restore-from-backup` annotation；该 annotation 是 operations restore handler 根据源 Backup 和 OpsRequest 输入生成的内部恢复 intent。独立 `dataprotection Restore` CR 只负责 PVC 数据准备和 postReady，不会替用户创建新的 Cluster。

最小示例要包含目标 Cluster 名、源 Backup 名和必要的恢复选项：

```yaml
apiVersion: operations.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: restore-acmedb
  namespace: demo
spec:
  clusterName: acmedb-restore
  type: Restore
  restore:
    backupName: acmedb-backup-20260605
    backupNamespace: demo
    restorePointInTime: "2026-06-05T08:00:00Z"
    volumeRestorePolicy: Serial
    deferPostReadyUntilClusterRunning: true
    parameters:
      - name: restore-mode
        value: full
```

这个入口依赖源 Backup 中保存的 cluster snapshot annotation。controller 会先读取 `backupName/backupNamespace`，要求 Backup 已 Completed，continuous backup 则会校验 `restorePointInTime`；随后从 Backup 的 cluster snapshot 反序列化源 Cluster，改成 `spec.clusterName` 指定的新名字和 OpsRequest namespace，并写入恢复 annotation。后续 Cluster/Component reconcile 会基于 annotation 创建 `Restore` 对象、PVC 和 restore Job。addon README 的 restore 示例必须让用户先确认源 Backup 存在、phase 合法、component/sharding label 可识别、method 和 ActionSet 带 restore 阶段、artifact 可访问、Backup 中存在 cluster snapshot；否则新 Cluster 不会被可靠创建或无法完成数据恢复。

Restore OpsRequest 派生出的目标 Cluster 不是源 Cluster 的逐字段复刻。LoadBalancer Service、NodePort、selector、offline instances、TLS issuer、sharding account SecretRef、调度 selector 等字段可能被重置、清理或重写；addon 的连接文档和验收脚本必须以恢复后 live Cluster、Service、Secret 和 Pod spec 为准，不从源 Cluster YAML 推断最终值。

PVC 恢复只覆盖 Backup method `targetVolumes` 声明、且目标 Cluster volumeClaimTemplates 中存在的卷。target volume 名称对不上时，目标 PVC 可能只走普通 provision，不会恢复数据。sharding restore 要说明 source targets 数量和目标 shards 数量的关系；targets 多于 shards 应失败，目标 shards 多于 source targets 时额外 shard 的 skip 行为不能写成数据完整恢复。

restore 示例还要写恢复后的检查链：先看 OpsRequest phase/message，再看新 Cluster 是否创建、Cluster annotation 是否被逐步清理、Component/PVC 是否带 restore intent，最后看 `Restore` phase、restore Job、postReady、业务数据、账号、TLS 和 Service。跨 namespace 时必须显式写 `backupNamespace` 并验证 BackupRepo/Secret/Job 执行面有权限；PITR 只能用于对应 continuous method 和可恢复 timeRange，不能把任意 `restorePointInTime` 当成通用字段。

ActionSet `onError` 字段有 `Continue` 和 `Fail` 两个枚举，但 release-1.0 API 注释明确当前只有 `Fail` 支持，`Continue` 是未来能力。addon 不应把 Continue 当成可用容错策略。需要忽略某个步骤失败时，应在脚本内部显式处理并输出可审计信息，而不是依赖 controller 忽略错误。

BackupRepo 是独立依赖。Backup 会按 Backup label、BackupPolicy `backupRepoName` 或默认 BackupRepo 选择仓库；仓库必须是 Ready。BackupRepo 支持 Mount/Tool 两种 access method，status 里会暴露生成的 StorageClass、PVC、tool config Secret 等。repo 不可用、default repo 冲突、Secret 轮换、PVC/PV retain/delete、BackupRepo 删除被已有 Backup/PVC 阻塞，都应作为数据保护排查入口，而不是归因到 ActionSet。

BackupSchedule 不是 BPT schedules 的简单展示。BPT 可以生成 BackupSchedule；用户也可以看到独立的 BackupSchedule 对象。排查自动备份时要看 BackupSchedule `status.phase`、`status.schedules[*].lastScheduleTime/lastSuccessfulTime/failureReason`、生成的 CronJob/Backup、Backup labels 和实际 Backup status。`cronExpression` 使用 UTC；schedule `name` 为空时会使用 backupMethod 作为名称，多个 schedule 名称必须避免冲突。

## 11. Day-2 运维以内置操作为主

release-1.0 `OpsRequest.spec.type` 支持 `Start`、`Stop`、`Restart`、`Switchover`、`VerticalScaling`、`HorizontalScaling`、`VolumeExpansion`、`Reconfiguring`、`Upgrade`、`Backup`、`Restore`、`Expose`、`RebuildInstance` 等类型。addon 作者不要从 OpsRequest 入口推断能力已经成立；每种 Ops 都依赖底层合同。

Ops 矩阵：

| Ops | release-1.0 入口字段 | 依赖的 addon 合同 | 运行验收 | 常见误用 |
| --- | --- | --- | --- | --- |
| Start/Stop/Restart | `start/stop/restart`，空列表表示全部组件 | 启动脚本幂等、readiness、一次性初始化保护 | Pod 重建或恢复后数据不丢 | restart 后重复初始化 |
| HorizontalScaling | `horizontalScaling` | replicasLimit、memberJoin/memberLeave、sharding 边界 | 新成员加入或旧成员摘除 | 只看 Pod 数量 |
| VerticalScaling | `verticalScaling` | 资源变更后引擎是否感知 | Pod/cgroup/进程资源状态 | 资源改了但数据库内部未更新 |
| VolumeExpansion | `volumeExpansion` | VCT 名、PVC、mountPath、文件系统和引擎容量 | PVC、容器内 FS、引擎状态 | VCT 名和 volume 名不一致 |
| Reconfiguring | `reconfigures` | PD/PCR/ComponentParameter、reload/restart | 文件和进程配置一致 | 只看 Ops Succeed |
| Expose | `expose.services` | ComponentService、roleProbe、地址脚本 | Service/EndpointSlice/连接 | Service type 和 advertised address 混用 |
| Switchover | `switchover` | roleProbe、switchover action、roleSelector Service | candidate 成为目标 role，服务切换 | action 成功后不查角色 |
| Upgrade | `upgrade` | ComponentVersion、镜像、rollout、数据兼容 | old/new 版本和数据正确 | 只升级 runtime image |
| Backup/Restore | `backup`、`restore` | BPT、ActionSet、BackupRepo、Restore 脚本 | Backup artifact 和 Restore 成功 | 只有 Backup 无 Restore |
| RebuildInstance | rebuild 字段和 backup/restore 路径 | Restore 兼容矩阵、目标 PVC、postReady、member lifecycle | 单实例恢复并重新加入 | 新集群 restore 成功就声称 rebuild 支持 |

容易误用的 release-1.0 Ops 字段：

| 场景 | 字段 | 1.0 行为要点 | addon 验收 |
| --- | --- | --- | --- |
| scale-out 从备份扩容 | `horizontalScaling.scaleOut.fromBackup` | operations 会为新增实例准备 restore 数据 | method 必须支持该目标实例、RestoreEnv、postReady、memberJoin |
| rebuild 指定来源 | `rebuildFrom.backupName/sourceBackupTargetName/restoreEnv` | rebuild 路径把 restore 能力接入实例替换或 in-place | source target、env merge、PVC、成员重加都要验证 |
| 单实例原地 rebuild | `rebuildFrom.inPlace` | 不等价于新建 Cluster restore | 先证明数据准备和进程重启不会破坏成员关系 |
| shard 级操作 | `ops.shards` 或 sharding 相关字段 | 对象选择进入 shard Component 链路 | 不自动证明 rebalance 或跨 shard 数据一致 |
| Expose | `expose.services[*].roleSelector/podSelector/serviceType` | 只创建或更新 Service | advertised address、EndpointSlice 和客户端连接要另验 |
| force/cancel/ttl | OpsRequest 状态机字段 | 控制排队、取消和清理 | 不代表业务动作可安全中断或跳过 |

OpsRequest 还有 `force`、`enqueueOnForce`、`cancel`、TTL、precondition deadline、timeout 等状态机字段。它们只说明 operations controller 的队列和执行边界，不是业务事实。事实已经改变但 Ops 失败时，不要直接重试；先看 Cluster/Component/InstanceSet/Pod/PVC/Job、events、action 输出和业务自检，确认幂等安全。

无法由内置 Ops 自然表达的需求写入 gap，不要用隐藏脚本包装成已支持。比如复杂 rebalance、跨 topology 在线迁移、需要人工确认的数据迁移，都应明确限制。

组合操作要按“当前事实”重新判断，不要沿用上一个 Ops 的结论。典型组合包括 Reconfigure 后 Restart、Upgrade 后 Switchover、Scale-in 后 Backup、Restore 后 Rebuild、Stop 后 Delete。每次操作前先确认 Cluster/Component generation 已被观测、InstanceSet rollout 已完成、参数 desired/rendered/effective 三态一致、BackupPolicy/ComponentParameter 仍匹配当前 serviceVersion。否则后一个 Ops 的失败可能只是前一个 Ops 留下的未收敛状态，不应直接归因到 operations controller。

## 12. 常见操作排障要从入口 CR 逐层向下查

release-1.0 的运行问题通常跨多个控制器。排障顺序固定为：入口 CR 的 generation/observedGeneration/phase/message/conditions，被引用 definition/policy/action 的 Available 状态，controller 生成对象，最终 workload，脚本输出，业务自检。

问题域表：

| 问题域 | 第一批对象和字段 | 下一跳证据 | 边界 |
| --- | --- | --- | --- |
| 资源引用和 status | Cluster、Component、CMPD、CD、CMPV、PD、BPT、ActionSet 的 status | 生成的 Component、BackupPolicy、ComponentParameter、Job、events | 对象存在不等于可引用 |
| 发布迁移和 GitOps | 新旧渲染结果、server-side dry-run、live spec | 下游对象 diff | list merge、retainKeys、默认值以 server 结果为准 |
| 名字和身份 | topology name、component spec name、Component object name、Pod/PVC/Service | 引擎成员 ID、备份 target、shard identity | 不把 Pod 名或 ordinal 当业务成员 ID |
| 终止和数据保留 | terminationPolicy、deletionTimestamp、finalizers | InstanceSet、Pod、PVC、preTerminate action | 删除卡住先查执行面是否还存在 |
| 调度、PVC 和存储 | scheduling、runtimeClass、VCT、PVC retention | Pod/PVC events、StorageClass、mountPath | storage 问题常表现为创建失败 |
| Service 和网络 | Component services、Expose、ServiceRef、serviceVarRef | Service、EndpointSlice、Pod label、连接测试 | Service 有 endpoint 不等于业务 ready |
| ServiceAccount 和 RBAC | serviceAccountName、policyRules、Job/action SA | Role/RoleBinding、Pod/Job events | runtime Pod 和 Job/action 可能权限不同 |
| Pod 更新和实例身份 | podUpdatePolicy、podUpgradePolicy、InstanceSet revision | current/updatedReplicas、partition、Pod/PVC | rollout 状态不等于业务兼容 |
| Action、lifecycle 和 probe | action preCondition、target、container/image、timeout/retry | action worker、stdout/stderr、role/status | action 成功不等于业务收敛 |
| variables、env 和外部依赖 | vars、ServiceDescriptor/ServiceRef、credential/tls var | 最终 env、config/script、真实连接 | Optional、列表格式、变量刷新要验证 |
| 参数和 reconfigure | PD、PCR、ComponentParameter、OpsRequest | runtime ConfigMap、reload/restart、进程配置 | PD 匹配错先修 addon，不改 controller |
| 备份、自动备份和 artifact | BPT、BackupPolicy、BackupSchedule、Backup | target Pod、ActionSet、Repo、Job/artifact | Backup Succeed 不证明 Restore |
| Restore、scaleOut 和 rebuild | Restore、Cluster restore、RebuildInstance | prepareData/postReady、PVC、memberJoin | 不同消费者要分别验证 |
| Ops 状态和失败现场 | OpsRequest phase/message/conditions、force/cancel/TTL | Component/InstanceSet/Pod/PVC/action 输出 | Ops 字段只是证据，不是最终事实 |

操作表：

| 操作 | 先查入口对象 | 再查底层对象 | 常见 addon 问题 | 推荐处理 |
| --- | --- | --- | --- | --- |
| 创建 Cluster | Cluster topology、componentSpecs、serviceVersion、VCT | CD、CMPD、CMPV、Component、InstanceSet、Pod、Service | topology/name/volume/script 不闭合 | 修 chart 合同和 helper |
| 删除 Cluster/Component | deletionTimestamp、finalizers、terminationPolicy | InstanceSet、Pod、PVC、preTerminate | action 依赖已消失 Pod | 保留现场，先解释执行面为何消失 |
| 参数变更 | OpsRequest、PD、PCR、ComponentParameter | template ConfigMap、runtime ConfigMap、reload/restart | PCR `templateName`、PD `fileName` 或参数分类错 | 按参数对象链逐层定位 |
| restart/stop/start | OpsRequest、Component phase | InstanceSet、Pod、启动脚本 | 启动脚本不幂等 | 修脚本和 readiness |
| horizontal scale | OpsRequest、Component replicas | InstanceSet、Pod、memberJoin/memberLeave | 成员关系未收敛 | 实现 member lifecycle 或声明限制 |
| volume expansion | OpsRequest、VCT | PVC、Pod mount、文件系统、引擎容量 | 名称不一致或引擎不感知 | 先验 PVC，再验进程 |
| switchover | OpsRequest、role status | roleProbe、action、Service endpoint | roleProbe 不稳定或 candidate 处理错 | 先修 roleProbe 和脚本 |
| backup | BackupPolicy、Backup、BPT | target Pod、ActionSet、Repo、artifact | BPT 未匹配、target role 错、账号错 | 先看 BackupPolicy 生成和 target |
| restore/rebuild | Restore 或 rebuild Ops | Restore Job/PVC、postReady、目标 Component | 只验证 Backup 未验证 Restore | 每 method 单独 restore 验证 |
| upgrade | Cluster/Component serviceVersion、CMPV | resolved release、image、rollout、数据 | action/tool image 或参数 schema 未随版本更新 | 先证明版本解析和 rollout，再验数据 |

## 13. 最低验收标准

release-1.0 addon 功能不能只靠 README 声明。每个能力至少要有四类证据：实现文件、渲染对象、运行场景、观测结果。

验收层：

| 验收层 | release-1.0 需要做什么 | 能证明什么 | 不能证明什么 |
| --- | --- | --- | --- |
| chart lint/template | 用代表 values 渲染所有关键 topology、版本、TLS、备份和参数组合 | YAML 和 helper 闭合 | controller 消费后仍正确 |
| server-side dry-run | 对 examples 和关键 CR 做 server-side dry-run | API/schema/defaulting 可接受 | 业务脚本能运行 |
| 生成对象检查 | apply 后看 Component、InstanceSet、BackupPolicy、ComponentParameter、Job | controller 消费链路成立 | 业务状态已收敛 |
| live smoke | 创建 Cluster、连接服务、执行最小 Ops、Backup+Restore | runtime 和脚本可用 | 所有边界组合 |
| 故障用例 | 非法参数、缺 roleProbe、TLS on/off、target 错误、Restore 失败 | 排查路径可执行 | 自动恢复一定安全 |

最低清单：

- 关键 examples 能渲染，并且 helper、模板引用、script key、config key、compDef、BPT、PD、ActionSet 名称闭合。
- `ComponentDefinition`、`ClusterDefinition`、`ComponentVersion`、`ParametersDefinition`、`BackupPolicyTemplate`、`ActionSet` 进入可引用状态。
- 最小 Cluster 创建成功，Pod Ready，Service 可连接，脚本和配置文件存在且权限正确。
- 每个声明支持的 topology 都独立验证，而不是只验 default topology。
- 参数变更至少验证 PD/PCR/ComponentParameter、runtime ConfigMap、reload/restart 和进程内状态。
- backup method 必须至少有一次 restore 验证；没有 restore 证据时只能写“支持 backup，restore 未证明”。
- lifecycle action 必须验证目标选择、重复执行、失败输出和业务后置状态。
- upgrade 必须验证 resolved serviceVersion、runtime image、action/tool image、参数 schema、备份方法和数据兼容。
- 删除、scale-in、Stop 后删除必须验证 preTerminate/memberLeave 和 PVC/数据保留边界。

声明才验收的能力：

| 能力 | 最低证据 | 未证明时正文应如何写 |
| --- | --- | --- |
| HA/role/switchover | roleProbe、roleSelector Service、switchover action、业务角色查询 | “未证明切主”或“不支持在线切主” |
| TLS | runtime、action、backup/restore、client 全执行面 | “仅 runtime TLS 已验证” |
| 参数变更 | PD/PCR/ComponentParameter、runtime ConfigMap、进程配置 | “不支持 Ops reconfigure” |
| backup/restore | BPT、ActionSet、BackupRepo、Backup artifact、Restore 数据 | “仅 backup 已验证” |
| PITR/增量 | base/parent/timeRange、archive artifact、restoreTime 演练 | “PITR 未证明” |
| rebuild | restore 到单实例、PVC、postReady、memberJoin | “新集群 restore 不代表 rebuild” |
| upgrade | CMPV、镜像、rollout、数据兼容、回滚限制 | “升级路径未证明” |
| sharding | ShardingDefinition、shard Component、Service/vars、数据迁移限制 | “sharding 部署可用，rebalance 未证明” |
| 外部依赖 | ServiceRef declaration、Cluster binding、变量、真实连接 | “需要用户提供并重启/重建” |

最终原则：没有 release-1.0 最终对象、执行面、脚本输出和业务自检证据的能力，只能标为“不支持”“未证明”或“当前 API 缺口”，不能写成已支持。


## 14. 横向实践规则

遇到故障时，先按对应能力选择排查路径，再回到具体对象和脚本验证。

### 14.1 版本和镜像矩阵要同时覆盖 runtime 与执行面

ComponentVersion 不能只替换主容器镜像。每个 release 至少要列清楚 runtime containers、init containers、lifecycle action、backup/restore ActionSet、参数 reloader、外部工具镜像是否随 serviceVersion 变化。API 注释允许 release image key 指向 lifecycle action 字段名；addon 作者必须用渲染结果和运行时 Pod/action env 证明 key 生效，不能靠字段名猜测。

版本矩阵的验收顺序是：Cluster 或 Component 选择的 serviceVersion，匹配到的 ComponentDefinition，匹配到的 ComponentVersion release，resolved Pod image，resolved action/tool image，PD/PCR/BPT 对该 serviceVersion 的覆盖，最后是数据兼容和回滚限制。任何一步缺证据，都不能把 upgrade 写成已支持。

### 14.2 Chart 名称闭合先于业务调试

release-1.0 addon 中既有 `config/` 也有 `configs/`，既有 `config-template.yaml` 也有 `config-templates.yaml` 或 `redis-config-template.yaml`。这些文件名不是 API 语义；真正的合同是 Helm 渲染后 CMPD `spec.configs[*].name/template/volumeName`、模板 ConfigMap data key、PD `fileName`、PCR `configs[*].templateName`、scripts ConfigMap key 和容器内路径全部闭合。

如果安装、升级或 GitOps sync 失败，先比较渲染对象和 live object，不要直接改 controller。特别关注 ConfigMap 大小、helm resource policy keep、server-side schema 差异、examples namespace/name、addons-cluster values 与 addon 本体 helper 是否同步。

### 14.3 参数问题按 desired、rendered、effective 三态排查

参数链路失败时不要只看 OpsRequest phase。先确认 desired：PD/PCR 是否匹配到目标 ComponentDefinition 和文件，static/dynamic/immutable 是否互斥，用户 patch 是否进入 ComponentParameter。再确认 rendered：runtime ConfigMap 是否按模板和 desired 生成。最后确认 effective：reloadAction 或 restart 是否执行，进程内配置是否变化。

错误 desired 会持久影响后续 restart、scale 和 upgrade。修复时应先纠正 PD/PCR/templateName/fileName，再清理或重建受污染的 ComponentParameter/runtime ConfigMap；不要通过放宽 controller 匹配策略掩盖错误绑定。

### 14.4 lifecycle 和删除问题先保留执行现场

memberJoin、memberLeave、switchover、preTerminate、postStart 等 action 都是业务合同的一部分。排查时必须同时看 action target、目标 Pod 是否仍存在、kbagent/action worker 输出、脚本 stdout/stderr、role/status 和业务成员列表。action 成功只能说明脚本进程结束，不能说明成员关系或角色已经收敛。

删除卡住时，先解释为什么 finalizer 需要的执行面不存在或 action 无法完成。如果 Pod 已全部消失，应追查谁先删除了 Pod、InstanceSet/PVC 是否仍在、terminationPolicy 和 preTerminate 依赖是否合理。`skip preTerminate` 或跳过 finalizer 只能作为明确风险的运维恢复手段，不能覆盖 addon 设计错误。

### 14.5 Service、ServiceRef、账号和 TLS 要覆盖所有执行面

Service 可达不等于客户端地址正确。NodePort、LoadBalancer、hostNetwork、pod direct access、roleSelector Service、advertised address 和脚本写入的 endpoint 要一起验证。ServiceRef 要按 declaration、Cluster binding、最终 env/config、真实连接四步验收；需要提前知道其它组件实例名、跨组件共享配置或依赖刷新时，如果 API 没有自然表达，应写入 gap。

账号和 TLS 不只影响主容器启动。roleProbe、switchover、member lifecycle、backup/restore、rebuild、postReady、client examples 都要能拿到同一套凭据或证书。只证明 Secret/证书存在时，只能写“凭据/证书已生成”，不能写“数据库账号、TLS 连接和所有 action 已可用”。

### 14.6 数据保护按 method 建兼容矩阵

每个 backup method 都要独立证明 target、ActionSet 或 snapshot、BackupRepo env、artifact、format、delete 行为、restore prepareData、postReady 和业务数据校验。Backup Succeed 不能外推 Restore、PITR、RebuildInstance、跨版本、跨 topology、sharding 或 TLS 场景。

BPT `compDefs` 是 topology 覆盖矩阵的一部分。每增加一个 ComponentDefinition 或 topology，都要重新确认对应 BackupPolicy 是否生成、target role 是否存在、单副本 fallback 是否符合预期。PITR/增量链还要证明 base/full backup、parent、archive path、timeRange 和 restoreTime。

### 14.7 常见排查入口

创建失败：Cluster -> ClusterDefinition/ComponentDefinition/ComponentVersion -> Component -> InstanceSet/Pod/PVC/Service -> scripts/configs -> 业务连接。重点看 topology/component 名、volumeName/mountPath、script key/defaultMode、serviceVersion resolved image。

参数未生效：OpsRequest -> PD/PCR -> ComponentParameter -> runtime ConfigMap -> reload/restart action -> 进程内查询。重点看 `templateName`、`fileName`、参数分类和 desired 污染。

scale/switchover 失败：OpsRequest -> Component/InstanceSet -> roleProbe/member action -> Service endpoints -> 业务成员状态。重点看 target selector、candidate env、member list 和脚本幂等。

backup/restore 失败：BPT/BackupPolicy -> Backup/Restore -> ActionSet/Repo -> Job/action output -> artifact/PVC -> postReady -> 数据校验。重点看 method 兼容矩阵和 target role。

删除卡住：Cluster/Component deletionTimestamp -> finalizers -> InstanceSet/Pod/PVC -> preTerminate/memberLeave 输出 -> 外部资源。重点看执行面是否还存在，跳过 finalizer 只能作为风险恢复。

升级失败：Cluster/Component serviceVersion -> CMPV release -> Pod/action/tool image -> PD/PCR/BPT version coverage -> rollout -> 数据兼容。重点看 action/tool image 是否也版本化。


### 14.8 release-1.0 实现边界

定义对象的 status 是第一道门。ClusterDefinition、ComponentDefinition、ComponentVersion、ParametersDefinition、BackupPolicyTemplate、ActionSet 创建成功后，仍要看 observedGeneration、phase/message 和 controller event；定义对象未 Available 时，不要继续追业务脚本。

lifecycle action 不是通用 shell hook。release-1.0 中 action `preCondition` 主要用于 postProvision；不要假设所有 action 都有同等前置条件。`exec.command` 是命令数组，不是 shell 字符串；需要管道、重定向或多命令时，脚本应显式使用 `sh -c` 或独立脚本文件。

变量注入有顺序和作用域。表达式只能依赖已经解析出的变量；credential 变量用于 Pod/action env，不应假设可以直接参与普通 ConfigMap 模板渲染。ServiceRef 跨 namespace credential、legacy 字段和 serviceDescriptor/clusterServiceSelector 的优先级要用最终 env 和连接测试确认。

数据保护模板同步也有边界。BPT 生成 BackupPolicy 后，method-level target 可能覆盖全局 target；snapshot method 可以不需要 ActionSet；已有 BackupPolicy 是否跟随模板更新，要看同步注解和 controller 行为。修改 BPT 后必须重新验证目标 Cluster 上的 BackupPolicy，而不是只看模板 YAML。

## 15. 通用证据门禁和发布验收

### 15.1 四级证据门禁

每个能力都按四级证据验收：Chart 能渲染、引用对象能被 controller 消费、生成对象符合预期、数据库或工具执行面真的完成。任何一级缺失，都不要把能力写成 release-1.0 已支持；如果字段存在但生成对象或业务自检缺失，应先修 addon 合同或记录 gap/ambiguity，而不是修改 controller 让单个样例跑通。

### 15.2 名称闭合要覆盖 data key 和脚本 key

Chart 命名闭合不只检查 CR 名称，还要检查 ConfigMap 名、ConfigMap `data` key、`ParametersDefinition.spec.fileName`、`ParamConfigRenderer.spec.configs[*].templateName`、ComponentDefinition `configs[*].name`、script ConfigMap key、lifecycle command 中的脚本路径。参数链或生命周期链断开时，先找这条链的断点，不要改 controller 的匹配策略。

### 15.3 ComponentDefinition 要落到 workload、PVC 和业务 identity

ComponentDefinition 的 `services`、`podService`、`volumeClaimTemplates`、`roles`、`lifecycleActions`、`configs` 和 `vars` 都只是蓝图。创建失败时按 `ComponentDefinition -> Component -> workload/PVC/Service/ConfigMap/Job -> Pod/action output -> database self-check` 追踪。`replicasLimit`、`roleSelector`、`highWatermark` 或 `updateStrategy` 只表达 controller 可消费的意图，不自动证明 scale、切主、只读或升级的业务语义。

### 15.4 Topology、orders 和 sharding 是独立验收维度

多 topology addon 必须逐个验 `componentDef` 匹配、`serviceVersion` 解析、`orders.provision/update/terminate` 和生成 Component。Sharding 对象链能生成不等于数据自动迁移或 rebalance；如果 addon 需要自动迁移 shard 数据，但 release-1.0 没有实现证据，只能标为未证明或 gap，不能靠组合字段推导支持。

### 15.5 版本矩阵要覆盖 runtime、action 和数据保护工具

`ComponentVersion` 的验收范围至少包括 runtime 镜像、lifecycle/action 工具镜像、backup/restore 工具镜像、BPT `versionMapping`、examples 中的 `serviceVersion`。不要把 Chart `appVersion`、Chart `version` 或默认 latest 选择当成 API 合同。升级能力必须证明 runtime 与执行面镜像同步，否则只能写成单项镜像替换已验证。

### 15.6 生命周期动作要保存现场并验证业务结果

memberJoin、memberLeave、switchover、preTerminate、readonly/readwrite 这类动作都要同时验 target 选择、action image/container、脚本参数、timeout/retry、退出码、可审计输出和数据库业务结果。删除卡住时，如果 Pod 已经不存在，先查 finalizer、事件、Job/action 输出和业务成员状态；跳过 preTerminate 只能作为明确风险的恢复手段，不能掩盖 addon 脚本不可重入或成员清理合同错误。

### 15.7 ServiceRef、账号和 TLS 要按执行面建矩阵

ServiceRef、credential Secret、TLS Secret、podService 和 role service 的验证要覆盖 runtime、readiness、roleProbe、lifecycle action、backup、restore、client 连接。`optional` 只能说明依赖缺失时 controller 是否允许继续生成对象，不能证明脚本能接受空值。Secret 存在也不等于数据库用户已创建、证书路径正确或工具镜像已挂载。

### 15.8 参数问题按 desired、rendered、effective 三态收敛

release-1.0 参数排查要固定看三态：`ComponentParameter` 中的 desired、runtime ConfigMap 中的 rendered、数据库进程中的 effective。`reloadAction` 成功只能证明执行过动作；静态参数、`restartOnFileChange`、`mergeReloadAndRestart`、`deletedPolicy`、formatter 和用户模板合并都要分别验。外部任意 ConfigMap 合并不是 release-1.0 的自然能力，不能用参数链字段强行解释。

### 15.9 数据保护按 method 建兼容矩阵

每个 backup method 都要记录 target role、backup type、ActionSet、BackupRepo 需求、artifact 路径、是否可 restore、是否可 rebuild、是否可作为 incremental/differential/continuous parent。`Backup.status.phase=Completed` 只说明备份控制面完成，不能外推 artifact 完整、restore 可用或 PITR 成功。BackupRepo、BackupSchedule 和 parent backup chain 必须作为独立排查入口。

### 15.10 Day-2 操作要定义完成条件

Restart、ScaleOut、ScaleIn、VerticalScaling、VolumeExpansion、Switchover、Rebuild、Upgrade 的完成条件不能只看 OpsRequest phase。每个操作都要继续查 Component/workload/PVC/Job/action output 和数据库状态：成员列表、读写角色、参数生效、磁盘识别、数据目录、工具镜像版本等。无法通过 release-1.0 API 自然表达的操作前置条件或回滚语义应进入 gap。

### 15.11 排障要有停止条件

排障路径从入口 CR 开始，但不能停在入口 CR。常用停止条件是：引用对象不存在或未 Available，生成对象缺失，action/Job 明确失败，业务自检无法证明，或需求本身超出 release-1.0 API。到达这些停止条件后，应修 addon chart/script、补验收或记录 gap/ambiguity，不要为了单个失败样例修改 controller 合同。

### 15.12 发布验收按能力声明组织

发布验收不是按文件数量组织，而是按能力声明组织。每项声明能力至少覆盖：Helm render、CRD/schema、controller 生成对象、live smoke、失败路径和限制边界。声明支持 TLS、backup、restore、reconfigure、scale、upgrade、ServiceRef 时，都要给出对应的最小正反例；声明不支持时也要写清楚，避免后续 agent 拼 workaround。

## 16. 高风险能力验收矩阵

字段存在但消费者不完整、语义不清或缺少真实 addon 证据的，只能写成“未证明”“需要验证”或进入 gap/ambiguity。

### 16.1 排障和验收要到达“下一跳”和“停止条件”

release-1.0 的排障不能只列入口对象。每类问题都要写清楚下一跳对象、停止条件和推荐处理，避免在最后一个报错处直接修改 controller。

| 问题域 | 入口 | 下一跳 | 停止条件 | 推荐处理 |
| --- | --- | --- | --- | --- |
| 资源引用和 status | Cluster、Component、CMPD、CD、CMPV、PD、BPT、ActionSet | 引用方对象、`observedGeneration`、phase/message、event | 引用对象不存在、未 Available、generation 未追上 | 修 chart 引用或等待/重建定义对象，不继续调业务脚本 |
| 发布迁移和 GitOps | 新旧 Helm 渲染、server-side dry-run、live spec | Component、InstanceSet/PVC、BackupPolicy、BackupSchedule、ComponentParameter | server 最终对象与本地渲染不一致 | 以 apiserver/defaulting 和 live object 为准，写明保留/迁移/删除策略 |
| 名字和身份 | topology name、component name、Component object name、Pod/PVC/Service name | 引擎成员 ID、备份 target、shard identity、持久数据里的 identity | 名称能渲染但业务 identity 不一致 | 修脚本 identity 映射，不把 ordinal 或 Pod 名当稳定业务身份 |
| 终止和数据保留 | deletionTimestamp、finalizers、terminationPolicy | InstanceSet、Pod、PVC、preTerminate/memberLeave action、Backup/BackupRepo | action 所需 Pod 或 PVC 已不存在 | 先解释执行面为何消失；跳过动作只能作为风险恢复，不算删除流程验收 |
| 调度、PVC 和存储 | scheduling、runtimeClass、VCT、PVC retention | Pod/PVC event、StorageClass、mountPath、文件系统、数据库容量查询 | PVC 已扩容但进程未识别 | 先验 K8s 存储，再验引擎内部容量刷新 |
| Service 和网络 | CMPD services、Expose、ServiceRef、serviceVarRef、podService、hostNetwork | Service、EndpointSlice、Pod label、注入 env/config、连接测试 | Service 有 endpoint 但业务连接失败 | 修 Service selector、地址格式、advertised address 或脚本消费逻辑 |
| ServiceAccount 和 RBAC | `serviceAccountName`、`policyRules`、Restore/Backup serviceAccountName | Pod/Job 使用的 SA、Role/RoleBinding、event | runtime Pod 有权限但 action/backup/restore Job 无权限 | 分执行面补 RBAC；不要假设主 Pod 权限自动继承到 worker |
| Action、lifecycle 和 probe | action preCondition、target、handler、container/image、timeout/retry | action worker、volumeMount、stdout/stderr、exit code、role/status、业务状态 | action exit 0 但业务状态未变 | 修脚本幂等和后置校验，不把 timeout/retry 当语义证明 |
| variables、env 和外部依赖 | CMPD vars、Cluster serviceRefs、ServiceDescriptor、credential/tls var | 最终 env、rendered config/script、真实连接 | Optional 缺失、列表格式或 credential 解析不符合脚本预期 | 写明变量格式和缺失行为；API 无法表达的刷新/共享需求进 gap |
| 参数和 reconfigure | OpsRequest、PD、PCR、ComponentParameter | template ConfigMap、runtime ConfigMap、reload/restart、进程配置 dump | desired/rendered/effective 任一层断开 | 按三态定位；PD/PCR 绑定错先修 addon，不改匹配策略 |
| 备份和 artifact | BPT、BackupPolicy、BackupSchedule、Backup | method、target Pod、ActionSet、BackupRepo、Job、artifact、formatVersion | Backup Completed 但 artifact/restore 未验 | 只能写 backup 已完成；restore/rebuild/PITR 仍需独立矩阵 |
| Restore、scaleOut 和 rebuild | Restore、HorizontalScaling `scaleOut.fromBackup`、RebuildInstance | prepareData、postReady、PVC、sourceBackupTargetName、memberJoin、Service endpoint | 恢复数据写入但目标实例未加入业务拓扑 | 区分新集群 restore、scaleOut 和 rebuild，分别验收 |
| Ops 状态和失败现场 | OpsRequest phase/message/conditions、force/cancel/TTL | Component、InstanceSet/Pod/PVC、Job/action output、业务自检 | Ops phase 与业务状态不一致 | 以底层对象和业务自检为准，保留失败现场后再决定重试或修 addon |

### 16.2 Restore、Rebuild 和 scaleOut 要按消费者建矩阵

release-1.0 同时存在 `Restore`、`RebuildInstance` 和 `HorizontalScaling.scaleOut.fromBackup` 这些消费者。它们可以共享 Backup、ActionSet 和 artifact，但不是同一个能力。一个 method 能 restore 到新集群，不等于能 rebuild 原实例，也不等于能用于 scaleOut 新副本。

| 消费者 | release-1.0 入口 | 必须证明的对象链 | addon 必须自证的业务结果 | 不能外推的结论 |
| --- | --- | --- | --- | --- |
| 新集群 Restore | `dataprotection.kubeblocks.io/v1alpha1 Restore` | Backup、ActionSet restore、prepareData、ReadyConfig/postReady、Restore Job/PVC | 目标 Cluster/Component ready，数据可读写，账号/TLS 可用 | 不证明 RebuildInstance、scaleOut 或跨 topology restore |
| PITR Restore | Restore `restoreTime` + continuous/PITR backup | base backup、parent chain、timeRange、archive artifact、restoreTime 格式 | 恢复到目标时间点，业务数据符合预期 | 不证明任意时间点都有可用归档 |
| K8s 资源恢复 | Restore `resources.included` | 被 include 的 GroupResource、labelSelector、恢复后的 K8s 对象 | 资源对象恢复成功 | 不等于数据库数据恢复成功；exclude 仍是 TODO 边界 |
| RebuildInstance | OpsRequest `rebuildFrom` | backupName、sourceBackupTargetName、restoreEnv、目标 instance、inPlace/replace、postReady/memberJoin | 被 rebuild 实例数据正确并重新加入复制组 | 不证明逻辑备份可用于多副本 rebuild；API 注释已限制多副本更适合 full physical backup |
| scaleOut.fromBackup | OpsRequest `horizontalScaling.scaleOut.fromBackup` | backup、newInstances/replica change、restore prepareData、memberJoin | 新实例从备份拉起并加入成员列表 | 字段注释说明只对非 sharding component 有效；不证明 shard scale 或 rebalance |
| rebuild 后补偿动作 | 真实 addon 的 post-rebuild 脚本或 OpsDefinition | post-rebuild action、目标 instance、业务元数据 | 引擎内部元数据完成修复 | 自定义或 addon 私有补偿不是通用 KB API 合同 |

数据保护 method 的兼容矩阵至少包含这些列：method name、backup type、target role、ActionSet、BackupRepo 需求、artifact 类型、是否支持新集群 restore、是否支持 PITR、是否支持 RebuildInstance、是否支持 scaleOut.fromBackup、是否支持 sharding/topology 变化、需要的账号/TLS/env、已验证 addon 示例。没有验证的格子写“未证明”，不要留空让后续 agent 猜。

### 16.3 网络、Service、RBAC 和 worker 执行面要分开验

release-1.0 的主 Pod、lifecycle action、backup Job、restore Job、postReady Job 不是同一个执行面。主 Pod 能启动、Service 能连通、Secret 能读取，都不能自动外推到其它 worker。

| 执行面 | 需要检查 | release-1.0 证据入口 | 常见错误 |
| --- | --- | --- | --- |
| runtime Pod | PodSpec、volumeMount、env、SA、ports、hostNetwork、hostNetworkVarRef | CMPD runtime、生成 Pod、真实 addon 中 Redis/MongoDB hostNetwork 样例 | 脚本读取 host IP/port，但 Service 或 env 格式不同 |
| Service/Endpoint | Service type、selector、roleSelector、podService、EndpointSlice、publish/not-ready 行为 | CMPD services、生成 Service/EndpointSlice、Kafka/Redis/Etcd podService 样例 | Service 有 endpoint 但 role 不对或 advertised address 不对 |
| ServiceRef | declaration、Cluster binding、ServiceDescriptor、credential、namespace | `pkg/controller/component/service_reference.go`、ServiceDescriptor controller | 跨 namespace credential 被禁止或 Secret key 未校验到业务可用 |
| lifecycle action | target、container/image、volumeMount、SA/RBAC、timeout/retry | lifecycle executor、kbagent、action output | 默认容器或工具镜像与 runtime 版本不一致 |
| backup worker | BackupPolicy serviceAccountName、ActionSet env、BackupRepo mount/tool secret | Backup/ActionSet/BackupRepo controller 和 Job | runtime TLS/账号未传递到 backup 工具 |
| restore worker | Restore serviceAccountName、prepareData scheduling、ReadyConfig、postReady | Restore API 和 restore manager | prepareData 成功但 postReady 或业务连接失败 |
| RBAC | Component `policyRules`、runtime SA、Job/action SA | final Pod/Job SA、Role/RoleBinding、event | 组件 Pod 有权限，restore/action Job 没权限 |

ServiceDescriptor 在 release-1.0 中会校验基础字段和 SecretRef 是否存在，但实现里没有验证 Secret data key 的完整性。因此不能把 “ServiceDescriptor Available” 当成外部服务凭据可用证明；必须继续验证最终 env/config 和真实连接。ServiceRef 的 credential 跨 namespace 也不是任意可用能力，代码会禁止跨 namespace credential 变量引用；需要跨 namespace 外部依赖时，应使用明确的 ServiceDescriptor 和权限/凭据交付方式，并声明限制。

### 16.4 高风险能力最低验收清单

release-1.0 addon 声明相关能力时，至少提供下面这些证据：

- 排障验收：至少能对创建、删除、reconfigure、scale、switchover、backup、restore/rebuild、upgrade 各给出入口 CR、下一跳对象、停止条件和推荐处理。
- Restore/Rebuild/scaleOut 验收：每个 backup method 至少填一张消费者矩阵；未证明的消费者显式写“未证明”。
- 网络和 worker 验收：runtime Pod、lifecycle action、backup Job、restore Job、postReady Job 分别检查 env、SA/RBAC、volumeMount、TLS/credential、工具镜像和业务连接。
- Controller 修改边界：如果一个问题只能通过修改 KB controller 才能让单个 addon 跑通，但 API 合同、生成对象或业务自检证据不足，应先判定为 addon 合同问题、ambiguity 或 gap。
