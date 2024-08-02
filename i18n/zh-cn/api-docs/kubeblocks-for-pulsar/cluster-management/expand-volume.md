---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
sidebar_position: 3
sidebar_label: 磁盘扩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 磁盘扩容

您可以调整 Pod 磁盘容量。

## 开始之前

确保集群处于 `Running` 状态，否则后续操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
```

## 步骤

当前支持通过以下两种方式扩容磁盘。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volume-expand
     namespace: demo
   spec:
     clusterRef: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: bookies
       volumeClaimTemplates:
       - name: ledgers
         storage: "200Gi"
       - name: journal
         storage: "40Gi"      
   EOF
   ```

2. 验证磁盘扩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
   ```

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

3. 查看对应的集群资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 更改集群 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的值。`spec.componentSpecs.volumeClaimTemplates.spec.resources` 定义了 Pod 的存储资源信息，更改此值会触发磁盘扩容。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: pulsar
     clusterVersionRef: pulsar-3.0.2
     componentSpecs:
     - name: pulsar
       componentDefRef: pulsar
       replicas: 1
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # 修改磁盘容量
     terminationPolicy: Delete
   ```

2. 查看对应的集群资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>
