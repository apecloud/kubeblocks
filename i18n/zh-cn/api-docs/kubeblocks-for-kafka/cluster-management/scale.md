---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [kafka, 水平扩缩容, 垂直扩缩容]
sidebar_position: 3
sidebar_label: 扩缩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Kafka 集群扩缩容

KubeBlocks 支持对 Kafka 集群进行垂直扩缩容和水平扩缩容。

## 垂直扩缩容

您可以通过更改资源需求和限制（例如 CPU 和存储）来实现集群垂直扩缩容。例如，您可以通过垂直扩缩容将资源类别从 1C2G 更改为 2C4G。

### 开始之前

检查集群状态是否为 `Running`。否则，后续操作可能会失败。

```bash
kubectl get cluster mycluster -n demo  
```

### 步骤

可通过以下两种方式实现垂直扩缩容。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterRef: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: broker
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

3. 查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  kafka
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      kafka
    Replicas:  2
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

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2
     componentSpecs:
     - name: broker
       componentDefRef: broker
       replicas: 1
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Delete
   ```

2. 查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  kafka
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      kafka
    Replicas:  2
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

</Tabs>

## 水平扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，除了支持副本（replica）的扩缩容外，KubeBlocks 还支持了实例（instance）的扩缩容。可通过 [水平扩缩容](./../../maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

### 开始之前

- 确保集群处于 `Running` 状态，否则后续操作可能会失败。
- 不建议在 controller 节点上进行水平扩缩容（包括组合模式和分离模式的 controller 节点）。
- 在进行水平扩缩容时，必须了解主题分区的存储情况。如果主题只有一个副本，在 broker 扩缩容时可能会导致数据丢失。

  ```bash
  kubectl get cluster mycluster -n demo  
  ```

### 步骤

可通过以下两种方式实现水平扩缩容。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   以下示例演示了增加 2 个副本。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterRef: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: broker
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   如果您想要缩容，可将 `scaleOut` 替换为 `scaleIn`。

   以下示例演示了删除 2 个副本。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterRef: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: broker
       scaleIn:
         replicaChanges: 2
   EOF

2. 查看运维操作状态，验证水平扩缩容是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  kafka
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      kafka
    Replicas:  2
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

1. 修改 YAML 文件中 `spec.componentSpecs.replicas` 的配置。`spec.componentSpecs.replicas` 定义了 pod 数量，修改该参数将触发集群水平扩缩容。

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2 
     componentSpecs:
     - name: broker
       componentDefRef: broker
       replicas: 2 # Change the amount
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
    terminationPolicy: Delete
   ```

2. 查看相关资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  kafka
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      kafka
    Replicas:  2
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

</Tabs>

### 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mycluster-kafka-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=ivy85
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=ivy85
   ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
