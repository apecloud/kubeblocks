---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [mongodb, 垂直扩容, 垂直扩容 MongoDB 集群]
sidebar_position: 2
sidebar_label: 扩缩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# MongoDB 集群扩缩容

KubeBlocks 支持对 MongoDB 集群进行垂直扩缩容。

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

### 开始之前

确保集群处于 `Running` 状态，否则后续操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME             NAMESPACE        CLUSTER-DEFINITION    VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster        demo             mongodb               mongodb-5.0        Delete                    Running        Apr 10,2023 16:20 UTC+0800
```

</TabItem>

</Tabs>

### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mongodb
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. 查看运维任务状态，验证垂直扩缩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ...
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   在编辑器中修改 `spec.componentSpecs.resources` 的参数值。

   ```yaml
   ...
   spec:
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - componentDefRef: mongodb
       enabledLogs:
       - running
       disableExporter: true
       name: mongodb
       replicas: 2
       resources: # 修改参数值
         limits:
           cpu: "2"
           memory: 4Gi
         requests:
           cpu: "1"
           memory: 2Gi
   ...
   ```

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ...
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

     ***示例***

     ```bash
     kbcli cluster vscale mycluster -n demo --components=mongodb --cpu=500m --memory=500Mi
     ```

   - `--components` 表示可进行垂直扩容的组件名称。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的 CPU 大小。

2. 通过以下任意一种方式验证垂直扩容是否完成。

   - 查看 OpsRequest 进度。

     执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

     ```bash
     kbcli cluster describe-ops mycluster-verticalscaling-g67k9 -n demo
     ```

   - 查看集群状态。

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    CREATED-TIME                 
     mycluster   demo        mongodb              mongodb-5.0      Delete               Running   Apr 26,2023 11:50 UTC+0800    
     ```

     - STATUS=Updating 表示正在进行垂直扩容。
     - STATUS=Running 表示垂直扩容已完成。
     - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
       > 您可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者您也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

   :::note

   垂直扩容不会同步与 CPU 和内存相关的参数，需要手动调用配置的 OpsRequest 来进行更改。详情请参考[配置](./../../kubeblocks-for-mongodb/configuration/configure-cluster-parameters.md)。

   :::

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## 水平扩缩容

水平扩缩容会改变 Pod 的数量。例如，您可以应用水平扩容将 Pod 的数量从三个增加到五个。

从 v0.9.0 开始，KubeBlocks 支持指定实例水平扩缩容，可参考 [水平扩缩容文档](./../../maintenance/scale/horizontal-scale.md)，查看详细介绍及示例。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY     STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME          NAMESPACE     CLUSTER-DEFINITION    VERSION          TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster     demo          mongodb               mongodb-5.0      Delete                    Running        April 26,2023 12:00 UTC+0800
```

</TabItem>

</Tabs>

### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   以下示例演示了增加 2 个副本。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mongo-horizontalscaling
     namespace: default
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mongodb
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   如果您想要缩容，可将 `scaleOut` 替换为 `scaleIn`。

   以下示例演示了删除 2 个副本。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mongo-horizontalscaling
     namespace: default
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mongodb
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. 查看运维任务状态，验证垂直扩缩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>
  
<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.replicas` 的配置。`spec.componentSpecs.replicas` 定义了 pod 数量，修改该参数将触发集群水平扩缩容。

   ```yaml
   kubectl edit cluster mycluster -n demo
   ```

   在编辑器中修改 `spec.componentSpecs.replicas` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: mongo
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - name: mongo
       componentDefRef: mongo
       replicas: 4 # 修改该参数值
   ...
   ```

2. 当集群状态再次回到 `Running` 后，查看相关资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

   配置参数 `--components` 和 `--replicas`，并执行以下命令。

   ```bash
   kbcli cluster hscale mycluster -n demo --components="mongodb" --replicas=2
   ```

   - `--components` 表示准备进行水平扩容的组件名称。
   - `--replicas` 表示指定组件的副本数。

2. 通过以下任意一种方式验证水平扩容是否完成。

   - 查看 OpsRequest 进度。

     执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

     ```bash
     kbcli cluster describe-ops mycluster-horizontalscaling-ffp9p -n demo
     ```

   - 查看集群状态。

     ```bash
     kbcli cluster list mycluster -n demo
     ```

   - STATUS=Updating 表示正在进行水平扩容。
   - STATUS=Running 表示水平扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查相关资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-08T04:20:26Z"
    message: VolumeSnapshot/mongodb-cluster-mongodb-scaling-dbqgp: Failed to set default snapshot
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
    kubectl delete backup -l app.kubernetes.io/instance=mycluster -n demo
   
    kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster -n demo
    ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
