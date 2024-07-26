---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [apecloud mysql, 磁盘扩容]
sidebar_position: 3
sidebar_label: 磁盘扩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 磁盘扩容

KubeBlocks 支持 Pod 扩缩容。

:::note

磁盘扩容会触发 Pod 重启。重启后，主节点可能会发生变化。

:::

## 开始之前

确保集群处于 `Running` 状态，否则后续操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   4m29s
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
     name: ops-volume-expansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: mysql
       volumeClaimTemplates:
       - name: data
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

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. 更改集群 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的值。

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发磁盘扩容。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 3
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # Change the volume storage size.
     terminationPolicy: Delete
   ```

2. 查看对应的集群资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>
