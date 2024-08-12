---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [kafka, 水平扩缩容, 垂直扩缩容]
sidebar_position: 3
sidebar_label: 扩缩容
---

# Kafka 集群扩缩容

KubeBlocks 支持对 Kafka 集群进行垂直扩缩容和水平扩缩容。

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list
>
NAME    NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
ivy85   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
```

### 步骤

1. 更改配置。

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ```bash
    kbcli cluster vscale ivy85 --components="broker" --memory="4Gi" --cpu="2" 
   ```

   - `--components` 的值可以是 `broker` 或 `controller`。
     - broker：在组合模式下表示所有节点；在分离模式下表示所有 broker 节点。
     - controller：表示在分离模式下的所有对应节点。
   - `--memory` 表示组件内存的请求和限制大小。
   - `--cpu` 表示组件 CPU 的请求和限制大小。
  
2. 验证垂直扩容。

    ```bash
    kbcli cluster list mysql-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS                 CREATED-TIME
    ivy85                 default          kafka                kafka-3.3.2            Delete                    VerticalScaling        Jan 29,2023 14:29 UTC+0800
    ```

   - STATUS=Updating 表示正在进行垂直扩容。
   - STATUS=Running 表示垂直扩容已完成。
   - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
     > 你可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

    :::note

    垂直扩容不会同步与 CPU 和内存相关的参数，需要手动调用配置的 OpsRequest 来进行更改。详情请参考[配置](./../configuration/configuration.md)。

    :::

3. 检查资源是否已变更。

    ```bash
    kbcli cluster describe ivy85
    ```

## 水平扩缩容

水平扩缩容会改变 Pod 的数量。例如，你可以应用水平扩容将 Pod 的数量从三个增加到五个。

从 v0.9.0 开始，KubeBlocks 支持指定实例水平扩缩容，可参考 [API 文档](./../../../api-docs/maintenance/scale/horizontal-scale.md)，查看详细介绍及示例。

### 开始之前

- 确保集群处于 `Running` 状态，否则以下操作可能会失败。
- 不建议在 controller 节点上进行水平扩缩容（包括组合模式和分离模式的 controller 节点）。
- 在进行水平扩缩容时，必须了解主题分区的存储情况。如果主题只有一个副本，在 broker 扩缩容时可能会导致数据丢失。

```bash
kbcli cluster list
>
NAME    NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
ivy85   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
```

### 步骤

1. 更改配置。

   配置参数 `--components` 和 `--replicas`，并执行以下命令。

   ```bash
   kbcli cluster hscale mysql-cluster \
   --components="broker" --replicas=3
   ```

   - `--components` - 的值可以是 `broker` 或 `controller`。
     - broker：在组合模式下表示所有节点；在分离模式下表示所有 broker 节点。
     - controller：表示在分离模式下的所有对应节点。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的 CPU 大小。
   - `--replicas` 表示指定组件的副本数。

2. 验证水平扩容。

   检查集群状态，确定水平扩缩容的情况。

   ```bash
   kbcli cluster list ivy85
   ```

   - STATUS=Updating 表示正在进行水平扩容。
   - STATUS=Running 表示水平扩容已完成。

3. 检查相关资源规格是否已变更。

    ```bash
    kbcli cluster describe ivy85
    ```

### 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/ivy85-kafka-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***原因***

此异常发生的原因是未配置 `VolumeSnapshotClass`。可以通过配置 `VolumeSnapshotClass` 解决问题。

但此时，水平扩容仍然无法继续运行。这是因为错误的备份（volumesnapshot 由备份生成）和之前生成的 volumesnapshot 仍然存在。删除这两个错误的资源，KubeBlocks 才能重新生成新的资源。

***步骤：***

1. 配置 VolumeSnapshotClass。

   ```bash
   kubectl create -f - <<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotClass
   metadata:
     name: csi-aws-vsc
     annotations:
       snapshot.storage.kubernetes.io/is-default-class: "true"
   driver: ebs.csi.aws.com
   deletionPolicy: Delete
   EOF
   ```

2. 删除错误的备份（volumesnapshot 由备份生成）和 volumesnapshot 资源。

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=ivy85
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=ivy85

   ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
