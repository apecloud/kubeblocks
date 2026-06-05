# KubeBlocks Addon 开发实践指南 release-1.1

适用版本：KubeBlocks release-1.1 和 `kubeblocks-addons` release-1.1。

适用对象：addon 开发者和负责开发 addon 的 agent。核心样本包括 MySQL、PostgreSQL、Redis、ClickHouse、Kafka、Etcd；扩展样本包括 MongoDB、Elasticsearch、Milvus、MinIO、Pulsar、RabbitMQ。

## 1. 各环节都要先写清楚合同

addon 开发不是把 YAML 字段填满，而是为每个能力建立可验证合同。release-1.1 的核心 apps API 使用 `apps.kubeblocks.io/v1`，主要包括 `Cluster`、`ComponentDefinition`、`ClusterDefinition`、`ComponentVersion`、`ShardingDefinition`、`ServiceDescriptor`。参数、数据保护和运维主要使用 `parameters.kubeblocks.io/v1alpha1`、`dataprotection.kubeblocks.io/v1alpha1`、`operations.kubeblocks.io/v1alpha1`。

实践判断以四类证据闭环：addon 声明、controller 消费点、运行时对象、业务自检结果。四类证据缺一类时，只能把结论写成待验证、限制或 gap。

| 环节 | addon 必须声明的合同 | release-1.1 controller 主要消费点 | 常见误判 | 先查什么 |
| --- | --- | --- | --- | --- |
| Chart 组织 | helper 统一生成名称、label、模板引用、版本前缀、脚本和配置 key | Helm 渲染后的 CR、server-side dry-run 后的对象 | 一个 values 能渲染就认为所有 topology 闭合 | `helm template`、server dry-run、最终引用关系 |
| 组件蓝图 | runtime、volumes、configs、scripts、services、roles、vars、accounts、TLS、lifecycle、RBAC | `controllers/apps/component` 和 synthesized component | 把动态副本、资源、存储容量写死到蓝图里 | CMPD status、Component、InstanceSet/Pod/PVC/Service |
| 拓扑 | topology、components、shardings、orders、default | `controllers/apps/cluster` normalization 和 topology 解析 | 未设置 default 却假设选第一个 | Cluster resolved topology、生成的 Component 列表 |
| 版本管理 | CMPD 名称、serviceVersion、CMPV releases、镜像 key、工具镜像矩阵 | ComponentDefinition matching、ComponentVersion compatibility、workload image override | 宽正则能命中就等于版本治理完成 | CMPV status、resolved serviceVersion、Pod/action/tool image |
| 配置参数 | CMPD config name、模板 ConfigMap、PD fileName、PCR configs、schema、reload/restart 分类 | `controllers/parameters`、ComponentParameter、runtime ConfigMap | `templateName`、模板 ConfigMap 名、runtime ConfigMap 名混用 | PD、PCR、ComponentParameter、runtime ConfigMap、进程内值 |
| 变量和服务 | vars 来源、Service、ServiceRef、ServiceDescriptor、外部依赖、地址格式 | var resolver、service builder、ServiceDescriptor controller | 脚本私下约定变量格式 | 最终 Pod/action/backup/restore env、Service、EndpointSlice、连接自检 |
| 账号和 TLS | systemAccounts、accountProvision、credential vars、证书挂载、worker 权限 | component validation、action executor、backup/restore job、脚本 | 账号 Secret 存在就等于数据库内权限可用 | Secret、accountProvision 输出、Job/Pod SA、实际连接和权限 |
| lifecycle | roleProbe、availableProbe、memberJoin/Leave、switchover、preTerminate、dataDump/dataLoad | lifecycle executor、kbagent、component deletion/scale、Ops switchover | 脚本退出 0 等于业务状态收敛 | action 输入、target Pod、role/status、业务成员状态 |
| 内置 Ops | 每种 Ops 的底层前提、目标对象和完成条件 | `controllers/operations` 和 `pkg/operations` | OpsRequest Succeed 等于业务完成 | OpsRequest、Component、InstanceSet/PVC/Service、脚本和业务状态 |
| 数据保护 | BackupPolicyTemplate、ActionSet、BackupRepo、method、target、脚本、artifact | `controllers/dataprotection` 和 `pkg/dataprotection` | 只有 ActionSet 就声称支持备份 | BackupPolicy 是否生成、target/actionSet/repo/artifact/restore |
| Restore/Rebuild | source backup、target mapping、prepareData、postReady、账号/TLS 兼容 | restore manager、operations rebuild flow、restore jobs | Backup 成功等于 Restore 成功 | Restore Job/PVC、目标 Component、memberJoin、数据健康检查 |
| Upgrade/Rollout | serviceVersion、ComponentVersion、镜像、rollout、数据兼容 | ComponentVersion、workload rollout、operations upgrade | runtime image 能启动就等于升级支持 | old/new serviceVersion、Pod image、action image、BPT/PD 版本 |
| Sharding | ShardingDefinition、shard template、shardsLimit、shard lifecycle | sharding/component transformers | 多副本等于 sharding | ShardingDefinition、shard Component、Service/vars、数据迁移边界 |
| 操作排障 | 创建、删除、参数、scale、backup/restore、upgrade 的对象链 | cluster/component/operations/dataprotection/parameters 控制器 | 只看最后一个报错 | 从入口 CR 到生成对象再到脚本和业务状态 |

每增加一个能力，都要能说明对象关系、controller 消费方式、运行验证和限制边界。控制器能接受字段，只说明 control plane 有入口；addon 能否作为产品能力交付，取决于最终对象、执行面和业务自检是否闭合。

## 2. 先定义 addon 的工作边界

release-1.1 的 addon 首先是 Helm chart。真实仓库里有三类入口：`addons/<name>` 放 KubeBlocks 定义对象和脚本，`addons-cluster/<name>` 放可安装的 Cluster chart，`examples/<name>` 放用户 CR 示例。定义对象以 `addons/<name>` 为主；验收用户路径时必须同时看 `addons-cluster/<name>` 和 `examples/<name>`。

真实 addon 通常包含 `Chart.yaml`、`values.yaml`、`templates/`、配置模板目录、脚本目录、脚本单测目录和 examples。历史目录名不完全统一；新 addon 应固定单复数和文件命名，避免 `config/configs`、`config-template.yaml/config-templates.yaml`、脚本 ConfigMap 和脚本源码之间反复猜测。

命名应按职责稳定下来：定义对象放在 `templates/`，配置源文件放在 `config/`，执行脚本放在 `scripts/`，用户安装入口放在 `addons-cluster/<engine>`，可直接验收的示例放在 `examples/<engine>`。模板文件名建议使用单数资源语义，例如 `clusterdefinition.yaml`、`componentdefinition-<component>.yaml`、`componentversion-<component>.yaml`、`parametersdefinition-<component>.yaml`、`parameterconstraint-<component>.yaml`、`config-template-<component>.yaml`、`script-<component>.yaml`、`actionset.yaml`、`backuppolicytemplate.yaml`。脚本源码名应表达动作语义，例如 `start.sh`、`role-probe.sh`、`member-join.sh`、`member-leave.sh`、`switchover.sh`、`reconfigure.sh`、`backup.sh`、`restore.sh`。

代表性 release-1.1 addon 样本：

- MySQL：`addons/mysql/templates/clusterdefinition.yaml`、`cmpd-mysql80.yaml`、`cmpd-mysql84.yaml`、`cmpd-proxysql.yaml`、`cpmv.yaml`、`backuppolicytemplate.yaml`、`actionset-xtrabackup.yaml`、`paramsdef-80.yaml`、`pcr-80.yaml`。
- PostgreSQL：`addons/postgresql/templates/cmpd.yaml`、`clusterdefinition.yaml`、`cmpv.yaml`、`paramsdef.yaml`、`pcr.yaml`、`backuppolicytemplate.yaml`、`actionset-wal-g.yaml`、`actionset-pgbasebackup.yaml`。
- Redis：`addons/redis/templates/clusterdefinition.yaml`、`cmpd-redis.yaml`、`cmpd-redis-cluster.yaml`、`shardingdefinition.yaml`、`cmpv-redis.yaml`、`backuppolicytemplate.yaml`、`backupactionset.yaml`、`paramsdef-redis.yaml`、`pcr-redis.yaml`。
- ClickHouse：`addons/clickhouse/templates/clusterdefinition.yaml`、`shardingdefinition.yaml`、`cmpd-ch.yaml`、`cmpd-keeper.yaml`、`cmpv.yaml`、`paramsdef-config.yaml`、`pcr-ch.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。
- Kafka：`addons/kafka/templates/clusterdefinition.yaml`、`cmpd-broker.yaml`、`cmpd-controller.yaml`、`cmpd-combine.yaml`、`cmpv-broker.yaml`、`cmpv-controller.yaml`、`paramsdef.yaml`、`pcr.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。
- Etcd：`addons/etcd/templates/cmpd.yaml`、`cmpv.yaml`、`backuppolicytemplate.yaml`、`actionset.yaml`。
- 扩展样本：MongoDB 使用 `cmpd-config-server.yaml`、`cmpd-mongodb-shard.yaml`、`cmpd-mongos.yaml`、`shardingdefinition.yaml` 和多 ActionSet；Elasticsearch 拆 6/7/8 版本 ComponentDefinition 和 `cmpv-es.yaml`、`cmpv-kibana.yaml`；Milvus 拆 standalone/data/index/query/proxy/minio/mixcoord；Pulsar 拆 bookkeeper/broker/proxy/zookeeper/bkrecovery；MinIO 和 RabbitMQ 是轻量单组件样本。

chart 层先做闭合检查，再谈业务语义。至少用代表性 values 渲染 standalone、HA、proxy、sharding、TLS on/off、不同 serviceVersion、备份恢复开关等组合，确认 helper、ConfigMap key、script key、`templateName`、`compDef`、BPT `compDefs`、PD `fileName`、ActionSet 名称、examples 中的名字全部闭合。

发布和 GitOps 验收要保存三类结果：旧版渲染、新版渲染、server-side dry-run 或 live object diff。release-1.1 API 中多处 list 字段使用 merge/retainKeys，例如 Cluster component 的 `volumeClaimTemplates` 和 OpsRequest 的 `volumeClaimTemplates`；不能只用文本 diff 判断最终对象。

Chart 默认 values 也属于合同。默认 CPU、memory、storage、image registry、pull secret、TLS 开关、backup repo 名称、serviceVersion、topology、terminationPolicy 都要能生成可运行对象；默认值只适合 demo 时必须在 README 和 examples 中写清楚生产限制。镜像 registry、工具镜像、init container、backup/restore image 和 reloader/action image 要同时可配置，不能只让 runtime image 可替换。

`values.schema.json` 只能作为用户输入约束的一部分。它能提前拦截类型、枚举、必填和范围错误，但不能证明 helper 生成的对象能被 apiserver 接受，也不能证明 controller 会按预期消费。schema、values、templates、examples 四者要一起维护：schema 允许的 topology、serviceVersion、storage class、TLS、backup method、expose service type 都必须有对应模板分支和示例；模板分支存在但 schema 不允许，用户入口仍然不可用。

发布验收按三层 diff 做。第一层是 chart render diff，看 helper、labels、ownerRef、ConfigMap data key、script key、templateName、compDefs、serviceVersion 是否变化；第二层是 server-side dry-run/diff，看 CRD schema、默认值、merge/retainKeys 和不可变字段；第三层是 live object diff，看 Component、InstanceSet、PVC、Service、BackupPolicy、ComponentParameter、Job 是否被 controller 重新生成或保留。只有三层都能解释，才说明发布变化可审计。

Helm uninstall、feature disable 和 chart 升级要区分定义对象和运行对象。删除一个模板文件不等于集群中对应 CR、Role、ServiceAccount、BackupPolicy、BackupSchedule、PVC 或 repo artifact 已被安全处理；保留策略、手工迁移步骤和不可回滚字段要在发布说明中写清楚。定义对象被删除后，已有 Cluster 是否继续引用旧对象、是否需要先升级 Cluster spec、是否需要保留模板 ConfigMap，都要在 live diff 中验证。

## 3. 用 ComponentDefinition 表达组件蓝图

release-1.1 的 `ComponentDefinition` 是组件静态蓝图。它描述 runtime PodSpec、volumes、configs、scripts、services、vars、systemAccounts、TLS、roles、lifecycleActions、ServiceRefDeclarations、update 策略、hostNetwork 和 RBAC。Cluster 的 `componentSpecs` 提供实例化时的动态输入，例如副本数、资源、存储容量、调度、TLS 开关、serviceRefs、serviceVersion、外部服务绑定、用户覆盖配置和 instances。

最小组件要能闭合以下关系：

- `spec.runtime.containers[*].volumeMounts[*].name` 必须能在 `spec.volumes`、`spec.configs[*].volumeName`、`spec.scripts[*].volumeName` 或 runtime volumes 中找到。
- `spec.configs[*].name` 是配置 item 名称和参数链路绑定键；`spec.configs[*].template` 是模板 ConfigMap 名；`spec.configs[*].volumeName` 决定挂载到哪个 volume。
- `spec.scripts[*].template` 提供脚本 ConfigMap；`defaultMode` 要让容器内脚本可执行，并以最终 Pod spec 和容器内文件权限为准。
- `spec.services[*].roleSelector` 依赖 `spec.roles` 和 `lifecycleActions.roleProbe`；`podService`、`disableAutoProvision`、Expose Ops 和 ServiceRef 变量要分别验收。
- `systemAccounts`、TLS、vars、policyRules 要能在启动脚本、lifecycle action、backup/restore、ServiceRef 或外部连接里找到使用点。
- `status.phase=Available` 是被 Cluster、BPT、PD、ComponentVersion 等引用前的最低门槛。CR 创建成功不是可引用证明。

字段语义矩阵：

| 字段或对象关系 | release-1.1 实践语义 | 必须同时对齐 | 常见误用 |
| --- | --- | --- | --- |
| `spec.serviceKind` | 组件提供的服务类型 | BPT `serviceKind`、ServiceDescriptor、README | 大小写或命名不统一导致依赖和策略不匹配 |
| `spec.serviceVersion` | 组件内核版本；可被 ComponentVersion release 覆盖 | ComponentVersion、PD serviceVersion、BPT versionMapping、examples | 未显式写 serviceVersion，误以为选择固定版本 |
| `spec.runtime` | 静态 PodSpec | volumes、configs、scripts、ports、command | 把资源和调度这类实例参数写死 |
| `spec.volumes[*].name` | 数据卷稳定身份 | Cluster VCT、runtime mount、BPT targetVolumes、volume expansion | PVC 创建了但没有挂到数据库目录 |
| `spec.volumes[*].needSnapshot` | volume snapshot backup 候选数据卷 | BPT method `snapshotVolumes`、StorageClass snapshot 能力 | 声称支持 snapshot，但 method 和 volume 没绑定 |
| `spec.volumes[*].highWatermark` | 容量阈值和 readonly/readwrite action 的触发依据 | lifecycle `readonly/readwrite`、引擎真实只读状态 | 字段存在但脚本无法让业务进入或退出只读 |
| `spec.configs[*].name` | 配置模板身份和参数绑定键 | PCR `configs[*].templateName`、Cluster configs、ComponentParameter | 写成模板 ConfigMap 名 |
| `spec.configs[*].template` | 模板 ConfigMap 名 | chart 渲染出的 ConfigMap | 模板 ConfigMap data 没有 PD `fileName` |
| `spec.configs[*].externalManaged` | 配置交给 parameters 链路管理 | PD/PCR/ComponentParameter/runtime ConfigMap | 误以为可以任意指向底层 runtime ConfigMap |
| `spec.scripts[*].template` | 脚本 ConfigMap 名 | command/action 脚本路径、defaultMode | 脚本存在但不可执行或 action 看不到 |
| `spec.services[*].roleSelector` | 按 role 暴露服务 | roles、roleProbe、Service endpoints | 没有 roleProbe 却声明 roleSelector |
| `spec.services[*].podService` | 每个 Pod 一个 Service | serviceVarRef 输出格式、外部访问脚本 | 误以为普通 roleSelector 仍然生效 |
| `spec.vars` | 注入 Pod/action 或渲染模板的运行时变量 | 脚本消费格式、Optional/Required 分支 | 脚本依赖未声明或 stale 的变量格式 |
| `spec.systemAccounts` | KB 管理的系统账号 | accountProvision、credentialVarRef、backup/probe/replication 脚本 | Secret 存在但数据库内账号未创建 |
| `spec.tls` | TLS 文件形状和挂载路径 | Cluster TLS、启动配置、client、backup/restore job | 只验证 runtime，不验证其它执行面 |
| `spec.roles` | roleProbe 输出集合和 update/service/backup 的角色基础 | roleProbe、updateStrategy、BPT target | role label 等同于业务一致性 |
| `spec.lifecycleActions` | 组件生命周期脚本和 probe | action target、kbagent、Ops、delete/scale | action 成功等同于业务收敛 |
| `spec.serviceRefDeclarations` | 外部依赖声明 | Cluster serviceRefs、serviceRefVarRef | 只声明依赖，没有 Cluster 绑定示例 |
| `spec.replicasLimit` | 声明 scale 支持边界 | README、HorizontalScaling、examples | 声称支持边界但没有业务验证 |
| `spec.available` | Component Available 判定策略 | readiness、roleProbe、availableProbe | Pod Ready 与业务可用混为一谈 |
| `spec.hostNetwork` | hostNetwork 端口和 DNS 行为 | container ports、hostNetworkVarRef、advertised address | 启用 hostNetwork 后脚本仍用固定端口 |
| `podUpdatePolicy`、`podUpgradePolicy`、`instanceUpdateStrategy` | 变更和升级 rollout 方式 | role priority、quorum、InstanceSet revision | 升级顺序破坏可用性 |
| `policyRules` | runtime/action 可能需要的 RBAC 权限 | Pod/Job ServiceAccount、Role/RoleBinding | runtime 有权限，restore/action worker 没权限 |

release-1.1 controller 会做一部分定义校验，例如 service 端口、roleSelector、account/lifecycle、file template、hostNetwork 和 availableProbe，但校验层不是完整业务证明。`roles` 与 `roleProbe` 的绑定、账号权限、TLS 工具链、脚本幂等和业务 member 状态仍要由 addon 的示例和测试自证。

写完 ComponentDefinition 后，必须立刻写最小 Cluster 样例证明闭环：Cluster 解析 topology 或 direct componentDef，生成 Component，Component 继续生成 InstanceSet/Pod/PVC/Service，Pod 内脚本和配置文件存在且权限正确。不要等到参数、备份和升级都写完后才发现基础蓝图未闭合。

最小验证链路固定看这些对象：Cluster spec/status，Component spec/status，引用到的 ComponentDefinition/ComponentVersion/ClusterDefinition，InstanceSet/Pod/PVC/Service，Pod events，容器内脚本和配置文件。Component `Running` 不等于 InstanceSet 全部 Available；Pod Ready 后还可能受 `minReadySeconds`、availableProbe、roleProbe 或 rollout 策略影响。

数据目录初始化、默认资源和文件权限属于 ComponentDefinition 合同。启动脚本至少要区分空目录、只有 `lost+found` 的新 PVC、已有业务数据、restore/rebuild 写入的数据和半初始化目录；不要用简单的“目录为空/非空”决定是否初始化或清空数据。`defaultMode` 要按最终 Pod spec 和容器内 `stat` 验收，不能只看源码里写了一个看起来像八进制的数字。

`replicasLimit` 是 scale 支持范围声明，不是业务已经支持所有边界的证明。声明后至少要验最小值、最大值、越界拒绝、HorizontalScaling Ops、README 示例和 0 副本语义。若引擎在 `replicas=0` 时没有可用入口、不能保留复制成员语义或不支持从 0 恢复，文档必须写成不支持或需要人工步骤。

实例身份要从 Component 往下看，而不是靠名字猜。release-1.1 使用 InstanceSet 承载工作负载，Pod/PVC/Service 的名字、label、ownerRef、ordinal 和引擎内部成员 ID 不是同一种身份。scale、rebuild、restore、同名 Cluster 重建和 sharding 场景下，脚本可以读取 Pod 名或 FQDN 作为运行时输入，但不要把它们写入不可重算的持久成员 ID。

## 4. 用 ClusterDefinition 表达拓扑，不表达组件细节

release-1.1 的 `ClusterDefinition` 主要表达 topology：每个 topology 包含 components、shardings、orders 和 default。组件 runtime、脚本、账号、TLS 不应该放在 ClusterDefinition 里。

实践规则：

- 每个用户可选部署形态都应有明确 topology。MySQL 有 `semisync`、`mgr`、`orc`、`*-proxysql`；Redis 有 `standalone`、`replication`、`cluster`；Kafka 有 broker/controller/combine；MongoDB 有 replicaSet 和 sharding；ClickHouse 有 sharding 和 keeper。
- `default: true` 要显式设置。Cluster 未指定 topology 时应依赖默认 topology，不应让用户猜测。
- `components[*].name` 是 topology 内组件名，Cluster `componentSpecs[*].name` 要和它对齐。
- `components[*].compDef` 支持精确名、前缀或正则。addon 应使用 helper 固定规则并用渲染结果验证，避免宽正则误命中未来或残留版本。
- `orders.provision/terminate/update` 表示跨组件顺序。proxy、sentinel、keeper、controller/broker 这类辅助组件要显式排序。
- sharding 不等于 replicas。`replicas` 是一个复制组内实例数，`ShardingDefinition` 表示多个 shard component。

拓扑能力矩阵：

| 拓扑能力 | release-1.1 表达方式 | controller 消费点 | 验收证据 | 不支持或未证明边界 |
| --- | --- | --- | --- | --- |
| 单组件 standalone | `ClusterDefinition.spec.topologies[*].components` | Cluster normalization | Component、InstanceSet、Pod、Service 全部生成 | 不证明 HA、backup、reconfigure |
| 多组件 HA/proxy | topology components + orders | cluster/component reconcile 顺序 | server 先于 proxy 创建，proxy 先删除；服务可连接 | proxy 路由表和后端角色要另验 |
| MGR/orc/sentinel/keeper | 不同 topology 指向不同 compDef | compDef matching | 每个 topology 的 CMPD/CMPV/PD/BPT 都闭合 | 一个 topology 成功不代表另一个成功 |
| sharding | `ShardingDefinition` + topology `shardings` | sharding/component transformer | shard Component、Service、vars、scale 行为 | 不自动证明 rebalance 或数据迁移 |
| direct componentDef | Cluster component 直接指定 ComponentDefinition，如果使用 | Cluster normalization | 不依赖 ClusterDefinition orders | 不应和 topology 语义混用 |

`ShardingDefinition` 的语义要按对象链理解：`spec.template.compDef` 指向每个 shard 使用的 ComponentDefinition，`shardsLimit` 约束 shard 数量，`provisionStrategy/updateStrategy` 和 lifecycle `postProvision/preTerminate/shardAdd/shardRemove` 只表达 shard 组件的创建、更新和增删动作。它不自动表达 Redis slot 迁移、ClickHouse 分布式表修复、Kafka partition rebalance 或跨 shard restore。

排查 topology 时按这个顺序：Cluster `spec.clusterDef/topology/componentSpecs/shardings`，ClusterDefinition status 和 topology，匹配到的 ComponentDefinition/ComponentVersion，生成的 Component，InstanceSet/Pod/Service。不要把 topology component name、generated Component object name、Pod name、PVC name 和引擎成员 ID 混为一谈。

每个 topology 都要作为验收切片。新增一个 topology 后，至少要确认 CMPD/CMPV/PD/PCR/BPT/ActionSet/Service/roleProbe/examples 都覆盖该 topology；不能只验证 Cluster Running。proxy/router 类 topology 还要验证路由表、账号权限、roleSelector Service 和业务读写路径。

## 5. 用 ComponentVersion 管理版本矩阵

release-1.1 有 `apps.kubeblocks.io/v1` 的 `ComponentVersion`。它用 `compatibilityRules[*].compDefs` 把一组 ComponentDefinition 和若干 release 关联起来；每个 release 有 `name`、`serviceVersion` 和 `images`。release 的 `serviceVersion` 会作为被选择 release 的 serviceVersion，覆盖 ComponentDefinition 自身定义的 serviceVersion。

版本管理至少分五层：

| 版本维度 | release-1.1 来源 | 被谁消费 | 必须验证什么 |
| --- | --- | --- | --- |
| chart version | `Chart.yaml` | Helm/package/release 管理 | 与模板、values、README 一致 |
| ComponentDefinition name | CMPD metadata/helper | ClusterDefinition、BPT、PD、CMPV | 正则只命中预期版本 |
| `spec.serviceVersion` | CMPD 或 CMPV release | Cluster normalize、PD、BPT versionMapping、upgrade | resolved serviceVersion 是预期版本 |
| runtime image | CMPD runtime 或 CMPV `images` | Pod/InstanceSet | 实际 Pod image 被替换 |
| action/tool image | lifecycle action、ActionSet、BPT env | action worker、backup/restore Job | 工具版本与 serviceVersion 兼容 |

实践规则：

- ComponentDefinition 名称应带引擎大版本或 addon 版本前缀，并保持 helper 可预测。BPT、PD、PCR、ClusterDefinition、examples 不应各自手写不同匹配规则。
- ComponentVersion `compatibilityRules[*].compDefs` 可以写精确名、前缀或正则；多匹配时不要把隐式优先级当作 addon 合同。
- `images` key 要对应 runtime container、init container、lifecycle action 字段名或明确的工具镜像名。未知 key 不应被当成已验证替换。
- 每个 serviceVersion 都要同时验证 runtime image、init image、lifecycle action image、参数 schema、BPT method env、backup/restore 工具镜像和升级路径。
- Cluster 未显式指定 serviceVersion 时，不要猜测最终选择；开发样例应显式写 serviceVersion，排查时看 resolved Component、ComponentVersion status 和 Pod image。

ComponentVersion 不能只替换主容器镜像。每个 release 至少要列清楚 runtime containers、init containers、lifecycle action、backup/restore ActionSet、参数 reloader、外部工具镜像是否随 serviceVersion 变化。ComponentVersion 能覆盖 ComponentDefinition 中的 runtime、init container 和 lifecycle action image；不会自动改写 ActionSet image，也不会改写脚本 ConfigMap 里硬编码的工具镜像。

BPT 的 `versionMapping` 是数据保护侧的版本矩阵。它按 Cluster component 的 serviceVersion 匹配 method 侧版本化配置，只能说明某个 backup method 用哪个版本化配置，不证明该 method 可 restore、可 rebuild、可 scaleOut.fromBackup 或可跨版本。每增加一个 ComponentVersion release，都要回查 PD/PCR、BPT `versionMapping`、ActionSet image/env 和 examples。

升级验收不能只看新镜像启动。至少要保存 old/new 的 ComponentVersion status、Cluster/Component resolved serviceVersion、Pod image、action image、参数定义、BPT method env 和业务数据兼容结果。跨大版本的数据兼容、回滚限制和备份恢复兼容要写进 README 或实现说明；没有证据的组合写成未证明。

版本选择排查先看用户入口，再看 resolved 对象。用户入口包括 Cluster `componentSpecs[*].serviceVersion`、ClusterDefinition topology 中的 compDef 规则、ComponentVersion compatibilityRules、chart values 中的 image tag 和 examples 中的默认值。resolved 对象包括 Component spec、Component status、InstanceSet template、最终 Pod image、action worker image 和 BackupPolicy method env。任何一层缺失，都不能只靠 `Chart.yaml` 或镜像 tag 下结论。

多个 ComponentVersion 同时覆盖同一 ComponentDefinition 时，addon 不应依赖隐式排序。推荐做法是让 ComponentDefinition 名称、serviceVersion、ComponentVersion release name 和 examples 保持可预测映射；如果需要兼容旧名，使用短期兼容规则并在发布验收中证明不会误命中新旧两个 release。

升级路径要覆盖回滚和失败中断。runtime image 更新后，如果参数 schema、配置文件格式、数据目录格式或备份工具版本也变化，回滚可能不是简单换回旧 image。addon 文档应写清楚支持原地回滚、需要 restore、需要人工处理，还是未证明。没有回滚证据时，不要把 upgrade example 写成完整升级能力。

## 6. 生命周期动作要按数据库语义建模

release-1.1 `ComponentLifecycleActions` 支持 `postProvision`、`preTerminate`、`roleProbe`、`availableProbe`、`switchover`、`memberJoin`、`memberLeave`、`readonly`、`readwrite`、`dataDump`、`dataLoad`、`reconfigure`、`accountProvision` 等动作。action 是业务合同，不是通用 shell hook。

实践规则：

- roleProbe 输出必须匹配 `spec.roles` 中定义的 role。role label 会影响 roleSelector Service、Backup target、switchover 和 rollout。
- `roles[*].isExclusive` 只约束 KB 记录的 role label 视角，不能证明数据库内部没有 split-brain，也不能替代引擎级 fencing、租约或 quorum 校验。
- memberJoin/memberLeave 要幂等，能区分目标实例不存在、已加入、已移除和半加入状态。
- switchover 要验证候选 Pod、旧主、复制状态、role label、Service endpoints 和业务读写状态。
- preTerminate 适合目标 Pod 或可管理执行面仍存在时的业务下线。删除卡住时先保留现场，查 action target、Pod/PVC 是否存在、kbagent/action 输出和业务成员列表。
- action `exec.command` 是命令数组，不是 shell 字符串；需要管道、重定向或多命令时，应调用脚本文件或显式 `sh -c`。
- `preCondition` 主要用于 `postProvision` 类等待场景，不要泛化为所有 lifecycle action 的收敛证明。
- release-1.1 的 `Ordinal` target selector 已在 lifecycle target 选择中实现；`matchingKey` 必须是非负整数，选择依据是 Pod 名最后一个 `-<ordinal>`，多 Pod 匹配同一 ordinal 会报 ambiguous。它适合明确的实例级动作，不适合表达持久业务成员身份。

quorum 和更新顺序要作为 HA addon 的显式验收项：

| 验收点 | 需要证明 |
| --- | --- |
| `roles[*].participatesInQuorum` | 哪些角色参与多数派；滚动、scale-in、Stop、故障恢复时不会把多数派同时拿下 |
| `roles[*].updatePriority` | leader/primary、follower/secondary、learner 等角色的升级顺序符合引擎安全要求；相同 priority 不代表 leader 一定最后更新 |
| `roleProbe` 周期和 threshold | role label 收敛延迟在 switchover、backup target、roleSelector service 和 rollout 中可接受 |
| `podUpdatePolicy/podUpgradePolicy/instanceUpdateStrategy` | 与 quorum、role priority 和 switchover 策略组合后，升级期间仍满足业务可用性或明确需要停机 |
| 引擎自检 | KB role label 与引擎 membership、term/epoch、primary lease 或一致性读写结果一致 |

常见 lifecycle/action 字段语义如下：

| 字段 | 脚本输入或选择语义 | addon 要证明什么 |
| --- | --- | --- |
| `postProvision.preCondition` | `Immediately`、`RuntimeReady`、`ComponentReady`、`ClusterReady` 决定动作触发时机 | 脚本依赖的 Pod、Service、账号、角色在该时机已经存在 |
| `preTerminate` | 删除或缩容前执行，controller 等待它成功后继续释放资源 | 脚本不依赖已不存在的 Pod；不需要执行时有明确跳过策略 |
| `availableProbe` 和 `available.withProbe` | 用 action 结果判定可用性；设置后会覆盖 withPhases/withRole 的判定方式 | probe 输出和失败语义稳定，不会把初始化中状态误判为可服务 |
| `switchover` | 候选实例可由 Ops 指定；未指定候选时 action 成功不等于业务切换已可被客户端感知 | candidate 为空和非空两条路径都能处理；roleProbe 和 Service endpoint 最终收敛 |
| `memberJoin` | 新成员加入复制组 | 新 Pod Ready 后能加入复制组，重复执行不会破坏已有成员 |
| `memberLeave` | 删除前摘除成员 | 删除前能摘除成员；如果需要数据迁移，要作为单独限制说明 |
| `readonly/readwrite` | 由 volume highWatermark 等容量保护路径触发 | 引擎有真实只读/读写切换命令，且状态可回查 |
| `dataDump/dataLoad` | 实例初始化类数据复制 | 不等价于 Backup/Restore method |
| `reconfigure` | 文件变更后的重载动作 | 脚本按文件名和参数名校验输入，支持幂等重试 |
| `accountProvision` | 创建或变更数据库内账号 | SQL/CLI 执行有转义、脱敏和最小权限控制 |
| `targetPodSelector` 和 `matchingKey` | `Role` 按 role 选择，`Ordinal` 按 Pod 名 ordinal 选择，`Any/All` 不使用 matchingKey | 依赖 role 的动作必须先证明 roleProbe；依赖 ordinal 的动作要证明 Pod 名到业务身份映射 |
| `timeoutSeconds` 和 `retryPolicy` | 非 0 退出、HTTP 非 2xx、超时都会按策略失败或重试 | 脚本要能安全重试；长耗时动作不要用默认超时碰运气 |

`preCondition` 要按脚本依赖选择，不要按“越晚越安全”猜：`Immediately` 适合只依赖 Secret 或本地初始化；`RuntimeReady` 适合需要容器文件、挂载卷或基础进程；`ComponentReady` 适合需要当前 Component 副本和 role 已收敛；`ClusterReady` 适合依赖其它 component 的后置初始化。若动作本身会推动 ComponentReady 或 ClusterReady，就不要选择会形成等待环的条件。

HTTP/GRPC action 要把接口当成版本化合同。HTTP action 至少固定 method、path、headers、body、成功状态码范围和 body assertion；`204` 无 body 时不要写依赖 body 的断言。GRPC action 至少固定 service/method、payload schema、是否 unary、是否依赖 reflection，以及工具镜像里客户端版本。接口、payload 或工具镜像随 serviceVersion 改变时，要和 runtime image、PD、BPT 一起进入版本矩阵。

超时和 retry 只说明 controller 视角的执行结果，不证明远端副作用已回滚。长耗时 action 要有幂等键或业务状态检查；超时后先查远端是否已部分执行，再决定重试、补偿或标为需要人工处理。retry 期间 targetPodSelector 可能重新选到不同 Pod，依赖单一目标的动作要固定目标输入或在脚本里拒绝目标漂移。

`highWatermark` 场景要同时验收只读进入和读写恢复。触发 readonly 后，检查引擎真实只读状态、写请求失败方式和用户可见限制；容量恢复或扩容后，`readwrite` action 必须能把引擎带回可写，并用业务写入或引擎状态查询证明，而不是只看 action 退出 0。

`memberJoin` 和 `memberLeave` 要同时证明三件事：controller 视角的 action 已完成，业务成员关系已经收敛，重复执行或中途失败后不会把状态写坏。scale-in 不能只等待固定时间：如果引擎因为 quorum、shard awareness、数据迁移、replica placement 或磁盘状态拒绝摘除成员，脚本要能给出可诊断错误、可恢复退出或明确人工处理步骤。

拓扑变化后的 action 不要依赖创建时写入 Pod env 的成员列表。`componentVarRef.podNames/podFQDNs`、旧 `KB_*` 列表或脚本启动时缓存的 endpoint 只能证明 Pod 创建时的视图；scale-out、scale-in、switchover、rebuild 之后，脚本应优先使用 action-time 注入的目标 Pod/FQDN、当前 Service/EndpointSlice、业务 membership 查询或显式传入的 candidate。

## 7. 变量注入和服务依赖是契约

release-1.1 `ComponentDefinition.spec.vars` 支持多种来源：cluster、component、credential、service、serviceRef、TLS、hostNetwork、ConfigMap、Secret 等。变量可用于 Pod/action env，也可用于渲染 config/script 模板。变量不是脚本私货；每个变量的格式、缺失语义和刷新语义都要写进实现说明。

常见变量和服务来源：

| 来源 | release-1.1 输出或行为 | 脚本消费方式 | 验收证据 | 边界 |
| --- | --- | --- | --- | --- |
| `clusterVarRef` | cluster name、namespace、uid 等 | 普通 env/template 值 | 最终 Pod/action env | 不应作为不可重算业务成员 ID |
| `componentVarRef` | component 名、replicas、Pod 名/FQDN 列表、按 role 过滤列表 | 逗号列表等字符串约定 | 脚本解析、scale 后验证 | Pod env 是创建时视图，拓扑变化要另验 |
| `credentialVarRef` | system account 用户名/密码 | env 传入脚本 | Secret 和数据库内账号都存在 | 不应渲染进普通 ConfigMap |
| `serviceVarRef` | Service host/port 或 podService 地址 | advertised address、client 连接 | Service、EndpointSlice、连接测试 | Service 可达不等于客户端地址正确 |
| `serviceRefVarRef` | 外部依赖 ServiceDescriptor 或 KB Cluster 信息 | 跨组件/外部连接 | declaration、Cluster binding、最终 env、真实连接 | credential 跨 namespace 有限制 |
| `tlsVarRef` | CA/cert/key 路径或 Secret 引用 | runtime/action/backup/restore TLS | 每个执行面连接测试 | Secret 存在不等于工具链可用 |
| `hostNetworkVarRef` | hostNetwork 动态端口 | advertised address、配置文件 | Pod spec、端口占用、最终 env | hostNetwork 不是固定端口合同 |

变量和服务合同检查：

- 每个变量都要在脚本、config、action 或 Job 中有明确使用点。不要为了“可能以后有用”注入大量变量。
- 避免 `runtime.containers[*].env`、CMPD `vars` 和 Cluster `componentSpecs[*].env` 定义同名变量。确实需要覆盖时，验收口径以最终 Pod env、action env 和渲染后的 config/script 为准。
- `KB_` 前缀变量视为 KB 注入保留空间，addon 自定义变量不要复用。
- 多组件和多 shard 变量要明确值格式，例如逗号分隔 Pod 列表、FQDN 列表、`name:port` 列表或 JSON 片段。
- `Optional` 变量缺失时脚本要有显式分支；`Required` 变量缺失应让问题尽早暴露。不要假设 Optional 一定渲染为空字符串。
- ServiceRef 要同时给出 declaration、Cluster 侧绑定示例和脚本读取方式。
- 外部访问能力必须逐类型验证。ClusterIP、NodePort、LoadBalancer、hostNetwork 的地址来源和 DNS/端口语义不同，不能用一个 advertised address 模板覆盖所有模式。

ServiceDescriptor 在 release-1.1 中会校验 `serviceKind`、`serviceVersion` 和 SecretRef 是否存在，但实现仍不校验 Secret data key 的完整性；credential 变量解析时 Secret key 缺失也可能不直接报错。不能把 `ServiceDescriptor.status.phase=Available` 当成外部服务凭据可用证明，必须验证最终 env/config 和真实连接。

ServiceRef 的 credential 不能任意跨 namespace 引用。需要跨 namespace 外部依赖时，应使用明确的 ServiceDescriptor 和权限/凭据交付方式，并声明限制。`clusterServiceSelector` 和 `serviceDescriptor` 是不同绑定路径；前者引用 KB Cluster 服务，后者引用外部服务描述。两者产生的 endpoint、credential 和 serviceVersion 校验边界不同，不能在脚本里混成一个隐式合同。

外部依赖要设计变更生命周期，而不只是首次注入。provider 的 endpoint、host、port、credential、ServiceDescriptor status 或拓扑发生变化后，consumer 是否自动 re-render、是否需要 Reconfigure、Restart、重建或明确不支持，都要写成支持矩阵并用最终 Pod/action env、runtime ConfigMap 和真实连接证明。

服务和依赖注入的验收顺序是最终 Service、EndpointSlice、Pod label/Ready、注入 env、真实连接。`publishNotReadyAddresses` 只能说明 endpoint 可以早发布，不说明业务已经 ready；客户端是否会过早连接、是否有启动握手或重试，要由 addon 自己证明。

## 8. 账号和 TLS 要作为一等能力设计

账号至少分三类：初始化账号、运行时管理账号、执行面账号。执行面账号包括 roleProbe、backup、restore、replication、switchover、member lifecycle、client examples 等。每类账号都要说明生成方式、Secret key、数据库内权限、禁用行为和轮换边界。

账号矩阵：

| 账号类型 | 典型用途 | release-1.1 声明面 | 必须验收 |
| --- | --- | --- | --- |
| 初始化账号 | 首次初始化、root/admin 创建 | systemAccounts、accountProvision、init script | Secret key、数据库内账号、权限范围、重复执行 |
| 运行时管理账号 | roleProbe、switchover、member lifecycle | credentialVarRef、lifecycle action env | action 执行面可读取，权限最小化，输出脱敏 |
| 数据保护账号 | backup、restore、PITR、postReady | BPT target account、ActionSet env | Backup/Restore Job 可读，数据库权限和 TLS 均可用 |
| 外部连接账号 | client example、proxy/router、ServiceRef consumer | ServiceDescriptor Secret、system account、用户 Secret | Secret key 完整、连接成功、权限和轮换策略 |
| 复制或集群内部账号 | replication、sentinel、keeper、router | script/config、systemAccounts、Secret | scale、switchover、restore 后仍一致 |

TLS 不是只让主 Pod 能启动。必须分别验证 runtime、roleProbe、lifecycle action、backup Job、restore Job、postReady Job、client example 是否拿到正确 CA/cert/key、路径、权限和连接参数。只证明 Secret 存在时，只能说证书对象已生成，不能说数据库 TLS 能力已支持。

RBAC 要按执行面拆开。Component `policyRules`、runtime Pod 使用的 ServiceAccount、lifecycle action 所在 Pod、Backup/Restore Job、Repo pre-check Job 可能不是同一权限面。排查权限问题时先看最终 Pod/Job `serviceAccountName`、Role/RoleBinding/rules 和事件，不要因为 runtime Pod 有权限就推断 restore worker 也有权限。

凭据和 TLS 轮换要明确支持边界。Secret 更新后是否触发参数重渲染、Pod 重启、action 重新读取、BackupPolicy/Job 使用新 Secret，不能从 Secret resourceVersion 直接推断。不能证明自动轮换时，应写成需要重启、重新执行 Ops 或人工更新。

## 9. 配置和参数管理要串成闭环

release-1.1 的参数链路由 ComponentDefinition configs、模板 ConfigMap、`ParametersDefinition`、`ParamConfigRenderer`、`ComponentParameter`、runtime ConfigMap、reload/restart 路径共同构成。新增或重写 addon 应使用 `ParametersDefinition` 和 `ParamConfigRenderer` 作为主路径。

对象链：

| 链路节点 | 语义 |
| --- | --- |
| `ComponentDefinition.spec.configs[*].name` | 配置 item 名称，参数链路绑定键 |
| `ComponentDefinition.spec.configs[*].template` | 模板 ConfigMap 名 |
| 模板 ConfigMap data[file] | 实际模板文件和可渲染内容 |
| `ParametersDefinition.spec.fileName` | 参数定义绑定的文件名 |
| `ParamConfigRenderer.spec.configs[*].templateName` | 绑定 CMPD `configs[*].name` |
| `ComponentParameter` | controller 落地的参数 desired/status |
| runtime ConfigMap | 最终挂进 Pod 的配置文件 |
| reloadAction 或 restart | 参数真正生效路径 |
| 进程内配置查询 | 最终业务证据 |

实践规则：

- `ParametersDefinition.spec.fileName` 必须是模板 ConfigMap data 中真实存在的文件名。
- `ParamConfigRenderer.spec.componentDef` 要指向目标 ComponentDefinition；`parametersDefs` 要列出参与渲染的 PD；`configs[*].templateName` 要等于 CMPD `spec.configs[*].name`，不是模板 ConfigMap 名，也不是 runtime ConfigMap 名。
- `staticParameters` 表示需要 restart，`dynamicParameters` 表示可 reload，`immutableParameters` 表示不应修改。没有分类证据时不要声明热更新。
- release-1.1 PD 的 reload 语义受 `dynamicParameterSelectedPolicy`、`reloadAction`、`mergeReloadAndRestart`、`reloadStaticParamsBeforeRestart`、`deletedPolicy` 共同影响。每个字段只能说明 controller 执行路径，不能替代进程内查询。
- 外部 ConfigMap、formatter、default value、Secret 进入配置文件等能力要单独证明；敏感值不要渲染进普通 ConfigMap。
- `externalManaged` 表示配置交给 parameters 链路管理，不表示 `templateName` 可以指向底层 runtime ConfigMap。

参数排查固定看三态：desired 看 `Parameter`/OpsRequest 和 ComponentParameter，rendered 看 runtime ConfigMap 和 revision/status，effective 看 SQL/CLI/config dump。OpsRequest 成功不等于进程已生效，runtime ConfigMap 更新也不等于进程已 reload。

参数字段排查矩阵：

| 问题 | 第一跳 | 第二跳 | 业务证据 |
| --- | --- | --- | --- |
| 找不到 ComponentParameter | CMPD `configs[*].externalManaged`、PCR componentDef、PD status | Component status、controllers/parameters 事件 | 不支持 reconfigure 时明确声明 |
| ConfigMap 没生成或 data key 缺失 | CMPD `configs[*].template`、模板 ConfigMap | PD `fileName`、PCR `templateName` | Pod 内文件存在 |
| 参数被写入错误文件 | PD `fileName`、PCR configs、template data | ComponentParameter spec/status、runtime ConfigMap | 进程读取目标文件 |
| 动态参数没生效 | PD dynamic/static 分类、reloadAction | action 输出、Pod restart 状态 | 进程内查询值改变 |
| 静态参数被热更新 | PD 分类、reloadStaticParamsBeforeRestart | OpsRequest 条件、restart 计划 | 重启后进程值正确 |
| 删除参数行为异常 | `deletedPolicy`、template 默认值 | ComponentParameter desired、runtime ConfigMap | 进程内参数恢复或拒绝 |
| 多 Pod 参数分裂 | action target、role、All/Role/Ordinal 选择 | 每个 Pod runtime ConfigMap | 每个实例进程值一致或差异符合预期 |
| Secret 泄露 | template、runtime ConfigMap、action 输出 | Pod env/Secret mount | 普通 ConfigMap 不含敏感值 |

半支持状态要显式处理。某些 addon 的配置形态如果本来不能通过 release-1.1 参数链路安全管理，不要为了让 OpsRequest 看起来成功而制造空 PD/PCR 或错误的 `externalManaged`。正确做法是显式拒绝、不提供 reconfigure example，或写成需要重启/人工修改的限制。

ComponentParameter desired 会成为后续重渲染的输入。错误参数一旦写入 desired，可能在 scale、restart、reconfigure 或 controller 重试中持续污染 runtime ConfigMap。排查时要先保存旧对象，再判断是用户输入、PD 分类、模板渲染还是 merge 逻辑导致污染；不要直接改 controller 来吞掉错误参数。

动态参数作用域要按引擎语义设计。某些参数是单实例本地参数，某些必须全组件一致，某些只允许在 primary 上执行后由引擎传播。reload action 的 target 选择必须和这个语义一致；单 Pod reload 成功不能证明 HA 集群参数一致。

PD/PCR 设计要把文件格式和参数分类分开。`fileFormatConfig`、formatter、CUE schema、default value、unit、枚举和范围只能证明参数能被解析；`staticParameters`、`dynamicParameters`、`immutableParameters` 才表达变更执行路径。一个参数在语法上可写，不代表运行时可热更新；一个参数被列为 dynamic，也不代表所有 topology、所有 role、所有 serviceVersion 下都能热更新。

多文件配置要避免“一个参数名多处出现”的隐式合同。相同 key 如果出现在多个文件，PD/PCR 要明确哪个 fileName 管理它，或者在文档中写明它是多个文件的组合参数。否则 reconfigure 时可能只更新一个文件，reload action 却读取另一个文件，最终形成 desired/rendered/effective 三态不一致。

删参和默认值要单独验收。用户把参数设置为 nil、空字符串、删除 key、恢复默认值，是四种不同输入；`deletedPolicy`、模板默认值和引擎默认值也不是同一个概念。addon 要说明删除参数后 runtime ConfigMap 是删除 key、写空值、恢复模板默认，还是拒绝操作，并用进程内查询证明。

参数和 Cluster overrides 的 ownership 要保持清楚。Cluster 创建时的 configs、后续 Parameter CR/OpsRequest、模板默认值、外部 ConfigMap、formatter 输出都可能参与最终文件。排查时先确定哪个对象拥有 desired，再看 controller 渲染结果；不要让用户同时修改 runtime ConfigMap 和 Parameter CR 来“碰运气”修复。

参数测试至少覆盖八类用例：合法动态参数、合法静态参数、immutable 参数拒绝、未知参数拒绝、类型错误、删参、同一参数多次变更、scale/restart 后参数保持一致。缺少其中某类时，文档只能写该类未证明。

## 10. 数据保护要同时提供策略和执行脚本

release-1.1 的数据保护至少包括 `BackupPolicyTemplate`、`ActionSet`、`BackupRepo`、Backup/Restore CR 和执行脚本。BPT 负责让目标 Cluster 生成 BackupPolicy；真正执行的是 BackupPolicy/Backup/ActionSet/BackupRepo 组合。

实践规则：

- BPT `compDefs` 支持精确名、前缀或正则。每新增 ComponentDefinition 或 topology 都要验证 BackupPolicy 是否生成。
- `backupMethods[*].snapshotVolumes=true` 可以走 CSI snapshot，不一定需要 ActionSet；非 snapshot method 必须有 `actionSetName`。
- method-level target 可能覆盖全局 target。target role 在多副本和单副本场景要分别验收，不能把单副本 fallback 外推到 HA。
- `versionMapping` 只证明 method 侧配置如何匹配 serviceVersion，不证明 restore、PITR、rebuild 或跨版本兼容。
- ActionSet `onError` 支持 `Continue` 和 `Fail` 两个枚举；使用 `Continue` 时必须证明被忽略的步骤不会破坏 artifact、status 和恢复路径。
- BackupRepo 必须 Ready。repo 不可用、default repo 冲突、Secret 轮换、PVC/PV retain/delete、BackupRepo 删除被已有 Backup/PVC 阻塞，都应作为数据保护排查入口。

字段语义矩阵：

| 字段 | addon 要表达的语义 | 必须同时验证 |
| --- | --- | --- |
| BPT `serviceKind`、`compDefs` | 哪类组件自动生成 BackupPolicy | resolved ComponentDefinition 只匹配预期 BPT |
| BPT `target.role/fallbackRole/account/strategy` | 选哪个 Pod、用哪个系统账号连接数据库 | roles/roleProbe、systemAccounts、单副本和多副本选 Pod 行为 |
| BPT `schedules/backoffLimit/retentionPolicy` | 默认备份计划、失败重试和保留策略 | 生成的 BackupPolicy/BackupSchedule、实际 Backup 创建 |
| method `name` | 用户和 Backup CR 调用的稳定方法名 | method 名语义单一，不把 physical/logical/incremental/PITR 混用 |
| method `compatibleMethod` | 增量或差异备份依赖的 full method | parent backup 链和 restore 顺序 |
| method `snapshotVolumes` | 是否走 CSI snapshot | CMPD `volumes[*].needSnapshot`、StorageClass snapshot 支持 |
| method `actionSetName` | 非 snapshot method 绑定执行定义 | ActionSet status Available，backup/restore 阶段都存在 |
| method `targetVolumes` | backup workload 挂载目标 Pod 的哪些 volume | CMPD volume 名、容器 mountPath、restore PVC |
| method `env/runtimeSettings/target` | method 级覆盖工具镜像、资源和选 Pod 策略 | versionMapping、资源请求、target 覆盖是否和全局 target 冲突 |
| ActionSet `backupType` | Full、Incremental、Differential、Continuous、Selective 之一 | Backup/Restore 示例和 method 名一致 |
| ActionSet `parametersSchema/withParameters` | Backup/Restore 参数名校验 | 脚本仍做值校验，不能只依赖 schema |
| `backup.preBackup/backupData/postBackup/preDelete` | 备份前、备份主体、备份后和删除前动作 | 失败退出语义、重试幂等性、产物是否写到 repo 路径 |
| `backupData.syncProgress` | 是否同步进度到 Backup status | 脚本输出、Backup status totalSize/duration/phase |
| `restore.prepareData/postReady` | 数据准备到目标 PVC，目标组件 ready 后修复集群内状态 | 新集群恢复、rebuild、账号、TLS、拓扑兼容性 |
| Restore `backup.sourceTargetName/restoreTime` | 从哪个 source target 和时间点恢复 | PITR、增量链路、source target 名称 |
| Restore `prepareDataConfig/readyConfig/env/backoffLimit/parameters` | restore job 的 PVC、调度、env 合并、重试和参数 | 最终 env 合并结果、参数默认来源和脚本行为 |

每个 backup method 都要建兼容矩阵：method name、backup type、target role、ActionSet、BackupRepo 需求、artifact 类型、是否支持新集群 restore、是否支持 PITR、是否支持 RebuildInstance、是否支持 scaleOut.fromBackup、是否支持 sharding/topology 变化、需要的账号/TLS/env、已验证 addon 示例。

每个 topology 也要单独证明 BackupPolicyTemplate 是否生效。`standalone`、`replication`、`mgr`、`proxy`、`sharding`、`keeper` 这类拓扑可能使用不同 ComponentDefinition 名称、roleProbe、target role、账号和数据卷；BPT `compDefs` 命中一个普通组件，不代表它覆盖了 topology 专属组件。最小证据是该 topology 的 Cluster 生成了预期 BackupPolicy，Backup target selector 选中正确 Pod，ActionSet method 与该 topology 的账号、TLS、volume 和 serviceVersion 兼容。

Backup `Completed` 不是 restore contract。每个 method 都应产出或能推导一份 artifact manifest：仓库路径、target 名称、数据文件、元数据文件、formatVersion、base/parent、加密信息、工具版本、必要模板或 keystore。restore preflight 要先检查这份 manifest 和目标 Cluster 条件，再开始覆盖 PVC；如果缺 artifact、路径不一致、工具镜像不兼容或目标 topology 不匹配，应清楚失败。

加密、压缩和格式版本必须做 payload 级验证。API 字段、Secret 或 env 存在，只能证明执行面收到了配置；它不证明备份数据实际经过了支持该能力的工具链。使用 native client、数据库自带 backup 命令、datasafed、kopia、对象存储 SDK 或自写脚本时，要分别说明数据从哪里写出、是否经过加密/压缩、artifact manifest 在哪里、restore 用哪个工具读取，以及错误密钥、错误 formatVersion、缺少 keystore 或缺少模板文件时如何失败。

空数据集、无增量或无变化也是合法场景，不能让备份脚本无限等待。addon 要定义空 full backup、空 incremental backup、没有 binlog/WAL/segment、没有 topic/table/slot 变化时的退出码、artifact 形状和 restore 结果。若该 method 不支持空数据或空增量，应在 method 文档和脚本前置检查里明确拒绝。

BackupRepo 是独立依赖，不是 BackupPolicy 的附属字段。排查备份失败、恢复失败或仓库凭据轮换时，必须同时看 BackupRepo/Secret、BackupPolicy、BackupSchedule、Backup/Restore Job 的 env 和 mount。Repo Secret 轮换后，已经存在的 Job 不会因此自动换凭据，后续 Backup/Restore 是否消费新 Secret 要以最终 Job spec 为准。

Continuous backup 和 PITR 的状态语义要单独写清楚。Continuous method 长期 `Running` 可能是正常归档状态，不等于一次性 Full Backup 未完成；addon 要验收归档产物持续生成、timeRange 可解释、`restoreTime` 落在可恢复窗口内，并实际演练 PITR。

跨 topology、sharding 或 rebuild restore 必须有 target 映射表：同 topology 新集群恢复要证明 source target 到目标 component/volume 一一对应；rebuild instance 要证明 source backup 到目标 ordinal/PVC/实例身份对应；跨 topology 恢复要显式映射 source component 和目标 component；sharding restore 要说明 source shard/target 到目标 shard 的映射，目标 shard 多于或少于源 shard 时不能默认支持。

从已有 `Backup` 恢复新 `Cluster` 时，release-1.1 的用户入口应写成 Restore `OpsRequest`。不要要求 addon 用户手写 `kubeblocks.io/restore-from-backup` annotation；该 annotation 是 operations restore handler 根据源 Backup 和 OpsRequest 输入生成的内部恢复 intent。独立 `dataprotection Restore` CR 只负责 PVC 数据准备和 postReady，不会替用户创建新的 Cluster。

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

## 11. Day-2 运维以内置操作为主

release-1.1 `OpsRequest.spec.type` 支持 Start、Stop、Restart、Switchover、VerticalScaling、HorizontalScaling、VolumeExpansion、Reconfiguring、Upgrade、Backup、Restore、Expose、RebuildInstance 等类型。addon 作者不要从 OpsRequest 入口推断能力已经成立；每种 Ops 都依赖底层合同。

Ops 矩阵：

| Ops | release-1.1 入口字段 | 依赖的 addon 合同 | 运行验收 | 常见误用 |
| --- | --- | --- | --- | --- |
| Start/Stop/Restart | `start/stop/restart` | 启动脚本幂等、readiness、一次性初始化保护 | Pod 重建或恢复后数据不丢 | restart 后重复初始化 |
| HorizontalScaling | `horizontalScaling` | replicasLimit、memberJoin/memberLeave、sharding 边界 | 新成员加入或旧成员摘除 | 只看 Pod 数量 |
| VerticalScaling | `verticalScaling` | 资源变更后引擎是否感知 | Pod/cgroup/进程资源状态 | 资源改了但数据库内部未更新 |
| VolumeExpansion | `volumeExpansion` | VCT 名、PVC、mountPath、文件系统和引擎容量 | PVC、容器内 FS、引擎状态 | VCT 名和 volume 名不一致 |
| Reconfiguring | `reconfigures` | PD/PCR/ComponentParameter、reload/restart | 文件和进程配置一致 | 只看 Ops Succeed |
| Expose | `expose.services` | ComponentService、roleProbe、地址脚本 | Service/EndpointSlice/连接 | Service type 和 advertised address 混用 |
| Switchover | `switchover` | roleProbe、switchover action、roleSelector Service | candidate 成为目标 role，服务切换 | action 成功后不查角色 |
| Upgrade | `upgrade` | ComponentVersion、镜像、rollout、数据兼容 | old/new 版本和数据正确 | 只升级 runtime image |
| Backup/Restore | `backup`、`restore` | BPT、ActionSet、BackupRepo、Restore 脚本 | Backup artifact 和 Restore 成功 | 只有 Backup 无 Restore |
| RebuildInstance | `rebuildFrom` | Restore 兼容矩阵、目标 PVC、postReady、member lifecycle | 单实例恢复并重新加入 | 新集群 restore 成功就声称 rebuild 支持 |

容易误用的 release-1.1 Ops 字段：

| 场景 | 字段 | 1.1 行为要点 | addon 验收 |
| --- | --- | --- | --- |
| scale-out 从备份扩容 | `horizontalScaling.scaleOut.fromBackup` | operations 会为新增实例准备 restore 数据；字段只对非 sharding component 有效 | method 必须支持该目标实例、RestoreEnv、postReady、memberJoin |
| rebuild 指定来源 | `rebuildFrom.backupName/sourceBackupTargetName/restoreEnv` | rebuild 路径把 restore 能力接入实例替换或 in-place | source target、env merge、PVC、成员重加都要验证 |
| 单实例原地 rebuild | `rebuildFrom.inPlace` | 不等价于新建 Cluster restore | 先证明数据准备和进程重启不会破坏成员关系 |
| shard 级操作 | sharding 相关字段 | 对象选择进入 shard Component 链路 | 不自动证明 rebalance 或跨 shard 数据一致 |
| Expose | `expose.services[*].roleSelector/podSelector/serviceType` | 只创建或更新 Service | advertised address、EndpointSlice 和客户端连接要另验 |
| force/cancel/ttl | OpsRequest 状态机字段 | 控制排队、取消和清理 | 不代表业务动作可安全中断或跳过 |

几个 Day-2 边界要提前写成支持矩阵。`Start` 只恢复停止的 workload，不等于重新执行 `postProvision`；一次性初始化必须有幂等保护。Vertical scaling 不只看 OpsRequest 成功，要给出调度、Pod 重建或 in-place 变化、容器内 cgroup 资源和引擎内部资源感知的证据。

`scaleOut.fromBackup` 是 Restore 能力的独立消费者，不能用“新建 Cluster restore 成功”替代。它要验收 source Backup、restoreTime 或 PITR 窗口、restoreEnv、目标新副本的 memberJoin、角色收敛和数据一致性。多 target Backup 场景必须声明 `sourceBackupTargetName` 到目标 component、volume、ordinal 或 shard 的映射。

`RebuildInstance` 至少区分 in-place 和 replace 两条路径。in-place 路径要证明旧进程停机、文件锁、PVC 清理/复用、restore 覆盖和 postReady 幂等；replace 路径要证明新旧 Pod/Service endpoint 过渡、memberLeave/memberJoin、PVC 绑定、存储拓扑和客户端切流。

交错 Ops 要么证明支持，要么写明禁止组合：升级期间是否允许 switchover，reconfigure 与 restart/vertical scaling 是否可并发，scale-in 期间是否允许 backup，stop 后 delete 是否保证 preTerminate。不能证明组合安全时，应写成禁止组合或需要人工确认。

Ops 状态机排查要允许“状态”和“事实”短暂不一致。并发 Ops、precondition、force、cancel、timeout 和 TTL 先看 OpsRequest `phase/message/conditions` 与目标 Component 状态，再看是否已有 action 后置校验失败、status 更新失败或 partial success。事实已经改变但 Ops 失败时，不要直接重试，先用最终对象和业务自检判断是否幂等安全。

复杂 rebalance、跨 topology 在线迁移、需要人工确认的数据迁移，不应包装成通用支持能力。

## 12. 常见操作排障要从入口 CR 逐层向下查

release-1.1 的运行问题通常跨多个控制器。排障顺序固定为：入口 CR 的 generation/observedGeneration/phase/message/conditions，被引用 definition/policy/action 的 Available 状态，controller 生成对象，最终 workload，脚本输出，业务自检。

| 操作 | 先查入口对象 | 再查底层对象 | 常见 addon 问题 | 推荐处理 |
| --- | --- | --- | --- | --- |
| create | Cluster、ClusterDefinition、CMPD、CMPV | Component、InstanceSet、Pod、PVC、Service、ConfigMap | compDef/template/volume/script 名错 | 修 chart 引用，不改 controller |
| delete | Cluster/Component deletionTimestamp、finalizers | InstanceSet、Pod、PVC、preTerminate/memberLeave | action 所需 Pod 已不存在 | 保留现场，解释执行面为何消失 |
| reconfigure | OpsRequest/Parameter、PD、PCR | ComponentParameter、runtime ConfigMap、reload/restart、进程配置 | templateName/fileName 错、desired 污染 | 按 desired/rendered/effective 三态修复 |
| horizontal scale | OpsRequest、Component replicas | InstanceSet、Pod、memberJoin/memberLeave | 成员关系未收敛 | 实现 member lifecycle 或声明限制 |
| switchover | OpsRequest、roleProbe、Service | action output、role label、EndpointSlice、业务查询 | roleProbe 与 switchover 脱节 | 修 role 合同和服务选择 |
| backup | BackupPolicy、Backup、BPT | target Pod、ActionSet、Repo、artifact | BPT 未匹配、target role 错、账号错 | 先看 BackupPolicy 生成和 target |
| restore/rebuild | Restore/OpsRequest | Job/PVC、postReady、memberJoin、数据校验 | backup method 只支持 backup | 建 method 消费者矩阵 |
| upgrade | Cluster serviceVersion、CMPV | Pod image、action image、PD/BPT 版本覆盖、rollout | 只覆盖 runtime image | 补工具镜像和数据兼容验证 |

按问题域组织排查路径时，可以使用下面的固定入口：

| 问题域 | 第一批对象和字段 | 下一跳证据 | 边界 |
| --- | --- | --- | --- |
| 资源引用和 status | 入口对象 namespace/source、CMPD/CD/CMPV/PD/BPT/ActionSet 的 `generation/observedGeneration/phase/message/conditions` | 引用它们的 Cluster、Component、BackupPolicy、ComponentParameter、OpsRequest、Job 和事件 | 不要只看 CR 是否存在；以引用方最终生成物为准 |
| 名字和身份 | Cluster UID、spec name、generated Component name、status key、Pod/PVC/Service 名、selector label | 引擎成员 ID、备份 target、sharding component、持久数据里的身份记录 | 不把 Pod 名、generated component name、clusterUID 或 ordinal 当作不可重算业务身份 |
| 终止和数据保留 | Cluster `terminationPolicy`、deletionTimestamp、finalizers、conditions | InstanceSet、Pod、PVC、Backup/BackupRepo、preTerminate action | PVC retention、备份仓库清理、脚本失败是不同层 |
| 调度、PVC 和存储 | scheduling、runtimeClass、affinity/tolerations、volumeClaimTemplates、PVC retention | 最终 Pod spec、PVC ownerRef/labels、StorageClass、Pod/PVC events | backup/restore/action worker 是否继承调度字段，以实际 Job/Pod 为准 |
| Service 和网络 | CMPD services、Expose spec、ServiceRef、serviceVarRef、hostNetwork/hostPort | Service type/ports/selector、EndpointSlice、Pod label、env、连接测试 | ServiceRef、publishNotReadyAddresses、advertised address 都按最终对象验收 |
| ServiceAccount 和 RBAC | Cluster/Component serviceAccountName、Action/Restore 运行位置 | Pod/Job SA、Role/RoleBinding/rules、事件 | 组件 Pod、action、backup/restore Job 可能不是同一个权限面 |
| Pod 更新和 InstanceTemplate | OpsRequest、podUpdatePolicy、podUpgradePolicy、instanceUpdateStrategy、InstanceTemplate | InstanceSet current/updatedReplicas、revision、partition、Instance/Pod/PVC 身份 | `OnDelete` 表示等待用户删除 Pod，不是 controller 卡死 |
| Action、lifecycle 和 probe | preCondition、targetPodSelector、handler、container/image、timeout/retry | action worker、volumeMount、stdout/stderr、exit code、role label、Component Available 条件 | action 成功和业务收敛分开判断 |
| variables、env 和外部依赖 | CMPD vars、Cluster env、ServiceDescriptor/ServiceRef、credential/tls var | Pod/action/backup/restore env、渲染后 config/script、真实连接 | Optional 缺失、变量顺序、delimiter 和外部值一致性都以最终注入结果为准 |
| 参数和 reconfigure | PD、PCR、OpsRequest reconfigure、外部 ConfigMap key | ComponentParameter、runtime ConfigMap、file-level action、Pod restart、进程配置 dump | defaultValue、删参、多文件冲突、formatter 幂等要用文件与进程双重证据定位 |
| 备份和 artifact | BPT、BackupPolicy、BackupSchedule、Backup | method/actionSet、target Pod、Repo、snapshot 或 repo artifact | 加密、snapshot、Selective、postBackup 失败和 repo 切换只在声明使用时验收 |
| Restore、scaleOut 和 rebuild | Restore、scaleOut.fromBackup、RebuildInstance | restore Job/PVC、prepareData/postReady、sourceBackupTargetName、memberJoin/memberLeave | 新集群 restore、scaleOut.fromBackup、rebuild 是不同消费者 |
| Ops 状态和失败现场 | OpsRequest `phase/message/conditions`、force/cancel/timeout/TTL/progress/lastConfiguration | Component、InstanceSet、Pod、PVC、action 输出、events、业务自检 | Ops 字段只是证据；采集完现场前不要过早清理失败对象 |
| 非主 workload | Backup/Restore/action worker、Job、Pod | env、SA、node、volumeMount、resources、runtimeClass、events | 主组件 runtime 的字段不能自动外推到 worker |

删除、卸载、scale-in 和 Stop 后删除要分流排查。Cluster 删除先看 terminationPolicy、Cluster finalizer、Component deletionTimestamp、orders；Component 删除看 Component finalizer、InstanceSet/Pod/PVC ownerRef、preTerminate action；scale-in 看 OpsRequest、目标实例、memberLeave、PVC retention；Stop 后删除要先判断 Pod 是否仍存在。Pod 已停止或删除时不能假设 preTerminate 可执行。

如果删除卡在 lifecycle 下线动作，而目标 Pod、InstanceSet 或其它执行面已经不存在，先保留 Cluster、Component、InstanceSet/Pod、Event 和 action 状态证据，定位执行面为何提前消失。只有确认业务清理已经不需要、已经由外部补偿完成，或当前恢复目标明确要求释放资源时，才能按明确风险的运维恢复流程绕过该动作；绕过后的结果不能作为 addon 删除流程验收通过的证据。

排障路径从入口 CR 开始，但不能停在入口 CR。常用停止条件是：引用对象不存在或未 Available，生成对象缺失，action/Job 明确失败，业务自检无法证明，或需求本身超出 release-1.1 API。到达这些停止条件后，应修 addon chart/script、补验收或记录 gap/ambiguity，不要为了单个失败样例修改 controller 合同。

## 13. 最低验收标准

每个 release-1.1 addon 至少提供以下验收证据：

| 验收项 | 必须做 | 不能证明什么 |
| --- | --- | --- |
| Helm render | 覆盖 standalone、HA、proxy、sharding、TLS、serviceVersion、backup 开关 | 不证明 controller 消费成功 |
| server dry-run | 对关键 examples 做 server-side dry-run | 不证明业务运行成功 |
| 定义对象可用 | CD/CMPD/CMPV/PD/PCR/BPT/ActionSet status Available | 不证明所有 topology 可用 |
| 最小 Cluster | Pod/PVC/Service/ConfigMap/script 全部生成并可启动 | 不证明参数、备份、升级可用 |
| workload 身份 | Component、InstanceSet、Pod、PVC、Service、ownerRef 和 label 闭合 | 不证明数据库成员 ID 正确 |
| role/lifecycle | roleProbe、memberJoin/Leave、switchover、preTerminate 均有输出和业务自检 | 不证明所有 failure path 幂等 |
| 参数 | desired/rendered/effective 三态覆盖动态、静态、删参 | 不证明任意用户模板可 merge |
| ServiceRef | declaration、Cluster binding、ServiceDescriptor、最终 env/config、连接自检 | 不证明凭据轮换自动生效 |
| 账号/TLS/RBAC | runtime、action、backup、restore、proxy 执行面分别验 | 不证明其它工具镜像继承同一权限 |
| 数据保护 | BackupPolicy、Backup、artifact、Restore、postReady、数据校验 | Backup 成功不证明 PITR/rebuild/scaleOut |
| Day-2 | restart、scale、reconfigure、switchover、upgrade、delete 的入口和底层对象链 | OpsRequest 成功不证明业务收敛 |
| 发布 | old/new render、server diff、live object、生成对象对比 | Git diff 不等于最终对象 |
| 限制 | 不支持能力明确写清楚 | 不支持项不能靠脚本暗中拼接 |

测试分层要区分：chart 单测验证 helper 和渲染闭合；server-side dry-run 验证 API schema；定义对象检查验证 controller 能引用；live smoke 验证最小运行；能力 smoke 验证参数、备份、restore、scale、switchover、upgrade；故障用例验证删除、重试、幂等、空数据、repo 不可用、凭据缺失和失败现场。

每个能力声明都要有对应示例。README 写“支持 backup”时，examples 中至少要有 BackupRepo、Backup、Restore 或等价入口；写“支持 reconfigure”时，要有动态、静态和非法参数示例；写“支持 HA”时，要有 roleProbe、Service、switchover 或故障恢复示例；写“支持 upgrade”时，要有 old/new serviceVersion 和数据兼容说明。

验收矩阵按能力声明组织，而不是按 CRD 类型组织。一个 addon 可以只支持 standalone + backup，不支持 HA；也可以支持 HA 但不支持 PITR；还可以支持普通 scale-out 但不支持 scaleOut.fromBackup。每个能力都要写清适用 topology、适用 serviceVersion、执行面、示例、失败路径和不支持项。

验收结果要保留能复现的对象证据。最小集合包括 rendered manifests、server dry-run 结果、定义对象 status、Cluster/Component/InstanceSet/Pod/PVC/Service、ComponentParameter/runtime ConfigMap、Backup/Restore/Job、OpsRequest、events、脚本输出和业务自检命令结果。只保存截图、只保存最终 Succeed 状态、或只保存某个 Pod Ready，都不足以审计能力合同。

失败用例也属于验收。参数非法、Secret key 缺失、ServiceRef 不存在、BackupRepo not ready、ActionSet 不可用、target role 不存在、restore artifact 缺失、scale-in memberLeave 失败、preTerminate target Pod 消失，这些问题应能给出可诊断错误和停止条件。没有负向用例时，addon 很容易把 controller 可接受字段误写成产品能力。

## 14. 高风险能力验收矩阵

字段存在但消费者不完整、语义不清或缺少真实 addon 证据的，只能写成“未证明”“需要验证”或进入 gap/ambiguity。

### 14.1 Restore、Rebuild 和 scaleOut 要按消费者建矩阵

release-1.1 同时存在 `Restore`、`RebuildInstance` 和 `HorizontalScaling.scaleOut.fromBackup`。它们可以共享 Backup、ActionSet 和 artifact，但不是同一个能力。一个 method 能 restore 到新集群，不等于能 rebuild 原实例，也不等于能用于 scaleOut 新副本。

| 消费者 | release-1.1 入口 | 必须证明的对象链 | addon 必须自证的业务结果 | 不能外推的结论 |
| --- | --- | --- | --- | --- |
| 新集群 Restore | `dataprotection.kubeblocks.io/v1alpha1 Restore` | Backup、ActionSet restore、prepareData、ReadyConfig/postReady、Restore Job/PVC | 目标 Cluster/Component ready，数据可读写，账号/TLS 可用 | 不证明 RebuildInstance、scaleOut 或跨 topology restore |
| PITR Restore | Restore `restoreTime` + continuous/PITR backup | base backup、parent chain、timeRange、archive artifact、restoreTime 格式 | 恢复到目标时间点，业务数据符合预期 | 不证明任意时间点都有可用归档 |
| K8s 资源恢复 | Restore `resources.included` | 被 include 的 GroupResource、labelSelector、恢复后的 K8s 对象 | 资源对象恢复成功 | 不等于数据库数据恢复成功 |
| RebuildInstance | OpsRequest `rebuildFrom` | backupName、sourceBackupTargetName、restoreEnv、目标 instance、inPlace/replace、postReady/memberJoin | 被 rebuild 实例数据正确并重新加入复制组 | 不证明逻辑备份可用于多副本 rebuild |
| scaleOut.fromBackup | OpsRequest `horizontalScaling.scaleOut.fromBackup` | backup、newInstances/replica change、restore prepareData、memberJoin | 新实例从备份拉起并加入成员列表 | 字段只对非 sharding component 有效；不证明 shard scale 或 rebalance |

### 14.2 网络、Service、RBAC 和 worker 执行面要分开验

release-1.1 的主 Pod、lifecycle action、backup Job、restore Job、postReady Job 不是同一个执行面。主 Pod 能启动、Service 能连通、Secret 能读取，都不能自动外推到其它 worker。

| 执行面 | 需要检查 | release-1.1 证据入口 | 常见错误 |
| --- | --- | --- | --- |
| runtime Pod | PodSpec、volumeMount、env、SA、ports、hostNetwork、hostNetworkVarRef | CMPD runtime、生成 Pod | 脚本读取 host IP/port，但 Service 或 env 格式不同 |
| Service/Endpoint | Service type、selector、roleSelector、podService、EndpointSlice、publish/not-ready 行为 | CMPD services、生成 Service/EndpointSlice | Service 有 endpoint 但 role 不对或 advertised address 不对 |
| ServiceRef | declaration、Cluster binding、ServiceDescriptor、credential、namespace | service reference resolver、ServiceDescriptor controller | 跨 namespace credential 被禁止或 Secret key 未校验到业务可用 |
| lifecycle action | target、container/image、volumeMount、SA/RBAC、timeout/retry | lifecycle executor、kbagent、action output | 默认容器或工具镜像与 runtime 版本不一致 |
| backup worker | BackupPolicy serviceAccountName、ActionSet env、BackupRepo mount/tool secret | Backup/ActionSet/BackupRepo controller 和 Job | runtime TLS/账号未传递到 backup 工具 |
| restore worker | Restore serviceAccountName、prepareData scheduling、ReadyConfig、postReady | Restore API 和 restore manager | prepareData 成功但 postReady 或业务连接失败 |
| RBAC | Component `policyRules`、runtime SA、Job/action SA | final Pod/Job SA、Role/RoleBinding、event | 组件 Pod 有权限，restore/action Job 没权限 |

### 14.3 Controller 修改边界

如果一个问题只能通过修改 KB controller 才能让单个 addon 跑通，但 API 合同、生成对象或业务自检证据不足，应先判定为 addon 合同问题、ambiguity 或 gap。只有 API 合同证据、controller 实现证据、生成对象证据和业务自检证据同时指向 controller 语义错误，才能讨论 controller 修改。

判断 controller 修改前至少回答四个问题：字段语义是否能从 API 类型或注释推出；controller 当前实现是否违反该语义；真实 addon 是否按语义正确声明；运行时对象和业务自检是否证明错误发生在 controller 而不是 chart/script。只证明“某个样例改 controller 后跑通”不是合同证据。
