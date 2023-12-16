---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [mysql, 水平扩缩容, 垂直扩缩容]
sidebar_position: 2
sidebar_label: 扩缩容
---

# MySQL 集群扩缩容

KubeBlocks 支持对 MySQL 集群进行垂直扩缩容和水平扩缩容。

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，如果你需要将资源类别从 1C2G 更改为 2C4G，就需要进行垂直扩容。

:::note

在垂直扩容时，所有的 Pod 将按照 Learner -> Follower -> Leader 的顺序重启。重启后，主节点可能会发生变化。

:::

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

### 步骤

1. 更改配置，共有 3 种方式。

   **选项 1.**（**推荐**）使用 kbcli

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ```bash
   kbcli cluster vscale mysql-cluster \
   --components="mysql" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` 表示可进行垂直扩容的组件名称。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的CPU大小。

   **选项 2.** 创建 OpsRequest
  
   可根据需求修改参数，将 OpsRequest 应用于指定的集群。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
   spec:
     clusterRef: mysql-cluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mysql
       requests:
         memory: "2Gi"
         cpu: "1000m"
       limits:
         memory: "4Gi"
         cpu: "2000m"
   EOF
   ```
  
   **选项 3.** 修改集群的 YAML 文件

   修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源需求和相关限制，更改配置将触发垂直扩容。

   ***示例***

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: default
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 1
       resources: # 修改资源值
         requests:
           memory: "2Gi"
           cpu: "1000m"
         limits:
           memory: "4Gi"
           cpu: "2000m"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Halt
   ```

2. 查看集群状态，以验证垂直扩容是否成功。

    ```bash
    kbcli cluster list mysql-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS                 CREATED-TIME
    mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    VerticalScaling        Jan 29,2023 14:29 UTC+0800
    ```

   - STATUS=VerticalScaling 表示正在进行垂直扩容。
   - STATUS=Running 表示垂直扩容已完成。
   - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
     > 你可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

    :::note

    垂直扩容不会同步与 CPU 和内存相关的参数，需要手动调用配置的 OpsRequest 来进行更改。详情请参考[配置](./../../kubeblocks-for-mysql/configuration/configuration.md)。

    :::

3. 检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mysql-cluster
    ```

## 水平扩缩容

水平扩缩容会改变 Pod 的数量。例如，你可以应用水平扩容将 Pod 的数量从三个增加到五个。扩容过程包括数据的备份和恢复。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

### 步骤

1. 更改配置，共有 3 种方式。

   **选项 1.** (**推荐**) 使用 kbcli

   配置参数 `--components` 和 `--replicas`，并执行以下命令。

   ```bash
   kbcli cluster hscale mysql-cluster \
   --components="mysql" --replicas=3
   ```

   - `--components` 表示准备进行水平扩容的组件名称。
   - `--replicas` 表示指定组件的副本数。

   **选项 2.** 创建 OpsRequest

   可根据需求配置参数，将 OpsRequest 应用于指定的集群。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
   spec:
     clusterRef: mysql-cluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mysql
       replicas: 3
   EOF
   ```

   **选项 3.** 修改集群的 YAML 文件

   修改 YAML 文件中 `spec.componentSpecs.replicas` 的配置。`spec.componentSpecs.replicas` 控制 Pod 的数量，更改配置将触发水平扩容。

   ***示例***

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
    apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: default
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 1 # 修改 Pod 数
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
    terminationPolicy: Halt
   ```

2. 验证水平扩容。

   检查集群状态，确定水平扩容的情况。

   ```bash
   kbcli cluster list mysql-cluster
   ```

   - STATUS=HorizontalScaling 表示正在进行水平扩容。
   - STATUS=Running 表示水平扩容已完成。

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

2. 删除错误的备份和 volumesnapshot 资源。

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=mysql-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mysql-cluster
   ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
