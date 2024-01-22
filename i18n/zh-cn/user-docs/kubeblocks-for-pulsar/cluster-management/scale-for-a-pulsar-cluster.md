---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [pulsar, 水平扩缩容, 垂直扩缩容]
sidebar_position: 2
sidebar_label: 扩缩容
---

# Pulsar 集群扩缩容

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，如果你需要将资源类别从 1C2G 更改为 2C4G，就需要进行垂直扩容。

:::note

在垂直扩容时，所有的 Pod 将按照 Learner -> Follower -> Leader 的顺序重启。重启后，主节点可能会发生变化。

:::

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。 

```bash
kbcli cluster list pulsar
```

### 步骤

1. 更改配置。共有 3 种方式进行垂直扩容。

   **选项 1.** (**推荐**) 使用 kbcli

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ```bash
   kbcli cluster vscale pulsar --cpu=3 --memory=10Gi --components=broker,bookies  
   ```

   - `--components` 表示可进行垂直扩容的组件名称。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的CPU大小。

   **选项 2.** 创建 OpsRequest
  
   将 OpsRequest 应用于指定的集群，根据需求配置参数。

   ```bash
   kubectl create -f -<< EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     generateName: pulsar-vscale-
   spec:
     clusterRef: pulsar
     type: VerticalScaling
     verticalScaling:
     - componentName: broker
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3
     - componentName: bookies
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3      
   EOF
   ```
  
   **选项 3.** 使用 `kubectl` 编辑 Pulsar 集群。

   ```bash
   kubectl edit cluster pulsar
   ```

2. 验证垂直扩缩容。

    ```bash
    kbcli cluster list pulsar
    ```

   - STATUS=VerticalScaling 表示正在进行垂直扩容。
   - STATUS=Running 表示垂直扩容已完成。
   - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
     > 你可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

    :::note

    垂直扩容不会同步与 CPU 和内存相关的参数，需要手动调用配置的 OpsRequest 来进行更改。详情请参考[配置](../configuration/configuration.md)。

    :::

3. 检查资源是否已经发生更改。

    ```bash
    kbcli cluster describe pulsar
    ```

## 水平扩缩容

水平扩缩容会改变 Pod 的数量。例如，你可以应用水平扩容将 Pod 的数量从三个增加到五个。扩容过程包括数据的备份和恢复。

### 开始之前

- Zookeeper 建议固定 3 节点，无需扩缩容，其他可以针对多个或单个组件进行水平扩缩容。
- 谨慎扩缩容 Bookies 节点。其数据复制与 EnsembleSize、Write Quorum 和 Ack Quorum 配置有关，扩缩容可能导致数据丢失。详细信息请查阅 [Pulsar 官方文档](https://pulsar.apahe.org/docs/3.0.x/administration-zk-bk/#decommission-bookies-cleanly)。

### 步骤

1. 更改配置，共有 3 种方式。

   **选项 1.** (**推荐**) 使用 kbcli

   配置参数 `--components` 和 `--replicas`，并执行以下命令。

   ```bash
   kbcli cluster hscale pulsar --replicas=5 --components=broker,bookies                  Running        Jan 29,2023 14:29 UTC+0800
   ```

   - `--components` 表示准备进行水平扩容的组件名称。
   - `--replicas` 表示指定组件的副本数。

   **选项 2.** 创建 OpsRequest

   可根据需求配置参数，将 OpsRequest 应用于指定的集群。

    ```bash
    kubectl create -f -<< EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      generateName: pulsar-horizontalscaling-
    spec:
      clusterRef: pulsar
      type: HorizontalScaling  
      horizontalScaling:
      - componentName: broker
        replicas: 5
      - componentName: bookies
        replicas: 5
    EOF
    ```

   **选项 3.** 使用 `kubectl` 编辑 Pulsar 集群。

   ```bash
   kubectl edit cluster pulsar
   ```
  
2. 验证水平扩缩容。

   检查集群状态，确定水平扩容的情况。

   ```bash
   kubectl get ops
   >
   NAME                             TYPE               CLUSTER   STATUS    PROGRESS   AGE
   pulsar-horizontalscaling-9lfvc   HorizontalScaling  pulsar    Succeed   3/3        8m49s
   ```

3. 检查相关资源规格是否已变更。

   ```bash
   kbcli cluster describe mysql-cluster
   ```

### 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mysql-cluster-mysql-scaling-dbqgp: Failed to set default snapshot
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

2. 删除错误的备份（volumesnapshot 由备份生成）和 volumesnapshot 资源。

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=mysql-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mysql-cluster
   ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到运行状态。