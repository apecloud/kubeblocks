---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [postgresql, 垂直扩缩容]
sidebar_position: 2
sidebar_label: 扩缩容
---

# PostgreSQL 集群扩缩容

KubeBlocks 支持对 PostgreSQL 集群进行垂直扩缩容。

:::note

集群垂直或水平扩缩容后，KubeBlocks 会根据新的规格自动匹配合适的配置模板。这因为 KubeBlocks 在 v0.9.0 中引入了动态配置功能。该功能简化了配置参数的过程，节省了时间和精力，并减少了由于配置错误引起的性能问题。有关详细说明，请参阅[配置](./../configuration/configuration.md)。

:::

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list <name>
```

***示例***

```bash
kbcli cluster list pg-cluster
>
NAME         NAMESPACE   CLUSTER-DEFINITION           VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
pg-cluster   default     postgresql-cluster           postgresql-14.7.0   Delete               Running   Mar 03,2023 18:00 UTC+0800
```

### 步骤

1. 更改配置。

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ```bash
   kbcli cluster vscale pg-cluster \
   --components="postgresql" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` 表示可进行垂直扩容的组件名称。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的 CPU 大小。

2. 验证垂直扩容。

    执行以下命令检查集群状态，验证垂直扩容。

    ```bash
    kbcli cluster list pg-cluster
    >
    NAME              NAMESPACE        CLUSTER-DEFINITION            VERSION                TERMINATION-POLICY   STATUS    CREATED-TIME
    pg-cluster        default          postgresql-cluster            postgresql-14.7.0      Delete               Running   Mar 03,2023 18:00 UTC+0800
    ```

   - STATUS=VerticalScaling 表示正在进行垂直扩容。
   - STATUS=Running 表示垂直扩容已完成。
   - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
     > 你可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

3. 检查资源是否已经发生更改。

    ```bash
    kbcli cluster describe pg-cluster
    ```

## 水平扩缩容

水平扩缩容会改变 Pod 的数量。例如，你可以应用水平扩容将 Pod 的数量从三个增加到五个。

从 v0.9.0 开始，KubeBlocks 支持指定实例水平扩缩容，可参考 [API 文档](./../../../api-docs/maintenance/scale/horizontal-scale.md)，查看详细介绍及示例。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list pg-cluster
>
NAME              NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
pg-cluster        default          postgreql                 postgresql-14.8.0      Delete                    Running        Mar 03,2023 19:29 UTC+0800
```

### 步骤

1. 更改配置。

   配置参数 `--components` 和 `--replicas`，并执行以下命令。

    ```bash
    kbcli cluster hscale pg-cluster \
    --components="postgresql" --replicas=2
    ```

   - `--components` 表示准备进行水平扩容的组件名称。
   - `--replicas` 表示指定组件的副本数。可按需修改该参数值，对应执行扩缩容操作。

2. 验证水平扩容。

   检查集群状态，确定水平扩容的情况。

    ```bash
    kbcli cluster list pg-cluster
    ```

   - STATUS=Updating 表示正在进行水平扩容。
   - STATUS=Running 表示水平扩容已完成。

3. 检查相关资源规格是否已变更。

    ```bash
    kbcli cluster describe pg-cluster
    ```

### 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-08T04:20:26Z"
    message: VolumeSnapshot/pg-cluster-postgresql-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***原因***

此异常发生的原因是未配置 `VolumeSnapshotClass`。可以通过配置 `VolumeSnapshotClass` 解决问题。

但此时，水平扩容仍然无法继续运行。这是因为错误的备份（volumesnapshot 由备份生成）和之前生成的 volumesnapshot 仍然存在。需删除这两个错误的资源，KubeBlocks 才能重新生成新的资源。

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

2. 删除错误的备份和 volumesnapshot 资源。

    ```bash
    kubectl delete backup -l app.kubernetes.io/instance=pg-cluster
   
    kubectl delete volumesnapshot -l app.kubernetes.io/instance=pg-cluster
    ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
