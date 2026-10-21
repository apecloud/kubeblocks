# Workloads 执行 hscale 动作的调整方案

本文记录 #10958 的职责边界、实现方式和验收要求。Component 接入、兼容和存量迁移另行处理。

## 职责边界

Component 继续构造和注入 kbagent、kbagent init、worker 容器及其基础运行环境。Workloads 使用已经准备好的环境，维护调用 data、membership actions 所需的动作配置、参数、任务和结果处理。

独立使用 ITS 时，由调用方在 PodTemplate 中准备相同的运行环境。Workloads 检查环境是否满足执行要求，不再通过 `LifecycleActions.Worker` 接收另一份容器定义，也不补造缺失的 agent 容器。

这次调整删除以下设计。

- `LifecycleActions.Worker`。
- `LifecycleActions.DataVolume`。
- PVC 上的 `workloads.kubeblocks.io/data-loaded` 完成标记，以及为读写该标记增加的协议和逻辑。

数据加载是否完成，使用实例维度的 `DataLoaded` 表达。ITS 直接维护 `ITS.status.instanceStatus`。ITS2 从 Instance status 聚合 `Provisioned`、`DataLoaded`、`MemberJoined`，两层的字段类型和含义保持一致。

ITS 直接管理 Pod 和 PVC。ITS2 通过 Instance 管理 Pod 和 PVC。两条路径使用相同的动作和结果约定，分别维护自身管理的资源。

当前 PR 仍然只把 workloads 准备到可接入的程度，不切换 Component 的动作执行职责，不修改 Component 私有 annotation writer。Workloads 不读取 Component 的私有任务配置，不依赖 Component 包。`InstanceLifecycle` condition 的命名另行讨论。

## API 和持久化变更范围

**已确认的新增 API：Instance 的 `InstanceStatus2` 扩展以下三个字段，与 ITS 的 `InstanceStatus` 保持一致。这是本次明确授权的状态扩展。**

```go
Provisioned  bool  `json:"provisioned,omitempty"`
DataLoaded   *bool `json:"dataLoaded,omitempty"`
MemberJoined *bool `json:"memberJoined,omitempty"`
```

`Provisioned` 表示实例已经创建过运行资源。`DataLoaded` 表示数据加载结果。`MemberJoined` 表示成员加入或退出的结果。停止和 Pod 重建保留需要继续使用的实例结果，不因为当前没有 Pod 就清空它们。

**其他新增 API 或持久化状态仍需先确认。** 不增加任务 ConfigMap、完成 annotation、结果文件、任务记录或额外身份字段，不把操作意图或执行 plan 保存下来供下轮照着执行。

| 内容 | 本方案的处理 | 是否需要确认 |
| --- | --- | --- |
| Workloads API | 删除 `Worker`、`DataVolume` | 已确定的调整范围 |
| Instance status | 增加 `Provisioned bool`、`DataLoaded *bool`、`MemberJoined *bool` | **用户已确认，属于新增 API 和持久化结果字段** |
| ITS 实例状态 | ITS2 从 Instance status 聚合上述字段 | 与直接管理 Pod 的 ITS 保持同一含义 |
| 任务输入 | 沿用已有 PodSpec、Instance template 中的 `KB_AGENT_TASK` 和现有 `proto.Task` | 不新增存储种类 |
| 加载完成观察 | 观察当前数据任务对应 worker 的成功终止结果，再写实例状态 | 不把 Event 通知作为完成状态的必要来源 |
| 其他结果存储或字段 | 不增加 | 如有新的必要性，必须单独说明并获得确认 |

现有资源能够保存任务输入，不等于必须再创建任务 ConfigMap。API 没加字段，也不等于实现没有新增持久化状态。

## 当前可以复用什么

已有 `proto.Task` 包含目标实例、任务标识、源地址和参数。`BuildEnv4Worker`、`UpdateEnv4Worker` 可以继续用于任务编解码，不需要引入新的执行框架。

Pod 已有 init container 的运行和终止状态。Workloads 根据当前数据任务及对应 worker 的成功终止结果确认加载完成，不能把任意 init 退出码为零直接当成成功，也不能用业务容器是否 Ready 代替加载结果。

Agent 已有 TaskEvent 通知能力，但本方案不以 Event 交付作为完成依据，不新增跨集群任务通知链。`newReplicaTask.status` 的任务查询尚未实现，也不为本次改动补建一套查询框架。

Lifecycle action 本身要求可重入。加载成功但完成状态尚未提交时，可以安全重试，不需要另外保存“执行过一次”的标记。原 Component 流程的“先清任务，再写结果”不再沿用，必须先成功提交实例的 `DataLoaded`，再清理任务输入。

## 任务触发

每轮协调都重新读取当前 spec、实例状态和实际资源，决定是否需要执行动作。已有任务只是可观察的运行输入，不代表上轮的执行安排仍然有效。

首次初始化与扩容沿用当前实例登记规则。首次创建的实例不因为没有 `DataLoaded=true` 就一律执行复制。后续新分配且确实需要复制数据的实例登记 `DataLoaded=false`，需要加入成员的实例登记 `MemberJoined=false`。停止期间登记的新实例也必须保留这些要求，恢复后不能因实例名已经登记而漏掉动作。

`DataLoaded` 的值按下面的含义使用。

| 值 | 含义 |
| --- | --- |
| `nil` | 当前实例没有登记数据加载要求，不能据此推断执行过加载 |
| `false` | 已登记加载要求，尚未确认完成 |
| `true` | 当前实例已确认完成加载 |

对 `DataLoaded=false` 的活动实例，workloads 选择可用 donor，生成已有格式的任务，并把动作配置和任务输入放到已存在的 agent 容器中。源端执行 DataDump，目标端 worker 执行 DataLoad。缺少运行环境时报告等待或配置错误，不再使用 `Worker` 字段补造环境。

健康 donor 和当前任务没有变化时，保持已有任务，避免重新选择来源中断加载。恢复时先恢复能够提供数据的实例，不因为先遇到待加载实例就挡住后面的 donor。

完成数据加载后，alignment 才按当前实例状态执行 MemberJoin。缩容时仍由 alignment 执行所需的 MemberLeave，再推进释放。停止本身不等同于退组，不因此清掉已经完成的数据状态。

## 结果观察与状态提交

ITS 在现有 status reconciler 中观察自己管理的 Pod。当前数据任务对应的 worker 成功结束后，把该实例的 `DataLoaded` 更新为 `true`。

ITS2 采用现有 Instance status 聚合链。Instance 的 status reconciler 观察自己管理的 Pod，维护新增的三个结果字段。ITS2 从 Instance status 读取并聚合到 `ITS.status.instanceStatus`，不绕过 Instance 直接接收 agent 完成通知来写 ITS 的加载结果。

状态提交与任务清理执行以下顺序。

1. 对当前资源和任务进行观察，确认数据加载已经完成。
2. ITS 提交对应实例的状态；ITS2 路径先提交 Instance status，再由 ITS2 聚合提交 ITS status。
3. 提交失败就保留任务输入，下轮重新观察和提交。冲突后重新读取并重新计算，不覆盖其他字段的合法更新。
4. 本层完成状态提交成功后，清理本层维护的任务输入。上层尚未聚合时，以 Instance 已提交的结果为依据，不重新给它安排加载。

没有完成状态而原 Pod 已经消失时，可以根据当前目标重建 Pod 并重试可重入动作，不要求找回一份独立的永久任务结果。已经完成的实例不会因为暂时没有 Pod、Pod 未 Ready 或一次旧观察而退回未完成。

Membership 动作沿用现有 lifecycle 调用和 alignment 调度。ITS 直接维护对应结果。ITS2 执行动作后先把结果提交到对应 Instance 的 `MemberJoined`，再从 Instance status 聚合；Instance 普通 status 更新保留这个结果，不额外执行同一个 membership 动作。动作成功而状态提交失败时安全重试，不增加“动作执行中”或“已调用”之类的持久化状态。

## ITS 与 ITS2 分别改哪里

| 环节 | ITS | ITS2 |
| --- | --- | --- |
| 任务触发 | alignment 根据当前实例状态，配置直接管理的 Pod | ITS2 alignment 配置 Instance template，Instance 继续负责落地 Pod |
| 动作运行环境 | 使用 PodTemplate 中已准备好的 server、worker 和 init | 使用传入 Instance template 的同一运行环境，不增加 `Instance.spec` lifecycle 字段 |
| 加载结果 | 原 status reconciler 观察对应 worker，写 ITS 实例状态 | Instance 观察对应 worker 并写自身 status，ITS2 从 Instance status 聚合 |
| 成员结果 | 现有 alignment 执行动作后提交实例结果 | 现有 ITS2 alignment 执行动作后先提交 Instance 成员结果，再聚合到 ITS |
| 普通状态汇总 | 原 status reconciler 保留动作结果并汇总可用性 | Instance 保留已提交结果，ITS2 聚合 Instance 的三个结果字段及已有运行状态 |
| 任务清理 | 完成后不再给新 Pod 生成任务，清理自己维护的可修改输入 | 同时清理 Instance template 和能够重新下发任务的 assistant object 内容 |
| Pod 重建 | `DataLoaded=true` 的实例生成不带加载任务的新 Pod | Instance 根据自身已提交的 `DataLoaded` 去掉加载任务，即使 ITS2 尚未清理 template，也不会重新加载 |
| 远端运行 | 沿用已有资源观察能力 | 沿用已有 Instance status 读取和聚合能力，不新增 agent 到 ITS 的通知通道 |

共享包只保留任务编解码、配置和结果解释等公共逻辑。登记、状态汇总、动作执行分别放回现有 status 与 alignment 流程，不增加专用 reconciler 或通用任务调度器。

## status 写入失败和清理失败怎么恢复

| 中断位置 | 恢复方式 | 限制 |
| --- | --- | --- |
| 数据任务仍在执行 | 保留当前有效任务，继续观察 | 不因随机更换健康 donor 重建目标 Pod |
| worker 成功结束，实例 status 写入失败 | 保留任务，下轮重新观察 worker 并提交 | 若运行资源已经消失，可安全重试 action |
| Instance status 已提交，ITS2 聚合写入失败 | 重新读取 Instance status 并提交 ITS 状态 | 不能把上层尚未聚合视为 Instance 需要重新加载 |
| status 已提交，清理前重启 | 根据 `DataLoaded=true` 继续清理 | 完成状态不依赖是否已经清理 |
| 清理 Instance template 后，助手对象更新失败 | 下轮继续清理实际对象及其来源 | 不能让旧 assistant snapshot 把任务回灌 |
| status 已提交，整个 Pod 被重建 | 从当前实例状态生成不带加载任务的新 Pod | ITS2 同时依据 Instance 已提交的状态，避免上层观察延迟重新带入任务 |
| 完成状态提交前，worker 再次执行 | 按 lifecycle action 的可重入要求安全重试 | 不增加用于保证只执行一次的标记或资源 |
| 完成状态未提交，原 Pod 已消失 | 按当前目标重建并重试 action，再观察和提交结果 | 不根据推测把未确认的结果写成成功 |

清理只删除 workloads 生成的任务内容，保留基础 agent 环境、无关任务和外部配置。不为了清任务删除正在使用的 PVC，也不把完成标记换个名字写回其他资源。

对已完成实例的 Pod 重建，不重新登记 `DataLoaded=false`。对停止、恢复、缩容取消，重新按当前目标与资源判断，不沿用上轮操作安排。需要保留的实例结果仍由当前分配和资源保留策略决定。

## 实现和清理范围

以下改动都在 #10958 中完成。

- API、生成代码和 CRD 文档删除 `Worker`、`DataVolume`，在 Instance status 增加已确认的三个字段，保留 Action 与 ComponentDefinition 对齐的定义。
- `pkg/controller/workloads/lifecycle` 删除 Worker 容器构造、DataPVCName、PVC 完成判定，以及仅为它们服务的环境变量和任务匹配逻辑。ObserveData 改为观察 Pod worker 的实际执行结果，使用已准备好的容器。
- `pkg/kbagent/proto`、`setup.go`、`service/task_new_replica.go` 删除 `DataLoadResult`、PVC 标记、结果环境变量及相关读写权限需求。保留通用任务和通知能力。
- ITS 的 tree loader、status、alignment 删除 PVC 结果观察和对应恢复分支，接入确认后的任务结果流程。
- ITS2 与 Instance 删除对应 PVC 结果逻辑。Instance status reconciler 维护加载结果和实例结果的保留，ITS2 status reconciler 聚合结果。调整 membership 结果写入、template、助手对象和更新比较，避免残留输入重新下发。
- 不为本次改动增加 Event handler 或远端通知通道，不修改 Component 内部动作与私有状态实现。
- 重写以 PVC 标记为前提的测试，保留仍符合最终契约的失败恢复覆盖。删除中间方案专用 fixture、helper、注释和文档。

## 验收要求

ITS、ITS2 都通过真实 controller 入口验证以下行为，不只测试 helper。

1. 首次初始化、后续扩容，以及停止期间扩容后恢复，正确区分是否需要加载和加入成员。
2. 使用已准备好的 agent 环境执行 DataDump、DataLoad、MemberJoin、MemberLeave，不需要 `Worker`、`DataVolume` 或 Component 私有任务配置。
3. 健康 donor 不因重复协调被随意更换，恢复过程不被等待 donor 的目标实例挡住。
4. 只有匹配当前数据任务的 worker 成功结束才能完成加载状态。未运行任务、仍在运行、执行失败，以及无关 init 成功都不能误报完成。
5. worker 成功结束而实例 status 连续写失败时，不清任务。重新观察或安全重试后能够提交结果。提交成功后即使立即重启 controller，也能完成清理。
6. status 提交后 Pod 重建不加载。ITS2 的 template 和实际助手对象都清理到位，旧任务不会重新下发。
7. 完成状态提交前重启 worker，可以重复执行可重入 action 并收敛，不需要额外的执行标记。尚无完成结果且 Pod 消失时能够重试，不误报成功。
8. ITS2 中 Instance status 已提交、ITS 聚合连续失败时，重新读取后可恢复，不重复安排已完成实例的加载。已有远端 Instance status 聚合路径同样覆盖新增字段，不依赖 Event 交付。
9. 停止、恢复、缩容取消、删除和资源保留不依赖 PVC 完成标记，不误删外部资源。
10. 全量 diff 不再残留被删除字段、PVC 标记、旧权限要求和过渡测试。按 AGENTS.md 运行相应回归、生成检查和完整 lint。

## 核对源码

以下位置对应上述职责。

- [当前 workloads 动作和任务配置](../../pkg/controller/workloads/lifecycle/lifecycle.go)。
- [现有任务执行能力](../../pkg/kbagent/service/task.go)。
- [Instance status API](../../apis/workloads/v1/instance_types.go)。
- [Instance 状态观察](../../pkg/controller/instance/reconciler_status.go)。
- [ITS2 状态聚合](../../pkg/controller/instanceset2/reconciler_status.go)。
- [旧 Component 任务生成与结果清理](../../pkg/controller/component/replicas.go)，只用于理解原有机制，不作为 workloads 依赖。
- [ITS 原生动作路径](../../pkg/controller/instanceset/lifecycle_utils.go)。
- [ITS2 原生动作和任务清理路径](../../pkg/controller/instanceset2/lifecycle_utils.go)。
- [Instance 助手对象维护](../../pkg/controller/instance/reconciler_assistant_object.go)。
- [现有实例结果字段的保留](../../pkg/controller/workloads/instancestatus/builder.go)。
