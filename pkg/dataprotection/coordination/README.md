# 备份执行与退出证据核心

这个包实现 [DOC-253@v27](https://infracreate.com/project-manager/designs/fa09d968-fde0-447d-b64f-210ec14b6412?revisionNo=27) D22 的局部判据，对应 [Issue 11102](https://github.com/apecloud/kubeblocks/issues/11102)。尚未接入 Reconcile、CRD、Job 模板或准入，默认行为没有变化。

例如 Pod 已取得备份许可，但尚未开始复制，随后执行被封闭。该 Pod 仍属于必须等待退出的集合，不能清空记录后让另一个备份开始。`ValidateTransition` 拒绝丢失、替换参与者、身份变化、解除封闭、跳过阶段及改写已保存证据。它必须用于所有协调状态写入；后续 adapter 还必须以完整 UID 和 resourceVersion 做真实 API CAS。这里的纯函数不是 CAS、鉴权或持久化实现。

`CollectTermination` 从批准的 Pod 身份和实际 API Pod 生成退出证据。当前只接纳一个普通业务容器、Never 重启、固定摘要镜像，无 init、ephemeral、hook 或 probe。它逐项核对完整状态和当前容器实例；空列表、缺项、`lastTerminationState`、运行中、NodeLost、UID 或 spec 改变都不生成证据。标准 JSON spec 摘要只规范化调度器填入的 `nodeName`。非零退出码证明进程结束，但不表示备份成功。

`ApprovePod` 只捕获身份与结构，不能证明入口脚本、挂载、镜像字节或权限安全。可信 controller 后续必须校验固定 gate/supervisor profile、所有业务都在 gate 后、无远端执行/逃逸、不可变脚本输入、源域及删除保护。单容器或 PodSpec hash 本身都不能替代这些校验。runtime ImageID 被记录，不将镜像索引 digest 与运行时平台/config digest 错当成同一值。

`TerminalReceipt` 和 `ReleaseReceipt` 必须来自可信 adapter 的直接 API 读回，并绑定当前执行身份。任意调用方自己构造这些结构不能获得实际执行权限。退出证明先保存和读回，再提交并读回业务终态，之后才可尝试释放原 Lease。`ReadyToRelease` 不做释放，也不能清除后来的 holder。无登记 Pod 的执行只能以失败或取消收尾；完整数据和仓库结果成功条件仍由后续 adapter 验证。

待接入的必要部分包括真实 UID/RV 冲突、所有状态写者隔离、组件统一域、固定父链输入、gate/manager 在同一进程树内、主业务退出后上传完成、删除与 GC、仓库 deleter 的路径复用隔离。缺对象、未知退出或证据丢失保持未知，不自动接管。这里的测试不替代 envtest、真实 Job/节点故障或 PolarDB-M 备份恢复验收。

验证命令：

```sh
go test -mod=readonly -race -count=1 ./pkg/dataprotection/coordination
go vet -mod=readonly ./pkg/dataprotection/coordination
```
