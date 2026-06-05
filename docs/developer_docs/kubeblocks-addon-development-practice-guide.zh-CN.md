# KubeBlocks Addon 开发实践指南

本文面向 addon 开发者和负责开发 addon 的 agent。它不是 API reference，而是一份实践指南：从设计一个完整数据库 addon 的过程出发，解释应该如何组合 KubeBlocks core API、真实 addon chart 模式和控制器实现。

真实参考样本来自 `kubeblocks-addons` 仓库中的 addons。当 API 注释不够明确时，以 `kubeblocks` 仓库的 core controller 和 `pkg` 实现为准；仍然不确定或实现不完备的内容记录在 [addon-api-ambiguities.zh.md](addon-api-ambiguities.zh.md) 和 [addon-api-gaps.zh.md](addon-api-gaps.zh.md)。

## 1. 各环节都要先写清楚合同

addon 开发不是把 YAML 填满，而是为每个能力建立一组可验证合同。下面这张表是开发和 review 的主线。每个环节都要回答：addon 作者声明了什么、KB controller 实际消费什么、运行时应该看到什么、失败时先查 addon 还是查 KB。

| 环节 | addon 必须声明的合同 | controller 主要消费点 | 常见误判 | 先查什么 |
| --- | --- | --- | --- | --- |
| Chart 组织 | helper 统一生成名称、label、selector、模板引用 | Helm 渲染后的 core CR | 样本 values 能跑就认为名称规则正确 | `helm template` 后所有引用是否闭合 |
| ComponentDefinition | runtime、volumes、configs、scripts、services、roles、vars、accounts、TLS | component transformer 和 synthesized component | 把动态资源、运维状态塞进 CMPD | status Available、volumeMount/volume/config/script 名称是否对齐 |
| ClusterDefinition | topology、component/sharding 组成、创建/删除/更新顺序 | cluster normalization 和 topology 解析 | 未设置 default 却假设选第一个 topology | resolved componentDef、orders、default topology |
| 版本和匹配 | chart version、ComponentDefinition name、serviceVersion、ComponentVersion 关系 | componentDef matching、componentVersion compatibility | 用宽正则碰巧匹配“最新” | 每个 BPT/PD/ClusterDefinition 是否引用同一套 helper |
| 配置参数 | config template name、template ConfigMap、fileName、schema、static/dynamic 分类 | parameters controller、reconfigure controller | 字段名相似就当成同一个对象 | PD、CMPD configs、template ConfigMap、ComponentParameter、runtime ConfigMap |
| 变量和服务 | vars 来源、值格式、Service 类型、ServiceRef 绑定 | var resolver、service builder | 脚本私下约定变量格式，README 不写 | 渲染 env/action vars、service/podService、外部访问模式 |
| 账号和 TLS | systemAccounts、accountProvision、secret 使用边界、证书路径 | component definition validation、lifecycle action | 账号存在就等于业务权限可用 | secret、accountProvision 输出、备份/探测/复制账号实际可用性 |
| lifecycle | roleProbe、memberJoin/Leave、switchover、preTerminate 的业务语义 | lifecycle executor、kbagent、component deletion | 脚本能执行一次就等于幂等可靠 | action 输入变量、目标 Pod、重试、失败状态、资源是否还存在 |
| 内置 Ops | 每种 Ops 的底层前提 | operations pkg 和各 domain controller | OpsRequest 失败就只看 operations 层 | restart/scale/reconfigure/backup/restore/switchover 各自依赖的底层合同 |
| 数据保护 | BPT、ActionSet、backup methods、target role、脚本职责 | BPT driver、backup/restore controller | 只有 ActionSet 就声称支持备份 | BackupPolicy 是否生成、method/actionSet 是否一致、target 是否选中正确 Pod |
| Restore/Rebuild | 恢复目标、版本/拓扑/TLS/账号兼容性、prepareData/postReady | restore manager、operations rebuild flow | Backup 成功就等于 Restore 成功 | Restore phase、PVC/Job、数据校验、重建实例路径 |
| Upgrade/Rollout | 版本矩阵、镜像映射、兼容限制、更新顺序 | ComponentVersion、rollout、workload controller | 镜像能拉起就等于升级支持 | old/new serviceVersion、rollout 状态、数据兼容、回滚限制 |
| Sharding | shard 模板、跨 shard 变量、scale shard 后处理、数据迁移语义 | sharding/component transformers | 多副本等同于 sharding | ShardingDefinition、shard lifecycle、rebalance 或限制说明 |
| 操作排障 | 创建、删除和 day-2 ops 的资源链路 | cluster/component/operations/dataprotection 等控制器 | 只看最后一个报错 | 从入口 CR 到底层对象逐层查 |

这张表用于开发和 review：每新增一个能力，都要能在对应行里说明对象关系、controller 消费方式、运行验证和限制边界。

## 2. 先定义 addon 的工作边界

实践中，一个 addon 首先是一个 Helm chart，而不是一个单独 CR。真实 addon 通常包含 `Chart.yaml`、`values.yaml`、`templates/`、`configs/`、`scripts/` 和 README。例如 MySQL、PostgreSQL、Redis、ClickHouse、Kafka、Etcd 都采用这种组织方式。

本指南用一个合成 addon `acmedb` 作为叙述主线。`acmedb` 是一个有主从复制、可选 proxy、可选 sharding、支持参数变更和备份恢复的数据库。它不是新增到仓库里的真实 chart；它用于把真实 addon 中已经存在的模式串成一条完整开发路径。

addon 作者开始前应先回答这些问题：

- 引擎需要几个组件。单进程数据库可以只有一个 `ComponentDefinition`；带 proxy、keeper、controller/broker 或 sentinel 的系统应拆成多个组件。真实样例包括 Kafka 的 controller/broker、ClickHouse 的 clickhouse/keeper、Redis 的 redis/sentinel/twemproxy、MySQL 的 mysql/proxysql。
- 是否需要多个拓扑。`ClusterDefinition` 应表达 standalone、HA、proxy、sharding 等用户可选拓扑，而不是把拓扑差异塞进脚本。
- 哪些能力是内置 day-2 操作可以覆盖。扩缩容、重启、配置变更、备份恢复、切主、重建等优先通过内置能力完成；超出内置能力的需求应先记录边界和缺口，不在本指南中作为推荐路径展开。
- 哪些功能只能记录为限制。当前 API 无法自然表达的需求不要用脚本和隐藏约定硬拼，应写入 gap 文档。

推荐 chart 结构：

```text
addons/acmedb/
  Chart.yaml
  values.yaml
  templates/
    clusterdefinition.yaml
    cmpd-storage.yaml
    cmpd-proxy.yaml
    cmpv-storage.yaml
    paramsdef.yaml
    backuppolicytemplate.yaml
    actionset.yaml
    config-templates.yaml
    script-templates.yaml
  configs/
    acmedb.conf.tpl
    constraints.cue
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
```

真实依据：MySQL 的 `templates/clusterdefinition.yaml`、`templates/cmpd-mysql80.yaml`、`templates/cmpv.yaml`、`templates/backuppolicytemplate.yaml`、`templates/actionset-xtrabackup.yaml`；PostgreSQL 的 `templates/cmpd.yaml`、`templates/cmpv.yaml`、`templates/paramsdef.yaml`；Redis 的 `templates/cmpd-redis.yaml`、`templates/cmpv-redis.yaml`、`templates/cmpv-redis-sentinel.yaml`；ClickHouse 的 `templates/cmpd-ch.yaml`、`templates/cmpv.yaml`；Etcd 的 `templates/cmpd.yaml`、`templates/cmpv.yaml`。

chart 层还要先做闭合检查，再谈运行语义。`helm lint` 只能证明一部分语法；review 时必须用代表性 values 渲染 standalone、HA、proxy、sharding、TLS on/off、外部 ServiceRef、不同 serviceVersion 等组合，确认所有 `include` helper、ConfigMap key、script key、`templateName`、`compDef`、BPT `compDefs`、PD `componentDef/templateName/fileName` 和 examples 中的名字都能闭合。helper 名称要带 addon 前缀，不能让多个模板文件各自拼接同一个资源名；否则单个样例能跑，换 topology 或版本后就会出现引用漂移。

历史 issue 反复暴露同一个问题：topology 不是 README 里的展示分类，而是必须单独验收的支持维度。只要 addon 声称支持 `standalone`、`replication`、`mgr`、`mgr-proxy`、`sharding`、`sentinel`、`keeper`、`proxy` 等 topology，就要分别证明该 topology 的 ComponentDefinition、ClusterDefinition、ComponentVersion、ParametersDefinition、BackupPolicyTemplate、ActionSet、Service/ServiceRef、systemAccounts、TLS、examples 都能闭合。某个 topology 的 Cluster 能 Running，不代表它的 BackupPolicy 会自动生成、proxy 会路由到 writer、roleSelector Service 会选到正确 Pod，或参数和 restore 能继续工作。

发布和升级还要单独做资源治理。多数 definition、policy、action 类资源是 cluster-scoped 或被跨 namespace 引用的公共定义，名称不应把安装 namespace 拼进去；多 chart、多 release 共存时靠 addon 前缀、引擎大版本和稳定 helper 隔离，避免两个 namespace 安装同一个 addon 时互相污染。升级前后必须比较 `helm template` 新旧结果、server-side dry-run 结果和 apiserver 中的最终对象，不能只相信本地渲染或客户端 schema。被 values 关闭的功能也要有迁移策略：旧 `ComponentDefinition`、`ClusterDefinition`、`ComponentVersion`、`ParametersDefinition`、`BackupPolicyTemplate`、`ActionSet`、已生成 `BackupPolicy`、`BackupSchedule` 和示例测试资源，是保留、删除、改名还是标为不再支持，都要在 release checklist 里写清楚。否则旧资源仍可能被 ComponentVersion 匹配、BackupPolicy 继续消费，或者测试 ActionSet 混入发布包后被真实 BackupJob 使用。

示例验收要看 server 端最终对象。推荐在每个关键 example 上执行 server-side dry-run，再把 dry-run 输出、实际 apply 后对象和下游生成的 Component/BackupPolicy/ComponentParameter 做 diff。若 dry-run 与实际对象不同，应以 apiserver 最终对象和 controller 生成物为准定位问题；本地 `helm template` 只能作为输入证据，不能证明 admission/defaulting/mutation 后的合同仍闭合。

发布迁移要先把字段分成“可 patch”“只能创建期决定”“需要新建对象迁移”三类。至少应把下面这些 Cluster 层字段列入 release checklist：

| 字段或对象关系 | 迁移原则 | 验收证据 |
| --- | --- | --- |
| Cluster `spec.clusterDef`、`spec.topology` | 不把拓扑切换设计成在线 patch；需要新建 Cluster、数据迁移或明确不支持 | 新旧 Cluster 的 topology、Component 列表、PVC/Backup/Restore 映射表 |
| Cluster `spec.restore` | 视为创建期恢复入口；恢复源写错、Restore 阶段失败或 source target 选错时，优先新建恢复 Cluster 重试 | 源 Backup namespace/name/UID、Restore Job/PVC、目标 Cluster status 和清理策略 |
| `componentSpecs[*].name/componentDef` 和 sharding 名称 | 组件身份变更等同迁移，不用改名 patch 伪装成原组件 | live Cluster spec、生成的 Component 名、InstanceSet/PVC ownerRef |
| 已生成 `BackupPolicy`、`BackupSchedule`、`ComponentParameter` | 默认 repo、method、schedule 或参数模板变更不必然自动刷新旧对象 | 生成对象的当前 spec/status，而不是 chart 新默认值 |

GitOps 和 server-side apply 要单独防 list 字段风险。`componentSpecs`、`shardings`、`services`、`configs`、`schedules` 这类列表可能有 merge key、retainKeys 或 controller 默认值；Git diff 中“删掉一项”不等于 apiserver 最终对象一定删除，反过来 apply 后的 retained 字段也可能继续被 controller 消费。发布前用 server-side dry-run、`kubectl diff --server-side`、live spec 和下游对象四层对照：输入 manifest、apiserver 最终对象、controller 生成对象、业务自检。最终判定只看 live 对象和生成物，不看 Git 仓库里某个 YAML 片段的意图。

## 3. 用 ComponentDefinition 表达组件蓝图

`ComponentDefinition` 是 addon 作者最重要的资源。它描述组件的静态蓝图：runtime PodSpec、配置模板、脚本、服务、变量、账号、TLS、角色、生命周期动作、更新策略和 RBAC。Cluster 或 Component 只提供实例化时的动态参数，例如副本数、资源、存储、调度和用户选择的 serviceVersion。

最小可运行组件需要这些内容：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ComponentDefinition
metadata:
  name: acmedb-storage-1.0.0
spec:
  provider: acme
  serviceKind: acmedb
  serviceVersion: 1.0.0
  runtime:
    containers:
      - name: acmedb
        image: acme/acmedb:1.0.0
        command: ["/scripts/start.sh"]
        ports:
          - name: client
            containerPort: 5432
        volumeMounts:
          - name: data
            mountPath: /var/lib/acmedb
          - name: scripts
            mountPath: /scripts
  volumes:
    - name: data
      needSnapshot: true
  scripts:
    - name: acmedb-scripts
      template: acmedb-scripts
      volumeName: scripts
      defaultMode: 0555
```

实践规则：

- `runtime` 放引擎必需且跨实例稳定的 PodSpec。CPU、内存、PVC 大小、调度等动态内容留给 Cluster。
- `volumes[*].name` 必须和 runtime 中的 volumeMount 名称、Cluster 的 `volumeClaimTemplates[*].name` 对齐。否则用户写了 PVC 也不会挂载到预期路径。
- `scripts` 和 `configs` 应引用 chart 渲染出的 ConfigMap。真实 addon 通常把启动、探测、成员变更、重载、备份脚本拆成独立文件，再通过模板挂载。
- `services` 用来声明 KB 管理的服务形态。常规 client service、roleSelector service、podService、disableAutoProvision 都应在组件蓝图中声明，再允许用户在 Cluster 侧按需覆盖。

ComponentDefinition 的合同检查：

- 每个 `runtime.containers[*].volumeMounts[*].name` 都能在 `spec.volumes`、`spec.configs[*].volumeName` 或 `spec.scripts[*].volumeName` 中找到来源。
- 每个脚本路径都来自 mounted script volume，脚本 ConfigMap 的 key、defaultMode 和容器 command 对齐。
- 每个 service 端口都能对应 runtime container port；roleSelector service 依赖的 role 必须由 roleProbe 产生。
- 每个声明的 account、TLS、var 都要能在启动脚本、lifecycle action 或 backup/restore 中找到使用点。
- `status.phase=Available` 是被引用前的最低门槛；不是仅看 CR 创建成功。

控制器实现要求 `ComponentDefinition` 被引用时必须 `status.phase=Available`，且已观测最新 generation。实现依据见 `controllers/apps/component/transformer_component_load_resources.go`。这意味着实践中不能只检查资源存在，还要检查 status。

所有会被其它对象引用的 definition、policy 和 action 资源都按同一套可用性口径排查：先确认入口对象和被引用对象在同一预期 namespace 或显式 source namespace 下，再看 `generation/observedGeneration`、`phase/message/conditions` 是否指向同一轮 reconcile，最后看引用方生成的 Component、BackupPolicy、ComponentParameter、Job 或事件是否已经消费到新对象。同名删除重建、`phase=Available` 但 message/condition 仍残留旧错误、direct `componentDef` Cluster 与无关 ClusterDefinition 报错并存时，都不要只凭对象名下结论；要回到引用关系和最终生成物取证。

默认资源和数据目录初始化也是 ComponentDefinition 合同的一部分。values 里的 CPU、memory、storage、ulimit、文件句柄、临时目录和默认 JVM/进程参数必须满足引擎最低启动要求；否则 Pod OOMKilled、No space left on device 或启动探针失败会被误判成 KB 创建失败。启动脚本要区分真正空数据目录、只包含 `lost+found` 这类文件系统目录、已有业务数据、restore/rebuild 写入的数据和半初始化目录；不要用简单的“目录为空/非空”判断决定是否初始化或清空数据。

写完 ComponentDefinition 后，必须立刻写一个最小 Cluster 样例来证明闭环。不要等到 HA、备份、参数都写完才发现基础对象没有实例化成功：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: acmedb-smoke
spec:
  clusterDef: acmedb
  topology: ha
  terminationPolicy: Delete
  componentSpecs:
    - name: storage
      replicas: 1
      serviceVersion: 1.0.0
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes: [ReadWriteOnce]
            resources:
              requests:
                storage: 20Gi
```

这个样例要验证四个闭合点。第一，`componentSpecs[*].name` 等于 ClusterDefinition topology 里的 component name；如果绕过 ClusterDefinition 直接在 Cluster 里写 `componentDef`，则所有 componentSpecs 必须采用同一种写法，不能有的依赖 topology、有的直接指定 ComponentDefinition。第二，`serviceVersion` 在开发样例里要显式写出，避免未指定时 controller 选择最高语义版本导致误判。第三，`volumeClaimTemplates[*].name` 必须等于 CMPD `spec.volumes[*].name` 和容器 `volumeMounts[*].name`。第四，脚本和配置 volume 要能在 Pod 里看到对应文件和权限。

最小验证链路是：

```bash
kubectl get cluster acmedb-smoke -o yaml
kubectl get component -l app.kubernetes.io/instance=acmedb-smoke -o yaml
kubectl get componentdefinition acmedb-storage-1.0.0 -o yaml
kubectl get instancesets,pods,pvc,svc -l app.kubernetes.io/instance=acmedb-smoke
kubectl describe pod -l app.kubernetes.io/instance=acmedb-smoke
```

从 Component 往下排查时，固定按 `Component -> InstanceSet -> Instance -> Pod -> PVC` 看实例身份。Component `Running` 不等于 InstanceSet `InstanceAvailable=True`；Pod Ready 后还可能受 `minReadySeconds`、availableProbe、roleProbe 或 update strategy 影响。InstanceTemplate 名称、默认补齐的 instances、flat ordinal、PVC 命名和 ownerRef 都会影响身份连续性；README 和排障步骤不要建议用户手工修改 KB 系统 label/selector，也不要用连续 ordinal 猜测实际 Pod/PVC。滚动类问题要看 InstanceSet `current/updatedReplicas`、revision、partition/canary/OnDelete 等状态，再判断是否真正完成 rollout。

ComponentDefinition 中有几组字段特别容易被误读，应按下面的字段语义矩阵 review：

| 字段 | addon 为什么要填 | 必须同时对齐 | 误用表现 |
| --- | --- | --- | --- |
| `spec.volumes[*].name` | 给数据卷、snapshot 和 volume expansion 建立稳定名字 | runtime `volumeMounts[*].name`、Cluster `volumeClaimTemplates[*].name`、BPT `targetVolumes` | PVC 创建了但没挂载到数据库目录；volume expansion 找不到模板 |
| `spec.volumes[*].needSnapshot` | 标识 volume snapshot backup 的候选数据卷 | BPT `backupMethods[*].snapshotVolumes` 和 snapshot method 文档 | 声称支持 snapshot，但 BPT 和 volume 没有绑定 |
| `spec.volumes[*].highWatermark` | 容量到阈值时切只读/恢复读写 | `lifecycleActions.readonly/readwrite`、引擎实际只读命令 | 到达阈值后 action 不存在或脚本不幂等 |
| `spec.configs[*].name` | 参数链路绑定键 | `ParametersDefinition.spec.templateName`、Cluster `configs[*].name` | 参数 controller 解析不到 config item |
| `spec.configs[*].template` | 模板 ConfigMap 名 | chart `config-templates.yaml` 的 ConfigMap 名 | 模板文件不存在，Pod 挂载空或渲染失败 |
| `spec.configs[*].volumeName` 和 `spec.scripts[*].volumeName` | 决定文件挂到哪个 volume | runtime volumeMount、脚本路径、`defaultMode` | command 找不到脚本或权限不对 |
| `spec.services[*].roleSelector` | 只把流量指向某类角色 | `roles`、`roleProbe`、role label | 服务选不到 Pod，或切主后服务仍指向旧角色 |
| `spec.services[*].podService` | 每个 Pod 一个 Service | serviceVarRef 输出格式、外部访问脚本 | 误以为 roleSelector 仍生效；实际生成 `serviceName-ordinal` 多个 Service |
| `spec.services[*].disableAutoProvision` | 定义可暴露能力但默认不创建 | Cluster/Ops expose 显式启用 | addon 里声明了服务，运行时却找不到 Service |
| `spec.replicasLimit` | 给 horizontal scale 定边界 | README、examples、scale e2e | 用户 scale 到业务不支持的副本数 |
| `spec.available.withPhases/withRole/withProbe` | 定义 Component Available 判定 | readiness、roleProbe、availableProbe | Pod Ready 但 Component 不 Available，或反过来 |
| `spec.hostNetwork.containerPorts` | hostNetwork 动态端口分配 | container port 名、`hostNetworkVarRef`、advertised address 脚本 | hostNetwork 启动后脚本仍用固定端口 |
| `podUpdatePolicy`、`podUpgradePolicy`、`instanceUpdateStrategy` | 控制规格变更和升级 rollout | `roles.updatePriority`、quorum、switchover | 升级顺序破坏可用性，或 in-place/recreate 预期不一致 |

`defaultMode` 要按最终 Pod 和容器内文件权限验收。YAML、Helm values、JSON 和 Go template 对数字的处理不完全相同，`0555`、`365`、字符串再转整数这些写法容易在渲染链路里被误读。脚本和配置 volume 的权限验收应同时看渲染后的字段、Pod spec 中的 `defaultMode`，以及容器内 `stat` 看到的八进制权限；不能只看源码里写了一个看起来像八进制的数字。

`replicasLimit` 是 addon 声明的 scale 支持范围，不是业务已经支持所有边界的证明。声明后至少要验收最小值、最大值、越界拒绝、HorizontalScaling Ops、README 示例和 0 副本语义。若引擎在 `replicas=0` 时没有可用入口、不能保留复制成员语义或不支持从 0 恢复，文档必须写成不支持或需要人工步骤；不要用 Cluster phase 推断业务入口可用。

## 4. 用 ClusterDefinition 表达拓扑，不表达组件细节

`ClusterDefinition` 当前主要表达拓扑：有哪些 component 或 sharding、默认拓扑是什么、组件创建/删除/更新顺序是什么。组件的 runtime、账号、脚本和配置不应该放在这里。

`acmedb` 可以这样组织拓扑：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ClusterDefinition
metadata:
  name: acmedb
spec:
  topologies:
    - name: ha
      default: true
      components:
        - name: storage
          compDef: ^acmedb-storage-.*
    - name: proxy
      components:
        - name: storage
          compDef: ^acmedb-storage-.*
        - name: proxy
          compDef: ^acmedb-proxy-.*
      orders:
        provision: [storage, proxy]
        terminate: [proxy, storage]
        update: [storage, proxy]
```

实践规则：

- 每个用户可选部署形态都应该有明确 topology。MySQL 用 `semisync`、`mgr`、`orc-proxysql` 等 topology 表达差异；这比让用户猜组件组合可靠。
- 多组件拓扑必须声明顺序。proxy 通常在 server 后创建、先删除；controller/keeper 类组件要根据真实依赖排序。
- `default: true` 不是装饰字段。实现中未看到“没有 default 就选第一个 topology”的回退，Cluster 未指定 topology 时依赖默认 topology。
- `compDef` 支持精确名、前缀和正则，但多匹配的选择需要谨慎。ComponentDefinition 匹配的实现会按 serviceVersion 和名称排序选择；命名规则必须稳定。
- Cluster 有两种实例化写法：通过 `clusterDef/topology` 解析 component，或在 `componentSpecs[*].componentDef` 里直接指定 ComponentDefinition。一个示例或 README 场景里不要混用这两种写法；混用时 component name、orders、default topology 和 direct componentDef 的责任边界会变得不可审计。

ClusterDefinition 的合同检查：

- 每个 topology 都有用户可理解的业务含义，不能只是为了复用模板拼出来的名字。
- 多 topology 只能有一个 default；没有 default 时不能假设 controller 会选择第一个。
- 每个 topology 的 component name 都要和 Cluster 示例里的 `componentSpecs[*].name` 对齐。直接指定 `componentDef` 的样例应显式说明绕过了 ClusterDefinition topology，因此不能再引用 topology orders 或 topology component name 作为依据。
- topology 中的普通 component 名和 sharding 名不要复用同一个名字。确实无法避免时，必须用渲染结果、Cluster/Component status、Service selector 和变量输出证明 orders、componentSelector、vars 都没有歧义。
- `orders.provision/terminate/update` 要覆盖跨组件依赖，尤其是 proxy、keeper、controller/broker、sentinel 这类辅助组件。`orders` 中每个元素是一个阶段；同一字符串里用逗号分隔的对象表示可并行阶段，阶段之间按数组顺序执行。
- `compDef` 正则要能稳定命中唯一预期集合；若同一正则可能命中多个版本，必须同时说明 serviceVersion 或版本选择策略。
- sharding 拓扑要说明 shard 增删时的数据迁移、rebalance 或明确限制，不能只声明多个 shard component。
- ClusterDefinition 的 default topology 也要看 status。多个 `default: true` 或 default 缺失这类问题不应只期待 admission 阻止；review 时先查 ClusterDefinition `status.phase/message` 是否 Available，再查 Cluster normalize 后的 topology 选择。

真实依据：MySQL `templates/clusterdefinition.yaml` 把 server/proxy 顺序写在 topology；Redis 和 ClickHouse 分别用 sharding 和多组件定义表达复杂部署形态。

什么时候用 component，什么时候用 sharding：多副本不是 sharding。一个 component 的 `replicas` 表示同一复制组里的多个实例；sharding 表示多个 shard component，每个 shard 可以再有自己的 replicas。只有当用户需要增加或减少 shard 数、跨 shard 组织变量、或者每个 shard 都是一套独立 Component 时，才应引入 `ShardingDefinition`。

ClickHouse、Redis cluster 这类场景可以用下面的最小形状表达：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ShardingDefinition
metadata:
  name: acmedb-shard
spec:
  template:
    compDef: ^acmedb-storage-.*
  shardsLimit:
    minShards: 1
    maxShards: 32
  provisionStrategy: Serial
  updateStrategy: Serial
  lifecycleActions:
    shardAdd:
      exec:
        container: acmedb
        command: ["/scripts/shard-add.sh"]
    shardRemove:
      exec:
        container: acmedb
        command: ["/scripts/shard-remove.sh"]
---
apiVersion: apps.kubeblocks.io/v1
kind: ClusterDefinition
metadata:
  name: acmedb
spec:
  topologies:
    - name: sharding
      shardings:
        - name: shard
          shardingDef: acmedb-shard
```

Cluster 样例应通过 `spec.shardings[*].name`、`shards`、`template.replicas`、`template.volumeClaimTemplates` 来实例化 shard，而不是在 `componentSpecs` 里手写多个同名模式的组件。sharding template 里的 `name` 不应被脚本或变量当作真实 component 名；实际 shard component 的 spec name 来自 sharding name 和 shardID，底层对象名还可能带 cluster 前缀。sharding 场景下要同时打印或检查 componentName、shortName、Pod FQDN 和 Service endpoint，避免模板名、spec 名和对象名漂移。

`ShardingDefinition.spec.lifecycleActions.shardAdd/shardRemove` 只给新增或删除 shard 的 hook；它不自动证明 rebalance、数据迁移、slot 迁移或跨 shard 元数据修复已经完成。真实 ClickHouse 里存在 `post-for-scale-out-shard.sh` 这类补偿脚本，说明 shard scale-out 后的业务收敛必须逐 addon 证明。无法证明的数据迁移和 rebalance 要写成限制或 gap。`offline` shard、ClusterService `componentSelector` 指向 sharding、`minShards=0` 这类边界不应在 guide 里写成固定 endpoint 或 Available 语义；addon 必须用 status、Service endpoints 和 e2e 说明是否支持，或避免作为公共样例。

名字和身份要拆开记录。`componentSpecs[*].name`、sharding spec name、生成的 Component 对象名、status map key、Pod 名、Pod FQDN、selector label 和 PVC 名不是同一种身份；同名重建后 Cluster UID 也会变化。脚本可以把这些值作为运行时输入，但不要把 `clusterUID`、Pod 名、generated component name 或连续 ordinal 写入不可重算的数据库成员 ID、持久配置或备份元数据中。确实需要持久化成员身份时，应使用引擎自己的成员 ID 或可从业务元数据重建的映射，并在恢复、重建、sharding scale 和同名 Cluster 重建时验收映射仍然成立。

## 5. 用 ComponentVersion 管理版本矩阵

addon 的版本管理有四层容易混淆：chart 版本、ComponentDefinition 名称、ComponentVersion release、引擎 serviceVersion。实践中应让它们可追溯但不要混为一谈。

推荐做法：

- `ComponentDefinition.metadata.name` 包含引擎大版本或 chart 版本，例如 `acmedb-storage-1.0.0`。
- `spec.serviceVersion` 表示数据库内核或服务协议版本，例如 `1.0.0`。
- `ComponentVersion` 维护同一类组件的 release 矩阵：哪些 ComponentDefinition 可用哪些 release，每个 release 对应哪个 serviceVersion，以及容器、action 或外部应用镜像如何替换。
- helper 中统一生成 componentDef 名称、前缀和正则，避免 README、ClusterDefinition、BackupPolicyTemplate、ParametersDefinition 各写一套。

`acmedb` 可以这样声明一个 storage 组件版本矩阵：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: ComponentVersion
metadata:
  name: acmedb-storage
spec:
  compatibilityRules:
    - compDefs:
        - ^acmedb-storage-.*
      releases:
        - acmedb-1.0.0
        - acmedb-1.1.0
  releases:
    - name: acmedb-1.0.0
      serviceVersion: 1.0.0
      images:
        acmedb: acme/acmedb:1.0.0
    - name: acmedb-1.1.0
      serviceVersion: 1.1.0
      images:
        acmedb: acme/acmedb:1.1.0
```

不要把 `compDef: acmedb` 这种宽前缀作为长期约定。真实实现中 ComponentDefinition 多匹配会选择某个“最新”候选，但这不是给 addon 作者逃避版本治理的理由。

版本合同检查：

- README、values、ClusterDefinition、ParametersDefinition、BackupPolicyTemplate、examples 必须引用同一套 helper 生成的名字。
- 每个 serviceVersion 都要说明可用镜像、兼容的配置 schema、备份恢复方法和升级路径。
- `ComponentVersion.spec.compatibilityRules[*].compDefs` 支持精确名、前缀或正则；release 名必须能在 `spec.releases[*].name` 中找到。
- `ComponentVersion.spec.releases[*].serviceVersion` 会作为被选择 release 的 serviceVersion，覆盖 ComponentDefinition 自身的 serviceVersion 参与 Cluster 解析。
- `images` 的 key 要对应 ComponentDefinition runtime container、init container 或 lifecycle action 字段名。lifecycle action 名匹配大小写不敏感，例如 `switchover` 可以命中 `Switchover` 字段；未知 key 目前不会强制失败，不能把它当成已验证的镜像替换。
- action image fallback 的边界要写清楚：ComponentVersion 可以覆盖有 exec image 解析路径的 lifecycle action 镜像；但某个 action 没有独立 image key、没有 `exec.image`、或脚本依赖 runtime/init 产物时，不能仅凭 runtime 镜像升级推断 action 也已升级兼容。升级验收必须分别检查 runtime container image、init container image、kbagent/worker 执行 action 时使用的镜像和脚本挂载。
- ComponentVersion controller 会校验 serviceVersion 语义版本、compDef pattern 和 release 是否至少匹配到 ComponentDefinition；实践中还要检查 `status.phase=Available` 和 `status.serviceVersions`。
- release 名、compatibilityRules 或 compDef pattern 出错时，第一诊断入口是 ComponentVersion 的 `status.phase`、`status.message` 和 `status.serviceVersions`。不要只看 `spec.releases` 是否存在相近名字，也不要把 Cluster upgrade 失败直接归因到 Ops 层。
- Cluster 未指定 serviceVersion 时，normalization 会在 ComponentDefinition 自身 serviceVersion 和兼容 ComponentVersion releases 中选择最高语义版本；这会影响最终 resolved componentDef 和 serviceVersion。
- 每个 serviceVersion 都应有独立 Cluster 样例或 e2e，证明 resolved image、serviceVersion、参数定义、备份方法和升级路径一致。

升级发布要把“已完成的事实”和“未来可用性”分开。一次 Upgrade 已经完成，只能说明当时解析到的 ComponentVersion、镜像和 rollout 路径可用；如果之后新的 `ComponentVersion` 或相关 `ComponentDefinition` 变为 `Unavailable`、被 Helm 删除或被旧残留资源遮蔽，已运行 Pod 不一定立刻回滚，但后续 scale、restart、rebuild、backup method 选择或再次 upgrade 都可能重新消费这些 definition。排查时先保留旧版和新版的 server-side dry-run 结果，再看 apiserver 中残留的 CMPD/CMPV/PD/BPT/ActionSet、Cluster/Component resolved serviceVersion、Pod image 和 action image。不要把“当前业务还能连上”解释成版本资源仍健康。

升级闭环不能只看新镜像能启动。至少要准备 old/new 两个 Cluster 样例，以及一个最小 upgrade OpsRequest：

```yaml
apiVersion: operations.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: acmedb-upgrade-110
spec:
  clusterName: acmedb-smoke
  type: Upgrade
  upgrade:
    components:
      - componentName: storage
        serviceVersion: 1.1.0
```

验证时要同时看 `ComponentVersion.status.serviceVersions`、Cluster/Component resolved serviceVersion、Pod image、action image、参数 schema、BPT method 和 rollout 顺序。`ParametersDefinition.spec.serviceVersion` 的匹配对象要按当前实现单独验证；不要因为 ComponentVersion release 覆盖了 Cluster resolved serviceVersion，就默认 PD 会按最终 release serviceVersion 选择 schema。数据兼容、回滚限制、跨大版本恢复限制必须写进 README 或实现说明。

版本矩阵还要覆盖工具镜像和镜像来源。runtime image 能启动，不代表 lifecycle action、backup/restore、postReady、ReadyConfig、formatter 或 sidecar 使用的工具镜像与该 serviceVersion 兼容，也不代表目标集群或云区域能拉到这些镜像。对 Elasticsearch、ClickHouse、Kafka、Redis 这类工具链敏感的 addon，每个 serviceVersion 至少要列出 runtime image、init image、action image、backup image、restore image、formatter image、镜像 registry 来源、artifact format 或客户端协议版本；不能验证的组合写成“不支持/未证明”。

## 6. 生命周期动作要按数据库语义建模

角色型或复制型数据库通常需要 `roles`、`roleProbe`、`memberJoin`、`memberLeave`、`switchover`。共识系统还要特别关注 update 顺序和 quorum。`Action` 应尽量幂等，因为 controller 或 worker 重试可能导致重复执行。

`acmedb` 的 HA 组件可以这样声明：

```yaml
spec:
  roles:
    - name: primary
      updatePriority: 2
      isExclusive: true
    - name: secondary
      updatePriority: 1
  updateStrategy: BestEffortParallel
  lifecycleActions:
    roleProbe:
      exec:
        container: acmedb
        command: ["/scripts/role-probe.sh"]
      periodSeconds: 5
    memberJoin:
      exec:
        container: acmedb
        command: ["/scripts/member-join.sh"]
    memberLeave:
      exec:
        container: acmedb
        command: ["/scripts/member-leave.sh"]
    switchover:
      exec:
        container: acmedb
        command: ["/scripts/switchover.sh"]
```

实践规则：

- 如果 `roles` 被 service、update 或 backup target 使用，就必须实现可靠 `roleProbe`。否则 roleSelector service、leader backup、switchover 都会不可信。
- `memberJoin` 用于新副本加入复制组，`memberLeave` 用于副本移出前的数据库内清理，不要把数据迁移、重分片、用户确认等复杂流程藏进一个不可审计脚本。
- `switchover` 用于计划性角色转移。脚本会通过 KB 注入的 switchover 相关环境变量理解当前角色和候选 Pod。实现对有 candidate 和无 candidate 的成功判定不同：指定 candidate 时会继续观察 candidate 的 role 是否达到目标 role；未指定 candidate 时，action 调用成功后即可进入成功路径。因此脚本自身校验和 roleProbe 都必须可靠，不能只依赖 OpsRequest 进入 Succeed。
- 对于补偿性质的动作，比如 scale-out shard 后修复，若不能自然挂到内置生命周期，应记录为能力缺口或明确写出人工运维限制，不要包装成已经由 core API 自然支持。
- `roles[*].isExclusive` 只约束 KB 记录的 role label 视角，不能证明数据库内部没有 split-brain，也不能替代引擎级 fencing、租约或 quorum 校验。role label、业务角色、Service endpoint 和客户端写入成功要分四层验收。

生命周期合同检查：

- `roleProbe` 输出必须稳定且可被 controller 解析；它影响 service、备份 target、更新顺序和 switchover。
- `memberJoin` 和 `memberLeave` 要能重复执行或安全失败，且不要依赖可能已删除的 peer service。
- `switchover` 要明确候选 Pod、当前 primary、失败回滚和状态检查方式。
- `preTerminate` 只适合“Pod 仍存在时执行”的下线动作；如果清理动作必须在 Pod 消失后执行，就不应设计成 preTerminate。
- 每个 action 都要有脚本单测或至少有可复现的 kbagent/action 输出。

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
| `switchover` | 注入 `KB_SWITCHOVER_CURRENT_NAME/FQDN`、`KB_SWITCHOVER_CANDIDATE_NAME/FQDN`、`KB_SWITCHOVER_ROLE`；candidate 为空时可能不注入 candidate 变量 | candidate 为空和非空两条路径都能处理；有 candidate 时 roleProbe 最终能把 candidate 报成目标 role；无 candidate 时脚本要自己确认切换结果 |
| `memberJoin` | 注入 `KB_JOIN_MEMBER_POD_NAME`、`KB_JOIN_MEMBER_POD_FQDN` | 新 Pod Ready 后能加入复制组，重复执行不会破坏已有成员 |
| `memberLeave` | 注入 `KB_LEAVE_MEMBER_POD_NAME`、`KB_LEAVE_MEMBER_POD_FQDN` | 删除前能摘除成员；如果需要数据迁移，要作为单独限制说明 |
| `readonly/readwrite` | 由 volume highWatermark 等容量保护路径触发 | 引擎有真实只读/读写切换命令，且状态可回查 |
| `dataDump/dataLoad` | dump 通过 stdout 输出，load 从 stdin 读取；dump 可获得 `KB_TARGET_POD_NAME` | 只用于实例初始化类数据复制，不等价于 Backup/Restore method |
| `reconfigure` | 文件变更时可获得 `KB_CONFIG_FILES_CREATED/REMOVED/UPDATED` | 脚本按文件名和参数名校验输入，支持幂等重试 |
| `accountProvision` | 注入 `KB_ACCOUNT_NAME`、`KB_ACCOUNT_PASSWORD`、`KB_ACCOUNT_STATEMENT` | SQL/CLI 执行有转义、脱敏和最小权限控制 |
| `Action.targetPodSelector` 和 `matchingKey` | `Role` 时按 `matchingKey` 选 role；`Any/All` 不使用 matchingKey | 依赖 role 的动作必须先证明 roleProbe 可靠 |
| `timeoutSeconds` 和 `retryPolicy` | 非 0 退出、HTTP 非 2xx、超时都会按策略失败或重试 | 脚本要能安全重试；长耗时动作不要用默认超时碰运气 |

`preCondition` 要按脚本依赖选择，不要按“越晚越安全”猜：

| preCondition | 适合场景 | 不适合场景 |
| --- | --- | --- |
| `Immediately` | 只依赖定义对象、Secret 或本地初始化，不需要 Pod Ready | 需要连接数据库、依赖 Service endpoint 或 role 的动作 |
| `RuntimeReady` | 需要容器文件、挂载卷、基础进程或本 Pod endpoint | 需要整个组件达到业务可用或所有成员已加入 |
| `ComponentReady` | 需要当前 Component 的副本、role 或 service 已收敛 | 脚本本身会阻塞 ComponentReady，容易形成 readiness 环 |
| `ClusterReady` | 依赖其它 component，例如 proxy 等待 storage 完成后初始化 | 任何影响 ClusterReady 本身的必经初始化动作 |

action 目标和执行面要拆开验收：

| 动作类型 | targetPodSelector/matchingKey 设计 | 执行位置验收 |
| --- | --- | --- |
| roleProbe/availableProbe | 通常作用于每个候选 Pod，不依赖旧 role | stdout、退出码、周期噪声和 Instance role/status 写入要分开看 |
| memberJoin | 目标是新加入成员或能管理复制组的成员，按引擎语义固定 | 新成员 Ready、复制元数据广播、老成员视图刷新都要验证 |
| memberLeave/preTerminate | 目标成员必须仍存在，或选择能执行摘除命令的管理角色 | scale-in、删除、Stop 后删除分别证明；Pod 已消失时不能假设还能执行 |
| switchover/readwrite/readonly | 通常依赖 `Role` 和 `matchingKey`，candidate 路径另测 | action 成功后还要看 roleProbe、Service endpoint 和引擎状态 |

HTTP/GRPC action 要把接口当成版本化合同。HTTP action 至少固定 method、path、headers、body、成功状态码范围和 body assertion；`204` 无 body 时不要写依赖 body 的断言。GRPC action 至少固定 service/method、payload schema、是否 unary、是否依赖 reflection，以及工具镜像里客户端版本。接口、payload 或工具镜像随 serviceVersion 改变时，要和 runtime image、PD、BPT 一起进入版本矩阵。

超时和 retry 只说明 controller 视角的执行结果，不证明远端副作用已回滚。长耗时 action 要有幂等键或业务状态检查：超时后先查远端是否已部分执行，再决定重试、补偿或标为需要人工处理。retry 期间 targetPodSelector 可能重新选到不同 Pod，依赖单一目标的动作要固定目标输入或在脚本里拒绝目标漂移。

`highWatermark` 场景要同时验收只读进入和读写恢复。触发 readonly 后，检查引擎真实只读状态、写请求失败方式和告警给用户的限制；容量恢复或扩容后，`readwrite` action 必须能把引擎带回可写，并用业务写入或引擎状态查询证明，而不是只看 action 退出 0。

真实依据：Etcd `templates/cmpd.yaml` 同时实现 `roleProbe`、`memberJoin`、`memberLeave`、`switchover`、`dataDump`、`dataLoad`；PostgreSQL `templates/cmpd.yaml` 使用 HTTP roleProbe 和 switchover；Redis `templates/cmpd-redis.yaml` 声明 primary/secondary 角色和账号。

`available.withProbe` 只改变 Component Available 的判定入口，不替代角色发现。只要 roleSelector service、backup target、switchover 或 updatePriority 依赖 role，仍然必须实现可靠 roleProbe。`majority`、`strict`、stdout assertion 和 action 退出码要分开验收：stdout 包含期望文本不能替代成功退出，部分 Pod probe 失败时是否仍可 Available 也必须用 status 和 e2e 自证。2 副本这类偶数副本示例要特别说明 majority 判断是否符合业务预期。

lifecycle 排障要把 action 执行成功和业务状态收敛分开。`postProvision` 只适合一次性初始化，scale-out 新成员加入应由 `memberJoin` 或等价流程证明；`preTerminate` 必须能处理 Pod、Service、PVC 已部分删除或重复执行的场景；switchover action 退出 0 后还要用 roleProbe、Instance role status 和 roleSelector Service endpoint 证明角色已收敛。显式指定 action container 或独立 action image 时，要单独验证该执行面能看到脚本、volumeMount、TLS/Secret 和工具二进制，不能默认继承 runtime container 的文件系统。

`memberJoin` 和 `memberLeave` 要同时证明三件事：controller 视角的 action 已完成，业务成员关系已经收敛，重复执行或中途失败后不会把状态写坏。scale-out 期间 kbagent、Service、Pod env 或业务进程可能暂时不可用；脚本必须能识别成员已经存在、成员还未可见、成员加入失败这三类状态，不能把“命令返回 0”或“下一轮重试成功”当作完整合同。scale-in 也不能只等待固定时间：如果引擎因为 quorum、shard awareness、数据迁移、replica placement 或磁盘状态拒绝摘除成员，脚本要能给出可诊断错误、可恢复退出或明确人工处理步骤。

拓扑变化后的 action 不要依赖创建时写入 Pod env 的成员列表。`componentVarRef.podNames/podFQDNs`、旧 `KB_*` 列表或脚本启动时缓存的 endpoint 只能证明 Pod 创建时的视图；scale-out、scale-in、switchover、rebuild 之后，脚本应优先使用 action-time 注入的目标 Pod/FQDN、当前 Service/EndpointSlice、业务 membership 查询或显式传入的 candidate，而不是遍历旧环境变量里的静态列表。

## 7. 变量注入是契约，不是脚本私货

复杂 addon 会大量依赖 `vars`。这些变量把 Cluster、Component、Service、Credential、TLS、hostNetwork、ServiceRef 等运行时信息注入到容器和 action 中。

常用变量来源：

- `clusterVarRef`：cluster name、namespace、uid。
- `componentVarRef`：组件副本数、Pod 名称列表、FQDN 列表、某个 role 的 Pod。
- `credentialVarRef`：system account 的用户名和密码。
- `serviceVarRef`：同一 Cluster 内 KB 管理 Service 的 host、port、nodePort、loadBalancer。
- `serviceRefVarRef`：Cluster 创建时绑定的外部服务或另一个 Cluster 服务。
- `tlsVarRef`：TLS 是否启用。
- `hostNetworkVarRef`：hostNetwork 场景下动态分配端口。

实践规则：

- 每个变量都要在脚本里有明确使用点。不要为了“可能以后有用”注入大量变量。
- 避免 `runtime.containers[*].env`、CMPD `vars` 和 Cluster `componentSpecs[*].env` 定义同名变量。确实需要覆盖时，验收口径以最终 Pod env、action env 和渲染后的 config/script 为准，而不是以任一层 YAML 的书写顺序推断。
- `KB_` 前缀变量视为 KB 注入保留空间，addon 自定义变量不要复用；Pod runtime、lifecycle action、backup/restore Job 的 env、volume、Secret、TLS 挂载是不同执行面，必须分别验收。
- 多组件和多 shard 变量要明确值格式。真实 addon 中常见格式是逗号分隔 Pod 列表或 FQDN 列表，脚本经常用 `cut`、`awk`、`tr` 解析，这些格式必须在 README 或实现说明中固定下来。
- 对外暴露地址要区分 ClusterIP、NodePort、LoadBalancer、hostNetwork。Kafka 模板中已经明确写到当前只支持 NodePort 和 ClusterIP，LoadBalancer 仍是能力缺口。
- `ServiceRefDeclaration` 只声明依赖形状，不提供实际绑定。Cluster 创建时还要用 serviceRef 绑定具体服务。
- 同一个 Cluster 内的 ServiceRef `name` 是依赖身份，必须全局唯一或显式声明共享同一个外部依赖。不要让两个 component 用同名 ServiceRef 指向不同 provider；否则 vars、脚本和用户示例会把两个依赖混成一个合同。

变量和服务合同检查：

- 变量值格式要在 addon 文档中固定，例如逗号分隔、`name:port` 列表、FQDN 列表、JSON 片段，不允许只在脚本里隐式约定。
- `Optional` 变量缺失时脚本要有显式分支；`Required` 变量缺失应让问题尽早暴露。不要假设 Optional 一定渲染为空字符串：不同变量源可能表现为 env 不注入、模板值为空或解析结果为空，脚本应同时处理 unset 和 empty。
- ServiceRef 要同时给出 declaration、Cluster 侧绑定示例和脚本读取方式。
- 外部访问能力必须逐类型验证。ClusterIP、NodePort、LoadBalancer、hostNetwork 的地址来源和 DNS/端口语义不同，不能用一个 advertised address 模板覆盖所有模式。
- 服务和依赖注入的验收顺序是最终 Service、EndpointSlice、Pod label/Ready、注入 env、真实连接。`serviceVarRef` 或 `ServiceRef` 有值只证明注入成功，不证明 DNS、端口、协议、凭据或外部 ServiceDescriptor 兼容。
- `publishNotReadyAddresses` 只能说明 endpoint 可以早发布，不说明业务已经 ready；客户端是否会过早连接、是否有启动握手或重试，要由 addon 自己证明。默认 headless Service、额外 ComponentService、Expose 生成 Service 和 podService 的名称也要做冲突检查，以 apiserver 最终 Service 和 EndpointSlice 为准。

ServiceRef 的开发必须写成 declaration、Cluster 绑定、变量读取三段式。例如 storage 组件依赖一个外部元数据服务：

```yaml
# ComponentDefinition
spec:
  serviceRefDeclarations:
    - name: metadata
      serviceRefDeclarationSpecs:
        - serviceKind: etcd
          serviceVersion: ^3\.
      optional: false
  vars:
    - name: META_ENDPOINT
      valueFrom:
        serviceRefVarRef:
          name: metadata
          endpoint: Required
    - name: META_USER
      valueFrom:
        serviceRefVarRef:
          name: metadata
          username: Optional
    - name: META_PASSWORD
      valueFrom:
        serviceRefVarRef:
          name: metadata
          password: Optional
```

```yaml
# Cluster
spec:
  componentSpecs:
    - name: storage
      replicas: 3
      serviceRefs:
        - name: metadata
          clusterServiceSelector:
            cluster: acmedb-meta
            service:
              component: etcd
              service: headless
              port: client
            credential:
              component: etcd
              name: root
```

如果绑定外部服务，则 Cluster 侧使用 `serviceDescriptor` 指向 ServiceDescriptor 名称。`clusterServiceSelector` 和 `serviceDescriptor` 同时存在时，`clusterServiceSelector` 优先；引用另一个 KB Cluster 时，当前实现会从被引用 Cluster 组装一个内存中的 ServiceDescriptor，但不会按 declaration 强校验 `serviceKind/serviceVersion`，addon 文档和 e2e 必须自己证明兼容性。直接引用 ServiceDescriptor 时边界不同：controller 会要求 ServiceDescriptor `status.phase=Available`，并校验其 `spec.serviceKind/spec.serviceVersion` 至少匹配 declaration 中一条规则，`serviceVersion` 可按正则匹配。`Optional` 的 serviceRef 或变量不是“忽略错误”，脚本要写清楚缺失时是跳过、降级还是失败。

外部依赖要设计变更生命周期，而不只是首次注入。provider 的 endpoint、host、port、credential、ServiceDescriptor status 或拓扑发生变化后，consumer 是否自动 re-render、是否需要 Reconfigure、Restart、重建或明确不支持，都要写成支持矩阵并用最终 Pod/action env、runtime ConfigMap 和真实连接证明。source ServiceDescriptor 更新不等于进程内连接池刷新；optional 依赖从缺失变为存在、required 依赖临时不可用、凭据轮换、provider 扩容缩容和多 Service 入口切换，都是不同路径。不能证明自动刷新时，文档应写“需要重启/重建/人工更新”或“未证明”，不要把旧注入值继续工作误判为兼容。

网络覆盖只在 addon 声明使用时验收，不作为所有 addon 的必选项。声明 `hostAliases`、`dnsPolicy`、`dnsConfig`、`hostNetwork`、`hostPort`、自定义 advertised address 或跨 namespace ServiceRef 时，要按执行面拆开：

| 执行面 | 要看什么 | 常见边界 |
| --- | --- | --- |
| runtime Pod | 最终 Pod spec、DNS 解析、hostAliases、hostNetwork 端口、hostPort 调度事件、Service/EndpointSlice | hostAliases 可能遮蔽 ServiceDescriptor 更新；hostNetwork 通常需要确认 `ClusterFirstWithHostNet` 或等价 DNS；hostPort 是节点级冲突 |
| lifecycle/action worker | action 使用的 Pod、container/image、env、volumeMount、ServiceAccount、DNS 行为 | 不默认继承 runtime Pod 的 hostAliases、工具镜像、TLS trust store 或端口映射 |
| backup/restore Job | Job Pod spec、node、runtimeClass、resources、Repo Secret、ServiceRef env、DNS 和仓库连通性 | target volume、target Pod node、仓库网络、provider Service 都要在 Job 执行面自证 |
| restore postReady/ReadyConfig | postReady action 或 ready checker 的镜像、env、证书、Service 地址 | prepareData 成功不代表 postReady 工具链具备连接数据库和修复元数据的能力 |

Service 暴露验收也要分端口来源。`hostPort`、container port、Service port、nodePort、LoadBalancer ingress、podService 生成的 per-pod Service 和 `serviceVarRef.port/loadBalancer` 输出不是同一个值；advertised address 脚本必须脱敏打印最终 host/port 来源，并分别覆盖 ClusterIP、NodePort、LoadBalancer、hostNetwork 或明确未证明。

多 kind/version declaration 必须附协议矩阵。宽正则可以让 declaration 匹配多个 ServiceDescriptor，但脚本、客户端库和连接参数仍要逐分支验收。例如 `etcd 3.x`、`mysql 8.x`、`postgres 14/15` 或同一 provider 的 `rw/ro/admin` 入口，都应说明使用哪个 endpoint 字段、哪个端口名、是否需要 TLS、账号 key、超时和降级行为。多个外部依赖同时存在时，变量名要包含用途和 provider，例如 `META_ETCD_ENDPOINT`、`AUTH_REDIS_PASSWORD`，不要用 `ENDPOINT`、`USERNAME` 这类泛名；脚本自检只能打印脱敏后的来源、host、port 和变量是否为空，不能把 Secret 值写入运行输出或 ConfigMap。

变量字段语义矩阵：

| 字段 | 输出格式或限制 | 必须同时对齐 |
| --- | --- | --- |
| `VarOption: Required/Optional` | Required 解析失败应暴露问题；Optional 缺失时不保证固定默认值，可能为空或不注入 | 脚本分支和 README 限制 |
| `value` | `$(VAR_NAME)` 只展开前面已经定义的变量，`$$` 用于转义 | 变量顺序，避免引用后定义变量 |
| `expression` | Go template 只作用于已成功解析的非 credential 值；按 vars 定义顺序执行 | 表达式变量名中 `-` 要换成 `_` |
| `credentialVarRef` | 只能用于 Pod/Action env，不用于渲染 config/script 模板 | systemAccounts 名称、Secret、脚本脱敏 |
| `componentVarRef.podNames/podFQDNs` | `name1,name2` 或 `fqdn1,fqdn2` | 脚本解析 delimiter；副本数变化时重新验证 |
| `componentVarRef.podNamesForRole/podFQDNsForRole` | 按 role 过滤后的逗号列表 | `roles`、`roleProbe`、角色稳定性 |
| `serviceVarRef.host` | 普通 Service 输出 service name 或 FQDN；podService 输出按 service 名排序后的逗号列表 | 脚本 delimiter、FQDN/短名选择、pod ordinal 变化 |
| `serviceVarRef.port` | 普通 Service 输出单个端口数字；NodePort 或分配 nodePort 的 LoadBalancer 优先输出 nodePort；podService 输出按 service 名排序后的 `serviceName:port` 逗号列表 | ComponentService 名称、Service 类型、port name、advertised address 脚本 |
| `serviceVarRef.loadBalancer` | 只在 LoadBalancer Service 已有 ingress 时有值；podService 同样按 service 名排序后组合 | LoadBalancer 是否分配、脚本是否允许缺失或延迟 |
| `serviceRefVarRef.endpoint/host/port/podFQDNs` | 来自 Cluster 绑定的 ServiceRef；pod FQDN 和多 service 仍是字符串约定 | declaration、Cluster `serviceRefs`、外部 ServiceDescriptor |
| `multipleClusterObjectOption.strategy=individual` | 为每个匹配 component 生成带后缀的独立变量 | credential 这类不能按值组合的变量 |
| `multipleClusterObjectOption.strategy=combined` | 默认 flatten 形如 `comp1:value,comp2:value`，delimiter 和 keyValueDelimiter 可调 | 多 shard 或多 component 脚本解析 |
| `hostNetworkVarRef` | 读取 hostNetwork 分配后的端口 | CMPD `hostNetwork.containerPorts` 和 container port 名 |
| `tlsVarRef.enabled` | 只告诉 TLS 是否启用，不表示引擎已经兼容 TLS | config、启动、探测、备份、恢复都要分别验证 |

真实依据：Redis `templates/cmpd-redis.yaml` 使用 `serviceVarRef` 获取 advertised service 端口和 LB host；Kafka `templates/cmpd-broker.yaml` 用 podService 和服务变量构造 advertised listener；ClickHouse `templates/cmpd-ch.yaml` 使用 `multipleClusterObjectOption` 组织多 shard FQDN。

变量高级选项也要按最终输出验收。`requireAllComponentObjects` 不能当成 provision 等待机制；它只能作为变量解析边界的一部分。`newVarSuffix` 和 `expression` 的执行顺序、同名变量覆盖、ServiceDescriptor 路径下是否能提供 podFQDNs 这类依赖 KB Pod 列表的字段，都必须通过 Pod/action env 和脚本单测证明，不应在 README 中写成默认成立。

脚本对内置环境变量的依赖要保持最小化。`KB_*` 变量是 KB 执行面提供的输入，不应被 addon 当成无限稳定的公共 API 集合；尤其是成员列表、角色列表、候选 Pod、source target、restore target 这类会随操作变化的值，应优先从当前 action 注入、当前 Service/EndpointSlice、Component/Instance 状态或业务 membership 查询获得。跨组件共享配置也不要靠“某个 Pod env 里刚好有值”实现，应通过 ServiceRef、明确的 vars、Secret/ConfigMap 或实时查询建模，并在 scale、switchover、restore、rebuild 后重新验证。

## 8. 账号和 TLS 要作为一等能力设计

`systemAccounts` 用于数据库管理账号，例如初始化账号、备份账号、探测账号、复制账号。非 init account 通常需要 `lifecycleActions.accountProvision` 去执行创建语句。

实践规则：

- 区分用户账号和系统账号。系统账号由 KB 管理，用于备份、复制、探测和管理动作；用户账号不应混入这些系统流程。
- 对非 init account，提供 `statement.create` 并实现 `accountProvision`。ComponentDefinition controller 会校验：存在非 init account 时，必须有 accountProvision。
- accountProvision 应在能执行授权的目标 Pod 上运行。主从数据库通常选择 primary/leader。
- TLS 的声明只定义挂载路径和文件名，具体引擎可能还需要在启动脚本里转换证书格式、更新配置或兼容 client。

示例：

```yaml
spec:
  systemAccounts:
    - name: admin
      initAccount: true
    - name: kbdataprotection
      statement:
        create: CREATE USER ${KB_ACCOUNT_NAME} PASSWORD '${KB_ACCOUNT_PASSWORD}';
  lifecycleActions:
    accountProvision:
      exec:
        container: acmedb
        command: ["/scripts/create-account.sh"]
      targetPodSelector: Role
      matchingKey: primary
  tls:
    volumeName: tls
    mountPath: /etc/pki/tls
    caFile: ca.pem
    certFile: cert.pem
    keyFile: key.pem
```

真实依据：PostgreSQL `templates/cmpd.yaml` 声明 postgres、kbadmin、kbdataprotection、kbprobe、kbreplicator 等账号，并用 accountProvision 执行 SQL；Redis 和 Kafka 也有各自账号初始化脚本。

账号和 TLS 合同检查：

- system account 名称要和 backup target、probe、replication 脚本一致。
- 账号创建语句不能把密码写入命令输出；使用 shell eval 或模板拼 SQL 时要说明转义边界。
- TLS 打开后，要验证启动、client 连接、roleProbe、backup/restore 是否都支持证书路径。
- TLS 关闭时，脚本不能仍然强依赖证书文件存在。

字段级实践矩阵：

| 字段 | addon 要表达的语义 | 运行验证 |
| --- | --- | --- |
| `systemAccounts[*].initAccount` | 是否由初始化流程自然创建。非 init account 需要 accountProvision | Secret 是否生成；非 init account 是否真的在数据库内存在 |
| `systemAccounts[*].statement.create/update/delete` | 传给 `KB_ACCOUNT_STATEMENT` 的模板语句 | 脚本如何转义用户名、密码；stdout/stderr 是否脱敏 |
| `passwordGenerationPolicy.length/numDigits/numSymbols/symbolCharacters/letterCase` | 默认密码复杂度 | 引擎是否接受这些字符；备份/复制工具是否能正确引用 |
| Cluster `componentSpecs[*].systemAccounts[*].disabled` | 用户禁用某个系统账号 | 禁用后依赖该账号的能力要同时禁用或失败得足够清楚 |
| Cluster `componentSpecs[*].systemAccounts[*].passwordConfig` | 用户覆盖密码生成策略 | 覆盖后 accountProvision 仍可执行 |
| Cluster `componentSpecs[*].systemAccounts[*].secretRef` | 用户提供密码 Secret | key 名默认 `password`；Secret namespace 和 RBAC 要可读 |
| CMPD `tls.volumeName/mountPath/caFile/certFile/keyFile/defaultMode` | 定义 TLS secret 挂载到 Pod 的文件形状 | Pod 内路径、权限、引擎配置文件一致 |
| Cluster `componentSpecs[*].tls` 和 `issuer` | 用户是否启用 TLS，以及证书由 KB 生成还是用户提供 | `UserProvided` 时 Secret key 映射到 `ca/cert/key`；TLS on/off 两套用例都能跑 |

不要把 `tlsVarRef.enabled` 当作兼容性证明。它只是一位开关，真正的兼容性要看启动参数、配置模板、client 连接、roleProbe、accountProvision、Backup/Restore 脚本是否都走同一套证书路径。

凭据要按使用面分开设计和验收：

| 使用面 | 常见来源 | 必须证明 |
| --- | --- | --- |
| runtime 内部账号 | `systemAccounts`、`credentialVarRef`、Secret volume/env | 启动、复制、探测、roleProbe 和 accountProvision 使用的是同一账号语义 |
| proxy/router/ACL 账号 | system account、用户 Secret、proxy 自身配置、路由表同步脚本 | proxy 到后端、客户端到 proxy、管理面修改路由或 ACL 三条路径分别具备最小权限 |
| backup/restore 凭据 | BPT target account、ActionSet/Backup/Restore env、BackupRepo Secret | Backup Job、Restore Job、postReady action 能拿到正确账号和仓库凭据 |
| 公开连接凭据 | user-provided Secret、暴露服务文档、client connection string | README 给出的账号、TLS、Service 地址与实际 Secret 和 Service 一致 |
| 外部依赖凭据 | ServiceDescriptor credential、clusterServiceSelector credential、serviceRefVarRef | consumer env/config 更新、脱敏自检和真实连接都能对上 provider |

Secret rotation 和 TLS rotation 不能从首次启动成功外推。Secret volume 文件可能被 kubelet 更新，但数据库进程、连接池、backup 工具或 action worker 是否热加载，需要 addon 自己证明；不能证明时就写成需要 restart、reconfigure 或重建。issuer 切换、用户提供证书、CA trust store 变化和 key 文件权限要逐项验收：看 Secret key、Pod 内文件内容、`defaultMode`、`securityContext/fsGroup`、引擎配置、client 连接和 backup/restore 工具链。

多容器、init container、lifecycle action、Backup/Restore Job 和 postReady 不是同一个文件系统或权限面。每个会发起连接的执行面都要列出它需要的 Secret/TLS volume、env、serviceAccount、工具二进制和用户权限；不能因为主 runtime container 能连上数据库，就默认 action image、formatter image、backup image 或 restore job 也继承了相同挂载和 trust store。

带 proxy、router、sentinel、keeper 或外部管理组件的 addon，要把“后端数据库账号”和“管理组件账号”分开。proxy 能连上后端不代表它有权修改路由表；备份账号能读数据不代表 restore 后可以重建 ACL；公开连接 Secret 能登录不代表 roleProbe、switchover 或 postReady 使用的是同一权限面。每个账号都要写清楚用途、创建方式、禁用后影响、TLS 要求和失败时的排查入口。

## 9. 配置和参数管理要串成闭环

参数管理不是只写一个 CUE 文件。完整链路是：

1. `ComponentDefinition.spec.configs` 声明配置模板挂载到哪个 volume。
2. chart 渲染出对应 ConfigMap 和模板文件。
3. `ParametersDefinition` 绑定 componentDef、templateName、fileName 和 fileFormatConfig。
4. CUE 或 OpenAPI schema 描述参数合法性。
5. static/dynamic/immutable 分类决定 reload、restart 或禁止变更。
6. `reconfigure` action 让参数变更真正落到进程。

从 legacy `ConfigConstraint` 迁移到 `ParametersDefinition` 时按决策树处理：如果旧对象只用于描述某个 CMPD config file 的 schema，就迁到对应 PD，并让 `templateName/fileName/serviceVersion` 精确绑定该文件；如果旧对象还承载 reloadAction 或脚本输入约定，要把动作迁到 `ComponentFileTemplate.reconfigure` 或 lifecycle `reconfigure`，并保留兼容测试；如果同一文件被多个旧约束覆盖，先合并 schema 和参数分类，再迁移，不能让多个 PD 竞争同一 `fileName`；如果某个版本的配置文件格式、默认值或动态参数不同，就拆成按 `serviceVersion` 匹配的 PD。迁移验收入口不是旧 ConfigConstraint status，而是 PD status、ComponentParameter、runtime ConfigMap 和进程实际配置。

示例：

```yaml
apiVersion: parameters.kubeblocks.io/v1alpha1
kind: ParametersDefinition
metadata:
  name: acmedb-params
spec:
  componentDef: ^acmedb-storage-.*
  templateName: acmedb-config
  fileName: acmedb.conf
  fileFormatConfig:
    format: ini
  parametersSchema:
    cue: |-
      #AcmeDB: {
        "max_connections"?: int & >=1 & <=10000
      }
  staticParameters:
    - data_dir
```

实践规则：

- 新 addon 优先使用 ComponentDefinition file template 上的 `reconfigureAction` 或模板 `reconfigure`，不要依赖 legacy config-manager reloadAction。
- `ParametersDefinition.spec.componentDef` 是前缀/正则匹配 ComponentDefinition 名称，`serviceVersion` 为空表示匹配任意 serviceVersion，不是“自动选 latest”。
- `ParametersDefinition.spec.templateName` 绑定的是 `ComponentDefinition.spec.configs[*].name`，也就是 CMPD 里定义的配置模板标识，不是底层模板 ConfigMap 名。`ComponentFileTemplate.template` 才是引用的 ConfigMap 名；渲染后的运行时 ConfigMap 又是 controller 生成的另一个对象。
- `externalManaged: true` 表示该配置文件交给 parameters 链路管理。它不会改变上一条匹配合同：PD 仍然应该指向 CMPD config template 的 `name`，而不是把 `templateName` 写成模板 ConfigMap 名。
- `fileName` 是模板 ConfigMap data 中的文件 key，也是参数 schema 要作用的配置文件名。它必须能在对应 template ConfigMap 中找到。
- 同一个配置文件不能被多个 ParametersDefinition 覆盖。实现会拒绝重复覆盖同一 fileName。
- `ComponentParameter` 是 controller 内部执行模型和用户 desired 参数合并后的落地对象。调试 reconfigure 时应同时看 `ComponentParameter`、渲染 ConfigMap、InstanceSet/Pod 的配置状态和 OpsRequest 状态。
- 如果某个参数实际需要重启，不要假装支持动态 reload。把 static/dynamic 分类写清楚，让 KB 走正确路径。
- 修改 source template ConfigMap 不等于 runtime ConfigMap 自动重渲染。addon 发布时要验证触发路径：是 Helm upgrade 触发 definition/template 变化、用户发起 Reconfigure，还是需要重建 Component；没有实现证据时，不要把 ConfigMap `resourceVersion` 变化写成自动生效。
- 参数输入要区分三态：缺省表示沿用 schema 或模板默认，空字符串是用户显式写入空值，`value=nil` 表示删除 desired 参数。schema、渲染器和 reconfigure 脚本必须分别覆盖这三种输入。
- 每个 `serviceVersion` 都要验收 PD 渲染结果：`componentDef` 是否匹配该版本、`templateName/fileName` 是否存在、static/dynamic/immutable 分类是否渲染出来、formatter 或 reconfigure action 的工具镜像是否与该版本兼容。
- `ComponentFileTemplate.variables[*].defaultValue` 只能作为模板输入的一部分验收，不能从字段名推断它一定进入最终文件。最小证据是 runtime ConfigMap 中目标 `fileName` 的最终内容、Pod 内挂载文件和进程读取后的配置值三者一致。
- 使用外部 ConfigMap 或用户覆盖配置时，要把 `source.name/key`、模板 ConfigMap data key、PD `fileName`、runtime ConfigMap 文件名、`configHash`、`externalManaged` 和 reconfigure action 串成闭环。内容变了但 `configHash` 或触发信号不变时，不要假设 controller 会自动重渲染。
- 参数 key 规范化规则必须由 addon 固定。API schema 中的 key、文件里的 key、formatter 输出的 key、引擎实际接受的 key 大小写、分隔符和别名可能不同；如果允许 `max-connections`、`max_connections`、`maxConnections` 这类变体，必须说明归一化方向、冲突处理和回显方式。
- OpenAPI schema、CUE schema、formatter 和引擎配置解析要分别验收。formatter 必须幂等：同一 desired 参数重复 reconfigure 后，runtime ConfigMap 和进程配置不应产生无意义漂移。

排障时优先检查这些对象关系：

```text
ComponentDefinition.spec.configs[*].name        # 参数绑定键，PD.templateName 应指向它
ComponentDefinition.spec.configs[*].template    # 模板 ConfigMap 名，提供文件内容
ParametersDefinition.spec.templateName          # CMPD config template name，不是 ConfigMap 名
ParametersDefinition.spec.fileName              # 模板 ConfigMap data key
ComponentParameter.spec.configItemDetails[*]    # 参数 controller 解析后的执行计划
runtime ConfigMap                               # 最终挂载进 Pod 的渲染结果
```

如果 `ComponentParameter.spec.configItemDetails` 为空，先不要改 controller。应先证明 PD 是否匹配到正确 ComponentDefinition、`templateName` 是否等于 CMPD config template 的 `name`、`fileName` 是否存在、PD status 是否 Available、`externalManaged` 是否按预期开启。

不是所有 addon 都天然支持 OpsRequest reconfigure。若引擎配置来自外部系统、多个动态文件、运行时生成文件或业务 API，而当前 PD/ComponentParameter/reconfigure action 无法自然表达，就应把该能力写成“不支持”或“未证明”，并让示例避免暴露 reconfigure 路径。不要只为了让 OpsRequest 创建成功而打开 `externalManaged`、生成空的 ComponentParameter 或把所有参数塞进一个文件；这种半链路会在 restart、scale、upgrade 后把错误 desired 状态继续带到新 Pod。

参数 desired 是会被 controller 继续消费的状态，不是一次性命令。发现参数写错文件、effect scope 错误或 schema 误放行后，恢复步骤要同时清理或修正 OpsRequest、ComponentParameter desired、runtime ConfigMap 和进程真实配置；只回滚 chart 或删一个 ConfigMap 往往不能消除后续 reconcile 的输入。发布修复时要给出旧对象迁移说明：哪些 desired key 要删除，哪些参数要移到另一个 `templateName/fileName`，哪些 Cluster 需要 restart 或重新 reconfigure。

参数字段和脚本合同矩阵：

| 字段 | 实践语义 | 误用表现 |
| --- | --- | --- |
| `staticParameters` | 变更后需要 restart 的参数 | 实际需要重启却标成 dynamic，reload 后进程状态和配置不一致 |
| `dynamicParameters` | 变更后可以由 reconfigure action 生效的参数 | 为空时不要默认“所有参数都可热更新” |
| `immutableParameters` | 用户不应修改的参数 | 只写注释不写字段，变更请求仍进入执行链路 |
| `mergeReloadAndRestart` | 同一次变更既有 reload 又有 restart 时是否只 restart | 设错会导致先 reload 后 restart，或跳过引擎需要的预处理 |
| `reloadStaticParamsBeforeRestart` | 某些引擎要求静态参数先通过 SQL/CLI 预设再重启 | 没有引擎依据时不要打开 |
| `ComponentFileTemplate.reconfigure` | 某个配置模板自己的 reload 动作；排查时先看该文件级 action，再看全局 lifecycle reconfigure | 多配置文件共用脚本时没有按文件名分支 |
| Cluster `configs[*].reconfigureAction` | 用户提供外部配置时的自定义 reconfigure 动作 | 外部配置更新绕过 addon 默认 reload 逻辑 |
| `KB_CONFIG_FILES_CREATED/REMOVED/UPDATED` | 文件级变更输入，不是强类型参数 diff | 脚本假设拿到了参数名和值，导致删参或多文件变更处理错误 |

如果使用类似 MySQL、Redis、PostgreSQL 的 effect-scope 文件来生成 `staticParameters`、`dynamicParameters` 和 `immutableParameters`，生成结果要进入渲染后的 ParametersDefinition，而不是只存在于 chart helper 中。review 时要看 `helm template` 结果。reconfigure 脚本不能假设参数一定合法：即使 schema 已校验，脚本仍要检查参数名、文件名、值格式和当前引擎状态，并保证重复执行不会把配置写坏。

OpsRequest reconfigure 的输入边界要分清楚。`reconfigures[*].parameters[*].key/value` 会进入 `ComponentParameter.spec.desired.assignments` 这类 desired 参数模型；`value=nil` 表示删除该参数。后续 action 侧主要拿到的是文件级变化环境变量和渲染后的配置结果，例如创建、删除、更新了哪些配置文件，而不是一份稳定的结构化参数 diff。只有 legacy reloadAction 路径会把参数 JSON 放进特定请求里，新 addon 不应把这个私有输入形状当成通用合同。通用 reconfigure 脚本应以文件名、渲染结果和引擎状态为准，再用参数名作为可选校验信息。

动态参数还要定义作用域。某些引擎的参数只作用于当前 Pod，某些需要在 primary 执行并复制到全局，某些必须逐 Pod fan out。`ComponentFileTemplate.reconfigure` 或 lifecycle `reconfigure` 的 target 选择要和这个作用域一致：只改一个 Pod、只改 role Pod、还是改组件内所有 Pod，都要用进程配置 dump 或业务查询证明。不能用“一个 Pod reload 成功”代表整个 Component 参数一致。

参数生效排查分三层：第一层是 `ParametersDefinition` status、schema、`fileFormatConfig` 和 `ComponentParameter` desired/observed 是否符合预期；第二层是 runtime ConfigMap 中目标文件的最终内容，删参要确认 key 已从文件删除，多文件参数要同时看每个 `templateName/fileName`；第三层是进程实际状态，例如引擎 config dump、SQL/CLI 查询或重启后启动参数。Reconfigure Ops 成功不承诺 Pod env 会变化，runtime ConfigMap 删除 key 也不等于进程已经忘记旧值；如果参数被误标为 dynamic/static/immutable，addon 要给出回滚、重启或拒绝变更的恢复路径。配置模板不得把 Secret 渲染进普通 ConfigMap，确需使用敏感值时应通过 env、Secret volume 或 action 运行面传递并脱敏记录。

同一个文件同时声明 file-level reconfigure、全局 lifecycle reconfigure、`restartOnFileChange`、`mergeReloadAndRestart` 或 `reloadStaticParamsBeforeRestart` 时，不要从字段名推断固定先后或互斥关系。排查顺序应先看触发该文件的 `ComponentFileTemplate.reconfigure` 或外部 `Cluster configs[*].reconfigureAction` 输出，再看全局 lifecycle reconfigure、OpsRequest/ComponentParameter 状态和 Pod restart 结果。用户覆盖外部 reconfigureAction 后，addon 默认脚本只对自己声明的模板负责；外部模板的输入输出必须由用户 action 或 addon 文档另行证明。

最小 reconfigure 用例：

```yaml
apiVersion: operations.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: acmedb-reconfigure-max-conn
spec:
  clusterName: acmedb-smoke
  type: Reconfiguring
  reconfigures:
    - componentName: storage
      parameters:
        - key: max_connections
          value: "500"
```

验证顺序是 `OpsRequest` phase、`ParametersDefinition` status、`ComponentParameter.spec.configItemDetails`、runtime ConfigMap 内容、action 输出、Pod 是否按 static/dynamic 预期 reload 或 restart。`reconfigure.parameters[*].value=nil` 表示删除参数，脚本必须覆盖这个场景。

真实依据：MySQL `templates/paramsdef-80.yaml`、PostgreSQL `templates/paramsdef.yaml`、ClickHouse `templates/paramsdef-config.yaml`。实现依据见 `pkg/parameters/config_util.go`、`controllers/parameters/componentparameter_controller.go` 和 `controllers/parameters/reconfigure/sync.go`。

## 10. 数据保护要同时提供模板和执行脚本

完整备份恢复能力至少需要：

- `BackupPolicyTemplate`：告诉 KB 某个 ComponentDefinition 支持哪些 backup method、默认 schedule、目标 Pod 选择、账号和 volume mount。
- `ActionSet`：定义备份、恢复各阶段如何执行脚本或 Job。
- `scripts/backup.sh`、`scripts/restore.sh` 等脚本：真实执行引擎备份、推送数据、拉取数据、恢复、状态同步。
- 用户示例：BackupRepo、Backup、Restore 或 Cluster restore。

示例：

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: acmedb-backup-policy-template
spec:
  serviceKind: acmedb
  compDefs:
    - ^acmedb-storage-.*
  target:
    role: secondary
    fallbackRole: primary
    account: kbdataprotection
    useParentSelectedPods: true
  backupMethods:
    - name: physical
      snapshotVolumes: false
      actionSetName: acmedb-physical-br
      targetVolumes:
        volumes:
          - data
        volumeMounts:
          - name: data
            mountPath: /var/lib/acmedb
```

对应的最小 ActionSet 也要同时提供：

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: ActionSet
metadata:
  name: acmedb-physical-br
spec:
  backupType: Full
  backup:
    backupData:
      image: acme/acmedb-tools:1.0.0
      command: ["/scripts/backup.sh"]
      syncProgress:
        enabled: true
        intervalSeconds: 30
  restore:
    prepareData:
      image: acme/acmedb-tools:1.0.0
      command: ["/scripts/restore.sh"]
      runOnTargetPodNode: true
    postReady:
      - exec:
          container: acmedb
          command: ["/scripts/post-restore.sh"]
```

实践规则：

- 只写 `ActionSet` 不算支持备份；必须有 `BackupPolicyTemplate` 绑定到正确 `compDefs`，让 Cluster 自动生成 BackupPolicy。
- 非 snapshot backup method 必须有 `actionSetName`。snapshotVolumes=true 的方法可以使用 CSI snapshot，不一定需要 ActionSet。
- target role 和 fallbackRole 应和 ComponentDefinition 的 `roles`、`roleProbe` 一致。单副本时 controller 会调整 selector，不能只靠手写 label。
- 脚本要遵守 dataprotection 注入的 `DP_*`、`DATASAFED_*`、状态文件和进度同步约定。失败状态不能只打印错误后继续执行。
- PITR、增量备份、逻辑备份、volume snapshot 是不同 method，不要用一个 method 名掩盖多个语义。
- Restore 到新集群、重建实例、PITR、跨拓扑恢复的限制必须写明。

数据保护合同检查：

- BPT 的 `compDefs` 必须命中 resolved ComponentDefinition；不能被多个 BPT 同时竞争同一个 ComponentDefinition。
- 每个 backup method 的语义要单一。physical、logical、incremental、archive log、snapshot、PITR 不应混在一个 method 名下。
- target role 要和 `roles/roleProbe` 对齐；多副本和单副本都要验证选 Pod 行为。
- ActionSet 的 backup、restore、prepareData、postReady 阶段要说明输入环境变量、数据目录、状态文件、错误退出语义。
- Restore 需要单独验证，不得用 Backup 成功替代。恢复后的数据、账号、TLS、配置、service 都要检查。
- rebuild instance 是 Restore 能力的消费者；如果 restore 脚本只支持新集群恢复，就不能声称 rebuild 已支持。

字段语义矩阵：

| 字段 | addon 要表达的语义 | 必须同时验证 |
| --- | --- | --- |
| BPT `serviceKind`、`compDefs` | 哪类组件自动生成 BackupPolicy | resolved ComponentDefinition 只匹配一份 BPT |
| BPT `target.role/fallbackRole/account/strategy` | 选哪个 Pod、用哪个系统账号连接数据库 | `roles/roleProbe`、systemAccounts、单副本和多副本选 Pod 行为 |
| BPT `schedules/backoffLimit/retentionPolicy` | 默认备份计划、失败重试和保留策略 | 用户示例里能看到生成的 BackupPolicy |
| method `name` | 用户和 Backup CR 调用的稳定方法名 | method 名语义单一，不把 physical/logical/incremental/PITR 混用 |
| method `compatibleMethod` | 增量或差异备份依赖的 full method | parent backup 链和 restore 顺序 |
| method `snapshotVolumes` | 是否走 CSI snapshot | CMPD `volumes[*].needSnapshot`、StorageClass snapshot 支持 |
| method `actionSetName` | 非 snapshot method 绑定执行定义 | ActionSet status Available，backup/restore 阶段都存在 |
| method `targetVolumes.volumes/volumeMounts` | backup workload 挂载目标 Pod 的哪些 volume | CMPD volume 名、容器 mountPath、restore PVC |
| method `env/runtimeSettings/target` | method 级覆盖工具镜像、资源和选 Pod 策略 | versionMapping、资源请求、target 覆盖是否和全局 target 冲突 |
| ActionSet `backupType` | Full、Incremental、Differential、Continuous、Selective 之一 | Backup/Restore 示例和 method 名一致 |
| ActionSet `parametersSchema/withParameters` | Backup/Restore 参数名校验 | 脚本仍做值校验，不能只依赖 schema |
| `backup.preBackup/backupData/postBackup/preDelete` | 备份前、备份主体、备份后和删除前动作 | 失败退出语义、重试幂等性、产物是否写到 datasafed 路径 |
| `backupData.syncProgress` | 是否同步进度到 Backup status | 脚本输出、Backup status totalSize/duration/phase |
| `restore.prepareData/postReady` | 数据准备到目标 PVC，目标组件 ready 后修复集群内状态 | 新集群恢复、rebuild、账号、TLS、拓扑兼容性 |
| Restore `backup.sourceTargetName/restoreTime` | 从哪个 source target 和时间点恢复 | PITR、增量链路、source target 名称 |
| Restore `prepareDataConfig/readyConfig/env/backoffLimit/parameters` | restore job 的 PVC、调度、env 合并、重试和参数 | 最终 env 合并结果、参数默认来源和脚本行为 |

每个 backup method 都要附一张 restore 兼容矩阵，至少覆盖：新集群恢复、rebuild instance、PITR 或增量链、TLS on/off、系统账号、跨 serviceVersion、跨 topology、sharding 数变化。无法验证的格子写“不支持”或“未证明”，不要通过脚本假设补齐。

每个 topology 也要单独证明 BackupPolicyTemplate 是否生效。`standalone`、`replication`、`mgr`、`proxy`、`sharding`、`keeper` 这类拓扑可能使用不同 ComponentDefinition 名称、roleProbe、target role、账号和数据卷；BPT `compDefs` 命中一个普通组件，不代表它覆盖了 topology 专属组件。最小证据是该 topology 的 Cluster 生成了预期 BackupPolicy，Backup target selector 选中正确 Pod，ActionSet method 与该 topology 的账号、TLS、volume 和 serviceVersion 兼容。

从已有 `Backup` 恢复新 `Cluster` 要提供独立用户示例。这个示例不能只给 `Restore` CR，因为 `dataprotection Restore` 本身负责准备 PVC 数据和 postReady 动作，不负责声明目标 Cluster 的组件、拓扑、Service 和参数。addon 应按目标用户场景提供下面两类入口之一。

显式声明目标 Cluster 时，在新 Cluster 上写 `spec.restore`，并同时写清楚目标 Cluster 的 topology、component、volume 和 serviceVersion：

```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: acmedb-restore
  namespace: demo
spec:
  clusterDef: acmedb
  topology: replication
  terminationPolicy: Delete
  restore:
    source:
      apiGroup: dataprotection.kubeblocks.io
      kind: Backup
      name: acmedb-backup-20260605
      namespace: demo
    pitr: "2026-06-05T08:00:00Z"
    parameters:
      dataprotection.kubeblocks.io/volume-restore-policy: Serial
      dataprotection.kubeblocks.io/source-target-name: acmedb-storage-0
      restore-mode: full
  componentSpecs:
    - name: storage
      serviceVersion: "1.0.0"
      replicas: 2
      volumeClaimTemplates:
        - name: data
          spec:
            resources:
              requests:
                storage: 20Gi
```

希望从 `Backup` 记录的 cluster snapshot 派生目标 Cluster 时，用 Restore OpsRequest。该入口会先读取源 Backup，要求源 Backup 已完成或是合法 continuous backup，并要求 Backup 带有 cluster snapshot；controller 会把目标 Cluster 名称改成 `spec.clusterName`，把恢复 intent 写入目标 Cluster，再由 Cluster/Component/PVC/Restore 链路完成数据准备和 postReady：

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
    parameters:
      - name: restore-mode
        value: full
```

Restore OpsRequest 派生出的目标 Cluster 不是源 Cluster 的逐字段复刻。LoadBalancer Service、NodePort、selector、offline instances、TLS issuer、sharding account SecretRef、调度 selector 等字段可能被重置、清理或重写；addon 的连接文档和验收脚本必须以恢复后 live Cluster、Service、Secret 和 Pod spec 为准，不从源 Cluster YAML 推断最终值。

`Cluster.spec.restore.parameters` 中的 `dataprotection.kubeblocks.io/volume-restore-policy`、`dataprotection.kubeblocks.io/source-target-name`、`dataprotection.kubeblocks.io/source-target-pod-name`、`dataprotection.kubeblocks.io/restore-env` 和 `dataprotection.kubeblocks.io/defer-post-ready-until-cluster-running` 是 KB restore 链路消费的控制参数；addon 自定义参数可以共存，但不要为这些语义另造字段名。使用 Restore OpsRequest 时，`volumeRestorePolicy`、`env` 和 `deferPostReadyUntilClusterRunning` 会被 handler 转成对应参数。

PVC 恢复只覆盖 Backup method `targetVolumes` 声明、且目标 Cluster volumeClaimTemplates 中存在的卷。target volume 名称对不上时，目标 PVC 可能只走普通 provision，不会恢复数据。多 target 或 `PodSelectionStrategyAll` 场景要在示例中说明 `source-target-name`、`source-target-pod-name` 或等价映射如何选择；instanceTemplate ordinal 无法可靠推断时，不要让用户靠名字猜。

addon README 中的 restore 示例要明确这些输入从哪里来：源 Backup 的 namespace/name、Backup phase、backup method、target/targets、artifact 或 snapshot、PITR timeRange、目标 Cluster 的 topology/component/volume 映射、是否允许跨 namespace、是否保留或重建账号/TLS/Service。恢复后至少检查目标 Cluster Running、PVC restore condition、生成的 `Restore` 对象、restore Job、postReady、数据读写、公开连接 Secret 和 TLS 连接；只看到 OpsRequest 或 Restore phase 成功，不足以证明业务恢复完成。

数据保护的扩展能力只在 addon 声明使用时设计和验收。声明后要把字段、Secret、工具镜像、artifact 和恢复消费者连起来：

| 扩展能力 | 需要设计的合同 | 验收证据 |
| --- | --- | --- |
| 加密备份 | passphrase Secret 的创建、引用、轮换、保留、删除和旧备份兼容策略 | Backup/Restore Job env 或 mount、旧备份用旧密钥可恢复、新密钥只影响后续备份 |
| method `formatVersion` 或等价格式版本 | backup image、restore image、ActionSet、脚本参数和 artifact layout 的版本矩阵 | 每个格式版本至少一次 backup+restore；跨版本不支持时明确拒绝 |
| snapshot method | VolumeSnapshot/PVC restore 与 repository artifact 分流 | snapshot handle/PVC、StorageClass/VolumeSnapshotClass、是否仍需要 datasafed/repo artifact |
| repository artifact | `path`、`kopiaRepoPath`、parent/base metadata、target/targets status 的读取规则 | 仓库目录或对象存储产物、Backup status artifact 字段、restore 脚本输入一致 |
| `postBackup` 后处理 | 失败后 artifact 是删除、标记不可用、允许复用还是需要人工清理 | 完整性标记、幂等重跑策略、retention/preDelete 对半成品的行为 |
| `backupType=Selective` | 选择参数、默认值、允许对象、排除对象和 restore 参数闭环 | Backup 参数、artifact 清单、Restore 参数、恢复后只包含预期数据 |
| `RestoreKubeResources` 或恢复 K8s 资源 | 允许覆盖的资源清单、禁止覆盖的资源、和 parameters/controller 重渲染的关系 | 目标 ConfigMap/Secret/ServiceAccount/RBAC diff、后续 ComponentParameter 是否覆盖恢复结果 |
| `deferPostReadyUntilClusterRunning` | postReady 是依赖 Cluster Running，还是用于推动 Cluster Running | 没有等待环；ReadyConfig、postReady、Component/Cluster status 的先后关系可解释 |

备份恢复 action 的 env 覆盖也按最终 workload 验收。BackupMethodTPL env、Backup env、Restore env、ActionSet env 或 parametersSchema 默认值存在同名或缺省时，guide 不替具体优先级背书；脚本应在脱敏后记录关键最终值来源，或者在测试中显式打印最终 env，便于区分是 method、用户 CR、ActionSet 还是脚本内部默认。声明 `syncProgress.enabled=true` 的 method 还应验收 Backup status 中的进度、大小或等价观测结果；否则只能说脚本上传成功，不能说进度同步能力已证明。

加密、压缩和格式版本必须做 payload 级验证。API 字段、Secret 或 env 存在，只能证明执行面收到了配置；它不证明备份数据实际经过了支持该能力的工具链。使用 native client、数据库自带 backup 命令、datasafed、kopia、对象存储 SDK 或自写脚本时，要分别说明数据从哪里写出、是否经过加密/压缩、artifact manifest 在哪里、restore 用哪个工具读取，以及错误密钥、错误 formatVersion、缺少 keystore 或缺少模板文件时如何失败。

Backup `Completed` 不是 restore contract。每个 method 都应产出或能推导一份 artifact manifest：仓库路径、target 名称、数据文件、元数据文件、formatVersion、base/parent、加密信息、工具版本、必要模板或 keystore。restore preflight 要先检查这份 manifest 和目标 Cluster 条件，再开始覆盖 PVC；如果缺 artifact、路径不一致、工具镜像不兼容或目标 topology 不匹配，应清楚失败，而不是在 postReady 阶段才暴露成业务不可用。

空数据集、无增量或无变化也是合法场景，不能让备份脚本无限等待。addon 要定义空 full backup、空 incremental backup、没有 binlog/WAL/segment、没有 topic/table/slot 变化时的退出码、artifact 形状和 restore 结果。若该 method 不支持空数据或空增量，应在 method 文档和脚本前置检查里明确拒绝。

BPT method env 的 `versionMapping` 使用 Cluster component spec 上的 `serviceVersion` 做匹配，通常是 Cluster normalize 后的 component serviceVersion，而不是直接读取某个 ComponentDefinition 或 ComponentVersion 对象字段。匹配语义是先精确匹配，再按前缀或正则匹配。开发时要用显式 serviceVersion 的 Cluster 样例验证每个 method env 的最终值；不要因为 BPT `compDefs` 命中了某个 CMPD，就推断 `versionMapping` 一定会按该 CMPD 的原始 `spec.serviceVersion` 选择。

BackupRepo 是独立依赖，不是 BackupPolicy 的附属字段。排查备份失败、恢复失败或仓库凭据轮换时，必须同时看 BackupRepo/Secret、BackupPolicy、BackupSchedule、Backup/Restore Job 的 env 和 mount。Repo 临时不可用后，旧 Failed Backup 与新重试 Backup 要分开判断；Repo Secret 轮换后，已经存在的 Job 不会因此自动换凭据，后续 Backup/Restore 是否消费新 Secret 要以最终 Job spec 为准。删除 Backup 或清理 retention 时，也要区分 Kubernetes CR 链和仓库 artifact 链：parent/base metadata、Backup status 中的依赖关系、仓库目录或对象存储中的真实产物，都要能闭合。

Continuous backup 和 PITR 的状态语义要单独写清楚。Continuous method 长期 `Running` 可能是正常归档状态，不等于一次性 Full Backup 未完成；addon 要验收归档产物持续生成、timeRange 可解释、`restoreTime` 落在可恢复窗口内，并实际演练 PITR。只给 `restoreTime` 或只看 Backup phase 不能证明 PITR 可用。

跨 topology、sharding 或 rebuild restore 必须有 target 映射表：

| 场景 | 映射要求 | 必须声明的边界 |
| --- | --- | --- |
| 同 topology 新集群恢复 | source target 到目标 component/volume 一一对应 | 账号、TLS、serviceVersion 和参数兼容 |
| rebuild instance | source backup 到目标 ordinal/PVC/实例身份对应 | restore 脚本是否支持只修复单实例 |
| 跨 topology 恢复 | source component 到目标 component 的显式映射 | 缺失 component、额外 component、proxy/keeper 是否重建 |
| sharding restore | source shard/target 到目标 shard 的映射 | 空 shard、skip shard、目标 shard 多于或少于源 shard 时是否支持 |

`skip`、空 shard 或 source target 缺失只能说明某个阶段被跳过，不等于目标 Cluster 数据完整。prepareData 成功但 postReady 失败时，PVC 和部分数据可能已经存在；重跑前先判断脚本幂等、仓库 artifact、目标 PVC 和业务元数据，再决定清理、继续或标为人工恢复。

自动备份和 Restore 的收敛要单独排查。Backup target 误选时，从 BPT/BackupPolicy 的 target、生成 selector、Pod role label 和单副本 fallback 逐层看；Backup `Succeeded` 后还要证明 artifact 清单、repository 路径或 Backup status 中的产物信息可发现。BackupSchedule 不用泛化状态名代替真实字段，应检查 schedule name 是否唯一、`schedules[*].enabled`、`status.schedules`、`repoName`、关联 CronJob 以及实际 Backup 创建和失败事件；默认 repo 变更后，旧 BackupPolicy/BackupSchedule 是否刷新必须以生成对象为准。Cluster restore 要明确 source namespace、Backup 名称和 UID，避免同名源对象歧义；PITR 不能只看 `restoreTime`，还要看 continuous method、参数传递和 postReady/ReadyConfig 校验。Restore Job 完成后目标 Cluster 仍未 Running 时，继续查 prepareData/PVC mount、Component/InstanceSet status、ReadyConfig、postReady 和恢复后公开凭据来源，直到连接文档、systemAccounts 和实际 Secret 能对上。

真实依据：MySQL `backuppolicytemplate.yaml` 同时提供 xtrabackup、incremental、volume-snapshot、archive-binlog、mydumper；PostgreSQL 支持 pg_basebackup、pgdump、wal-g 和 PITR；Kafka 的 topics backup 说明不是所有引擎备份都等价于块设备备份；Etcd target leader 并提供 dataDump/dataLoad。

## 11. Day-2 运维以内置操作为主

内置 `OpsRequest` 覆盖常见管理动作：restart、stop/start、horizontal scaling、vertical scaling、volume expansion、reconfigure、expose、switchover、upgrade、rebuild、backup、restore。addon 作者要确保 ComponentDefinition、参数、备份和生命周期动作提供这些操作所需的底层能力。

内置操作失败时，先查 addon 是否提供了对应底层合同。例如 switchover 依赖可靠 roleProbe 和 switchover action，rebuild 依赖 restore 能力，reconfigure 依赖参数和配置链路，volume expansion 依赖 PVC 和存储类，delete 依赖生命周期动作能够收敛。

运维字段矩阵：

| Ops | 关键字段 | 依赖的 addon 合同 | 常见误用 |
| --- | --- | --- | --- |
| restart/stop/start | `restart/stop/start[*].componentName` | 启动脚本幂等、readiness 准确、一次性初始化有保护 | restart 后重复初始化破坏数据 |
| horizontal scaling | `horizontalScaling[*].scaleOut/scaleIn`、`shards`、`offlineInstancesToOnline/onlineInstancesToOffline` | `memberJoin/memberLeave`，sharding 时还要 `ShardingDefinition` | 把 offline/online 当普通 replicas；`shards` 和 scaleOut/scaleIn 混用 |
| scale out from backup | `scaleOut.fromBackup` | Backup/Restore method 支持作为新副本恢复 | 只验证新集群 Restore，未验证 standby/replica 场景 |
| vertical scaling | `verticalScaling[*].resources/instances` | 引擎能感知资源变化，或声明需要 restart | 资源改了但进程内部参数没调整 |
| volume expansion | `volumeExpansion[*].volumeClaimTemplates[*].name` | Cluster VCT 名、CMPD volumeMount、文件系统扩容 | VCT 名和 data volume 名不一致 |
| reconfigure | `reconfigures[*].parameters[*].key/value` | PD、config template、参数分类、reconfigure action | `value=nil` 删除参数未处理 |
| expose | `expose.services[*].name/serviceType/ports/roleSelector/podSelector` | ComponentService、roleProbe、外部访问脚本 | LoadBalancer/NodePort/ClusterIP 地址语义混用 |
| switchover | `switchover[*].instanceName/candidateName` | roleProbe、roleSelector service、switchover action | candidate 和脚本预期不一致；role 为空时没有分支 |
| upgrade | `upgrade.components[*].componentDefinitionName/serviceVersion` | ComponentVersion、镜像 key、rollout 策略、数据兼容 | 只升级 runtime image，action image/PD/BPT 没验证 |
| backup/restore | `backup`、`restore` | BPT、ActionSet、BackupRepo、restore 脚本 | 只有 Backup 成功，没有 Restore 验证 |
| rebuild instance | `rebuildFrom` 或 backup restore 路径 | Restore 兼容矩阵、目标 PVC、postReady | restore 脚本只支持新集群，却声称支持 rebuild |

运维合同检查：

- restart：脚本和进程管理要能接受重启；不要把一次性初始化逻辑写成每次启动都会破坏状态。
- horizontal scale：scale out 需要 memberJoin 或等价加入逻辑；scale in 需要 memberLeave 或明确不需要。
- vertical scale：资源变更后进程是否读取 cgroup/配置变化，是否需要 restart。
- volume expansion：PVC 名、volumeClaimTemplates、mountPath、文件系统扩容路径要闭合。
- reconfigure：参数分类、渲染结果、reload/restart 动作和状态必须一致。
- switchover：roleProbe、roleSelector、switchover action 三者必须同时成立；指定 candidate 时验证 candidate 最终达到目标 role，未指定 candidate 时验证 action 自身确实完成切换并由后续 roleProbe 反映。
- rebuild：必须能从 Backup/Restore 或 dataLoad 路径重建目标实例。

几个 Day-2 边界要提前写成支持矩阵。`Start` 只恢复停止的 workload，不等于重新执行 `postProvision`；一次性初始化必须有幂等保护，Start/Restart 后只能依赖启动脚本、memberJoin 或显式恢复动作。Vertical scaling 不只看 OpsRequest 成功，要给出调度、Pod 重建或 in-place 变化、PVC attach/detach、容器内 cgroup 资源和引擎内部资源感知的证据。`RebuildInstance` 的 ordinal 是 KB 实例身份，不一定等于数据库业务成员 ID；脚本要从引擎元数据确认目标成员，而不是只用 Pod 名或 ordinal 拼业务命令。

`scaleOut.fromBackup` 是 Restore 能力的独立消费者，不能用“新建 Cluster restore 成功”替代。它要验收 source Backup namespace/name、`restoreTime` 或 PITR 窗口、`restoreEnv`、目标新副本的 memberJoin、角色收敛和数据一致性；从备份恢复出的新副本是 standby、secondary 还是可投票成员，要由脚本和 roleProbe 共同证明。多 target Backup 场景必须声明 `sourceBackupTargetName` 到目标 component、volume、ordinal 或 shard 的映射；未映射的 target 不能默认为可安全跳过。

`RebuildInstance` 至少区分 in-place 和 replace 两条路径。in-place 路径要证明旧进程停机、文件锁、PVC 清理/复用、restore 覆盖和 postReady 幂等；replace 路径要证明新旧 Pod/Service endpoint 过渡、memberLeave/memberJoin、PVC 绑定、存储拓扑和客户端切流。`targetNodeName` 只能表达调度偏好或目标节点，不证明旧 PV 一定可 attach；最终仍要看 PVC/PV、Pod events 和存储类约束。

sharding Ops 字段要在示例里标注名字语义。例如 `shards` 指 shard spec 数量，sharding spec name 来自 `spec.shardings[*].name`，生成的 Component 对象名可能带 cluster 和 shard 后缀，`componentName`、`componentObjectName`、`instanceName` 不能混用。排查时同时打印 Cluster spec、Component 对象名、InstanceSet/Pod label 和 Service selector，避免把模板名当成可操作对象名。

交错 Ops 要么证明支持，要么写明禁止组合：

| 组合 | 支持前提 | 不支持时的文档边界 |
| --- | --- | --- |
| Upgrade + Switchover | roleProbe、updateStrategy、candidate 选择和 rollout 状态能同时收敛 | 升级期间禁止切主，或必须等 Component/InstanceSet 完成 |
| Reconfigure + Restart/VerticalScaling | 参数 desired/observed、runtime ConfigMap 和进程状态能重试到一致 | 配置变更完成前禁止触发资源变更 |
| Scale-in + Backup/Restore | target Pod、memberLeave、Backup target selector 不会选到即将删除成员 | 缩容期间禁止备份或恢复到该组件 |
| Stop + Delete/Start | 停止状态下 preTerminate、PVC retention 和启动恢复路径都有明确语义 | Stop 后删除不保证执行 preTerminate；Start 不重放 postProvision |

Ops 状态机排查要允许“状态”和“事实”短暂不一致。并发 Ops、precondition、force、`enqueueOnForce`、cancel、timeout 和 TTL 先看 OpsRequest `phase/message/conditions` 与目标 Component 状态，再看是否已有 action 后置校验失败、status 更新失败或 partial success；这些字段只说明队列和生命周期边界，不是业务最终事实。`progress` 不等于 Pod Ready，`lastConfiguration` 是该次 Ops 的前态证据，不是全局真实配置；TTL 清理 OpsRequest 后，要从 live Cluster/Component/InstanceSet/PVC/Job、events 和 GitOps 记录恢复证据。事实已经改变但 Ops 失败时，不要直接重试，先用最终对象和业务自检判断是否幂等安全。volume expansion 至少分三层验收：PVC capacity 变大、容器内文件系统可见、引擎内部容量或只读状态已更新；OpsRequest 成功只覆盖其中一部分证据。Stop/Start 这类操作如果输入列表为空，也要以 server 端最终 Ops spec、events 和目标 Component 变化判断默认作用范围，不从空数组字面推断。

内置 Ops 的“支持”要按消费者分开声明。新建 Cluster restore 成功，不代表 `scaleOut.fromBackup` 和 `RebuildInstance` 都可用；普通 `scaleOut` 成功，不代表从备份恢复的新副本能加入投票成员；`Reconfigure` 成功，不代表所有 Pod 的动态参数一致。每个 Ops 至少要说明入口字段、目标对象、执行面、后置业务自检和不支持路径。当前 API 无法自然表达的需求，比如需要复杂 rebalance、跨 topology 自动迁移或需要人工确认的数据迁移，应写成限制或 gap，不要藏进一个脚本后宣称支持。

## 12. 常见操作排障要从入口 CR 逐层向下查

addon 的运行问题通常不是单点问题。一次创建、删除或 day-2 操作会跨过 Cluster、Component、InstanceSet/Pod、Parameters、Backup/Restore、OpsRequest 和脚本动作。排障时应先确认入口对象、resolved definition、底层对象和脚本动作是否闭环，再判断是 chart 合同、脚本行为、用户操作还是 API/实现缺口。

排障时不要从最后一个错误倒推全部语义。更稳定的顺序是：先看入口 CR 的 `generation/observedGeneration`、`phase/message/conditions`，确认 controller 是否已经处理了当前 spec；再看被引用的 definition 或 policy 是否 `Available`；然后看生成对象和最终 workload；最后看脚本输出、事件和业务自检。对象存在不等于可引用，spec 写了字段也不等于最终 Pod、Job、Service、PVC 已按预期生成。

删除、卸载、scale-in 和 Stop 后删除要分流排查：

| 路径 | 先看 | 数据和外部资源边界 |
| --- | --- | --- |
| Cluster 删除 | `terminationPolicy`、Cluster finalizer、Component deletionTimestamp、orders | PVC retention、PV reclaim policy、BackupRepo artifact 和外部 ServiceRef 都是独立边界；外部依赖默认不归当前 Cluster 清理 |
| Component 删除 | Component finalizer、InstanceSet/Pod/PVC ownerRef、preTerminate action | 只清当前 component 管理的对象；共享 Service、RBAC 或外部依赖清理由 chart 和文档自证 |
| scale-in | OpsRequest、目标实例、`memberLeave`、PVC retention | 业务成员摘除和 PVC 保留是两件事；保留 PVC 不等于成员仍在复制组，删除 PVC 也不等于业务元数据已清理 |
| Stop 后删除 | Stop Ops 状态、Pod 是否仍存在、Component phase | Pod 已停止或删除时不能假设 preTerminate 可执行；需要跳过时必须保留现场并写明风险 |
| Helm uninstall/feature disable | release manifest、残留 cluster-scoped definition、Role/RoleBinding/SA | definition 删除不自动证明 RBAC、BackupPolicy、Schedule、Repo artifact 已清理；保留策略要写入发布说明 |

如果删除卡在 lifecycle 下线动作，而目标 Pod、InstanceSet 或其它执行面已经不存在，先保留 Cluster、Component、InstanceSet/Pod、Event 和 action 状态证据，定位执行面为何提前消失。只有确认业务清理已经不需要、已经由外部补偿完成，或当前恢复目标明确要求释放资源时，才能按明确风险的运维恢复流程绕过该动作；绕过后的结果不能作为 addon 删除流程验收通过的证据。

按问题域组织排查路径时，可以使用下面的固定入口：

| 问题域 | 第一批对象和字段 | 下一跳证据 | 边界 |
| --- | --- | --- | --- |
| 资源引用和 status | 入口对象 namespace/source、`ComponentDefinition`、`ClusterDefinition`、`ComponentVersion`、`ParametersDefinition`、`BackupPolicyTemplate`、`ActionSet` 的 `generation/observedGeneration/phase/message/conditions` | 引用它们的 Cluster、Component、BackupPolicy、ComponentParameter、OpsRequest、Job 和事件 | 不要只看 CR 是否存在；同名删除重建、phase/message 不一致或 namespace 源歧义时，以引用方最终生成物为准 |
| 发布迁移和 GitOps | 新旧渲染结果、server-side dry-run/diff、live Cluster spec、不可变或创建期字段 | 生成的 Component、InstanceSet/PVC、BackupPolicy、BackupSchedule、ComponentParameter | `clusterDef/topology/restore`、组件身份和 sharding 名称变更按迁移处理；list merge/retainKeys 以 server 最终对象为准 |
| 名字和身份 | Cluster UID、spec name、generated Component name、status key、Pod/PVC/Service 名、selector label | 引擎成员 ID、备份 target、sharding component、持久数据里的身份记录 | 不把 Pod 名、generated component name、clusterUID 或 ordinal 当作不可重算业务身份 |
| 终止和数据保留 | Cluster `terminationPolicy`、Component/Cluster deletionTimestamp、finalizers、conditions | InstanceSet、Pod、PVC、Backup/BackupRepo、preTerminate action | `Halt/Delete/WipeOut/DoNotTerminate` 与 PVC retention、备份仓库清理、脚本失败是不同层；删除失败不能直接归因到 lifecycle |
| 调度、PVC 和存储 | Cluster/Component 的 scheduling、runtimeClass、affinity/tolerations、volumeClaimTemplates、PVC retention | 最终 Pod spec、PVC name/ownerRef/labels、StorageClass、Pod/PVC events | 调度和 runtimeClass 对 backup/restore/action worker 是否继承，必须看实际 Job/Pod；PVC adoption 以名称、ownerRef、label 和事件为准 |
| Service、Expose 和网络覆盖 | CMPD `services`、ClusterService/Expose spec、ServiceRef、serviceVarRef、hostAliases、dnsPolicy/dnsConfig、hostNetwork/hostPort | 最终 Service type/ports/selector/annotations、EndpointSlice、Pod label、Pod/Job DNS、注入 env、连接测试 | 不覆盖 KB 系统 selector label；ServiceRef name 同 Cluster 内唯一或显式共享；publishNotReadyAddresses、Service 命名冲突、hostPort 调度和 advertised address 都按最终对象验收 |
| ServiceAccount 和 RBAC | Cluster/Component `serviceAccountName`、Action/Restore 运行位置 | Pod/Job 使用的 SA、Role/RoleBinding/rules、action 或 restore 失败事件 | 组件 Pod、lifecycle action、backup/restore Job 可能不是同一个权限面；跨 namespace Secret 或外部资源访问必须单独自证 |
| Pod 更新和 InstanceTemplate | OpsRequest、Component `podUpdatePolicy/podUpgradePolicy/instanceUpdateStrategy`、InstanceTemplate | InstanceSet `current/updatedReplicas`、revision、partition/canary/rolling budget、Instance/Pod/PVC 身份、OnDelete 等待状态 | `OnDelete` 表示等待用户删除 Pod，不是 controller 卡死；模板名、默认 instances、flat ordinal、PVC 名和系统 label 不要用连续数字或手工改 label 替代实际对象 |
| Action、lifecycle 和 probe | lifecycle action 的 preCondition、targetPodSelector、handler、container/image、timeout/retry | action worker/Pod、volumeMount、stdout/stderr、exit code、role label、Instance role status、Component Available 条件 | 多 handler、默认容器、HTTP/GRPC 能力、retry 成功语义都不能靠字段名猜；脚本幂等、probe 阈值、roleProbe 输出和独立 action image 工具链要分别证明 |
| variables、env 和外部依赖 | CMPD vars、Cluster env、ServiceDescriptor/ServiceRef、credential/tls/resource var、`KB_` 前缀 | 最终 Pod/action/backup/restore env、渲染后 config/script、脱敏后的脚本自检、真实连接 | 同名 env 覆盖、Optional 缺失、变量展开顺序、列表 delimiter/排序/转义和外部 ServiceDescriptor 值一致性，都以最终注入结果为准 |
| 参数和 reconfigure | PD `status`、`templateName/fileName/fileFormatConfig/serviceVersion`、OpsRequest reconfigure、外部 ConfigMap key、`configHash` | ComponentParameter、runtime ConfigMap、file-level action、全局 reconfigure、Pod restart、进程配置 dump | 新 addon 优先 file-level reconfigure；defaultValue、删参、多文件冲突、Secret 泄露、formatter 幂等和 key 规范化要用文件与进程双重证据定位 |
| 备份、自动备份和 artifact | BackupPolicyTemplate、生成的 BackupPolicy、BackupSchedule `schedules[*].name/enabled/status.schedules/repoName`、Backup | method/actionSet、backupType、target Pod、CronJob/Backup 结果、snapshot 或 repo artifact、passphrase Secret、formatVersion | 自动备份是否触发看 BackupSchedule、CronJob 和 Backup 结果；加密、snapshot、Selective、postBackup 失败和 repo 切换只在声明使用时验收 |
| Restore、scaleOut 和 rebuild | Restore、Cluster restore spec、scaleOut.fromBackup、RebuildInstance | restore Job/PVC、prepareData/postReady、ReadyConfig、sourceBackupTargetName、memberJoin/memberLeave、Service endpoint | 新集群 restore、scaleOut.fromBackup、rebuild in-place/replace 是不同消费者；RestoreKubeResources、deferPostReady、PITR 和多 target 映射必须有矩阵或写未证明 |
| Ops 状态和失败现场 | OpsRequest `phase/message/conditions`、precondition、并发、force/enqueueOnForce/cancel/timeout/TTL/progress/lastConfiguration | Component、InstanceSet/Instance、Pod、PVC、容器文件系统、action 输出、events、业务自检 | Ops 字段只是证据；partial success、Stop 空列表、sharding 字段名和 volume expansion 要按最终对象判断，事件和 action 输出采集完前不要过早清理失败现场 |
| 非主 workload | Backup/Restore/action worker、Job、Pod | 实际 env、SA、node、volumeMount、resources、runtimeClass、events | 主组件 runtime 的字段不能自动外推到 worker；需要实现或 e2e 证据时，只写“未证明”或“需自证” |

| 操作 | 先查入口对象 | 再查底层对象 | 常见 addon 问题 | 推荐处理 |
| --- | --- | --- | --- | --- |
| 创建 Cluster | Cluster topology、componentSpecs、serviceVersion、volumeClaimTemplates | resolved ComponentDefinition、ComponentVersion、Component、InstanceSet、Pod、Service | default topology 缺失；compDef 正则命中错误版本；PVC 名和 volumeMount 不一致；脚本 ConfigMap 没挂载 | 先修 chart 名称、topology、volume 和脚本挂载合同 |
| 删除 Cluster/Component | Cluster/Component deletionTimestamp、phase、conditions、finalizers | ComponentDefinition lifecycle、InstanceSet、owned Pods、preTerminate action | preTerminate 依赖 Pod，但 Pod 已提前消失；删除顺序没有覆盖 proxy/server 依赖；脚本不幂等 | 先解释 Pod/InstanceSet 为何消失；确认已经不需要执行清理动作时，再按产品文档化的恢复机制显式跳过该动作 |
| 参数变更 | OpsRequest reconfigure、ComponentParameter、ParametersDefinition | template ConfigMap、runtime ConfigMap、reconfigure action、Pod 重启状态 | PD.templateName 指错对象；fileName 不存在；static/dynamic 分类错误；reload 脚本没处理参数 diff | 按 PD -> ComponentParameter -> rendered ConfigMap -> action 输出逐层定位 |
| restart/stop/start | OpsRequest、Component phase | InstanceSet、Pod、container command、postStart/preStop 脚本 | 启动脚本不是幂等的；一次性初始化反复执行；进程退出码和 readiness 不一致 | 修启动脚本幂等性和健康检查，不把 restart 失败归因到 Ops 层 |
| horizontal scale | OpsRequest、Component replicas | InstanceSet replicas、Pod role、memberJoin/memberLeave | scale out 后未加入复制组；scale in 前未迁移或未摘除成员；roleProbe 没更新 | 补齐 member lifecycle 或明确该引擎不支持在线扩缩容 |
| volume expansion | OpsRequest、Component volumeClaimTemplates | PVC、StorageClass、Pod mountPath、文件系统扩容 | volumeClaimTemplates 名称和 runtime mount 不一致；引擎需要内部扩容命令但脚本未提供 | 先验证 PVC 和挂载路径，再验证引擎是否感知新容量 |
| switchover | OpsRequest、Component role 状态 | roleProbe、roleSelector service、switchover action、candidate Pod | roleProbe 不稳定；candidate 选择和脚本预期不一致；切主后服务没有选到新 primary | 先证明 roleProbe 和 roleSelector 正确，再调 switchover 脚本 |
| backup | BackupPolicy、Backup、BackupPolicyTemplate | Backup target Pod、system account secret、ActionSet、data volume | BPT 没匹配 resolved compDef；target role 不存在；backup 账号缺权限；method 语义混乱 | 先看 BackupPolicy 是否由 BPT 生成，再看 target 和 ActionSet |
| restore/rebuild | Restore 或 rebuild OpsRequest | Restore Job/PVC、prepareData/postReady、目标 Cluster/Component | 只验证 backup 未验证 restore；TLS、账号、拓扑或版本不兼容；restore 脚本只支持新集群不支持 rebuild | 每个 backup method 必须有独立 restore 验证和兼容矩阵 |
| upgrade | Cluster/Component serviceVersion、ComponentVersion | resolved release、image、rollout、role/updateStrategy | cmpv release 没匹配到 cmpd；image key 不对应容器/action；升级后参数 schema 或备份方法不兼容 | 先证明 serviceVersion 解析、镜像替换和 rollout 顺序，再验证数据兼容 |

这个排障表不是要求每个 addon 都支持所有操作，而是要求文档把“支持”“不支持”“需要停机”“需要人工步骤”分清楚。缺少底层合同的操作不能在 README 或 examples 中写成已支持。

常用检查命令模板：

```bash
# 创建和基础实例化
kubectl get cluster <cluster> -o yaml
kubectl get component -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get componentdefinition,componentversion,clusterdefinition | grep <addon-or-engine>
kubectl get instancesets,pods,pvc,svc -l app.kubernetes.io/instance=<cluster>
kubectl describe pod -l app.kubernetes.io/instance=<cluster>

# 参数变更
kubectl get opsrequest <ops> -o yaml
kubectl get parametersdefinition <pd> -o yaml
kubectl get componentparameter -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get configmap -l app.kubernetes.io/instance=<cluster> -o yaml

# 备份恢复
kubectl get backuppolicy,backup,restore -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get backuppolicytemplate <bpt> -o yaml
kubectl get actionset <actionset> -o yaml
kubectl get backupschedule -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get cronjob,backup -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get pods,jobs -l dataprotection.kubeblocks.io/backup-name=<backup>
kubectl get pods,jobs -l dataprotection.kubeblocks.io/restore-name=<restore>

# 运维操作
kubectl get opsrequest <ops> -o yaml
kubectl get component <component-object-name> -o yaml
kubectl get pods -l apps.kubeblocks.io/component-name=<component-object-name> -o wide
kubectl get pvc -l app.kubernetes.io/instance=<cluster>
kubectl get svc,endpointslice -l app.kubernetes.io/instance=<cluster> -o yaml
kubectl get serviceaccount,role,rolebinding -l app.kubernetes.io/instance=<cluster> -o yaml
```

## 13. 最低验收标准

addon 功能不能只靠 README 声明。每个功能至少要有四类证据：

- 实现文件：模板、脚本、values、CUE。
- 渲染对象：`helm template` 后实际出现的资源。
- 运行场景：用户或 e2e 如何触发。
- 观测结果：status、condition、Pod、Backup、Restore、OpsRequest、action 输出。

推荐验收清单：

- `helm lint` 和 `helm template` 成功，关键资源都渲染出来；至少用覆盖 standalone/HA/sharding/TLS/external ServiceRef/多 serviceVersion 的 values 组合跑渲染闭合检查。
- 对关键 examples 执行 server-side dry-run，并把 dry-run 输出、实际 apply 后对象、下游 Component/BackupPolicy/ComponentParameter/Job 做 diff；本地渲染和 server 最终对象不一致时，以 server 结果为准。
- 测试分层要清楚：chart 单元测试验证模板和 helper，server-side dry-run 验证 API/schema/defaulting，生成对象检查验证 controller 消费链路，live smoke 验证 Pod/Service/Job/action 真能运行，故障用例验证排查路径和限制说明。缺哪一层，就不要把对应能力写成已证明。
- 发布升级清单覆盖旧 definition/policy/action 资源、关闭功能后的残留 BackupPolicy/BackupSchedule、测试资源剔除和 cluster-scoped 命名冲突；每项写明保留、迁移、删除或不支持。
- helper、模板引用和 examples 里的名字闭合：`clusterDef/topology/component name`、direct `componentDef` 样例、CMPD config/script volume、PD `templateName/fileName`、BPT `compDefs`、ActionSet 名称和 service 端口都能互相指到。
- `ComponentDefinition`、`ClusterDefinition`、`ParametersDefinition`、`BackupPolicyTemplate`、`ActionSet` 进入 Available 或可被引用状态。
- 会被其它对象引用的 definition、policy 和 action 资源，凡 status 暴露 `observedGeneration` 的，都要证明它已追上当前 `generation`；只看 `phase=Available` 不足以排除旧状态。
- 最小 Cluster 创建成功，Pod Ready，Service 可连接。
- 每个声明支持的 topology 都要独立验收，而不是只验默认 topology：ComponentDefinition/ComponentVersion、ClusterDefinition orders、ParametersDefinition、BackupPolicyTemplate、ActionSet、systemAccounts、TLS、Service/ServiceRef、examples 和最低 Backup/Restore 或限制说明都要闭合。
- 声称支持 HA、TLS、backup、restore、upgrade、reconfigure 等组合能力时，不能只给一个 all-in-one example。至少要拆出每项能力的最小成功用例和一个能暴露边界的失败用例，例如缺 roleProbe 的 switchover、TLS on/off 的 backup/restore、非法参数或删参、旧版本升级、新集群 restore 与 rebuild 的差异。
- HA 场景能识别 role，switchover 可执行；candidate 和无 candidate 两种路径至少证明一种已支持，未覆盖路径写限制。
- 默认 resources、storage、数据目录初始化、脚本 defaultMode、工具镜像可拉取性和镜像 registry 来源要进入最低 smoke；这些问题一旦失败，表面现象经常是 Cluster 创建或 Pod Ready 问题，但根因属于 addon 合同。
- 参数变更能正确区分 reload 和 restart，并覆盖参数删除、Optional 变量缺失、多配置文件和脚本重复执行。
- backup 完成后能 restore，且 restore 后可读写或可执行明确的健康检查；artifact manifest、空数据集、加密 payload、formatVersion、TLS/账号、PITR、增量、rebuild、跨版本和跨拓扑未验证时必须标为未证明。
- 升级验收同时检查 ComponentVersion resolved serviceVersion、runtime/init/action image、参数 schema、BPT method env `versionMapping` 和数据兼容矩阵。

下面这些是“声明才验收”的扩展能力，不是所有 addon 的最低必选项。只要 README、values、examples、BPT、ActionSet、Cluster 示例或脚本声明了能力，就必须提供对应证据；没有证据时写“不支持”或“未证明”：

- 网络覆盖：hostAliases、dnsPolicy/dnsConfig、hostNetwork、hostPort、LoadBalancer advertised address、跨 namespace ServiceRef。
- Service 行为：publishNotReadyAddresses、podService、额外 ComponentService、Expose override、Service annotation/port 覆盖。
- 身份和 HA：`replicas=0`、quorum role、`participatesInQuorum`、role updatePriority、业务 fencing。
- 配置参数：外部 ConfigMap、`configHash`、`ComponentFileTemplate.variables.defaultValue`、formatter、参数 key 规范化。
- 数据保护：备份加密 passphrase、method formatVersion、snapshot artifact、repository artifact、Selective backup/restore、RestoreKubeResources、PITR/continuous backup。
- Restore 消费者：scaleOut.fromBackup、RebuildInstance in-place/replace、跨 topology/sharding restore、多 target `sourceBackupTargetName` 映射。
- Ops 证据：force/enqueueOnForce、TTL、progress、lastConfiguration、Stop 空列表、sharding Ops 字段。

故障类验收还要额外证明：

- 参数链路失败时，能定位到 PD、ComponentParameter、template ConfigMap、runtime ConfigMap 中哪一层断开。
- 创建、删除、reconfigure、scale、switchover、backup/restore、upgrade 这些操作失败时，能说明入口 CR、resolved definition、底层对象和脚本动作分别处于什么状态。
- 所有绕过执行、清理或一致性检查的恢复手段，只能作为明确风险的运维恢复，不应掩盖 addon 设计错误。

验收结论遵循一个原则：没有最终对象、执行面、脚本输出和业务自检证据的能力，只能标为“不支持”“未证明”或“当前 API 缺口”，不能写成已支持。
