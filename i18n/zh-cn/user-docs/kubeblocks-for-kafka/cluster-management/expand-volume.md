---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [kafka, 磁盘扩容]
sidebar_position: 4
sidebar_label: 磁盘扩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 磁盘扩容

KubeBlocks 支持 Pod 磁盘存储扩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

<Tabs>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl -n demo get cluster mycluster
>
NAME           CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
mycluster      kafka                kafka-3.3.2    Delete               Running    19m
```

</TabItem>

</Tabs>

## 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volumeexpansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: broker
       volumeClaimTemplates:
       - name: data
         storage: 40Gi
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

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看对应的集群资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Volume Claim Templates:
     Name:  data
     Spec:
       Access Modes:
         ReadWriteOnce
       Resources:
         Requests:
           Storage:   40Gi
   ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 在集群的 YAML 文件中更改 `spec.components.volumeClaimTemplates.spec.resources` 的值。

   `spec.components.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发磁盘扩容。

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
     - name: kafka 
       componentDefRef: kafka
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # 修改该参数值
     terminationPolicy: Delete
   ```

2. 当集群状态再次回到 `Running` 后，查看对应的集群资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Volume Claim Templates:
     Name:  data
     Spec:
       Access Modes:
         ReadWriteOnce
       Resources:
         Requests:
           Storage:   40Gi
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 使用 `kbcli cluster volume-expand` 命令配置所需资源，然后再次输入集群名称进行磁盘扩容。

   ```bash
   kbcli cluster volume-expand mycluster -n demo --storage=40Gi --components=kafka --volume-claim-templates=data
   ```

   - `--components` 表示需扩容的组件名称。
   - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
   - `--storage` 表示磁盘需扩容至的大小。

2. 验证磁盘扩容操作是否成功。

    - 查看 OpsRequest 进程。

       执行磁盘扩容命令后，KubeBlocks 会自动输出查看 OpsRequest 进程的命令，可通过该命令查看 OpsRequest 进程的细节，包括 OpsRequest 的状态、PVC 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一进程已完成。

       ```bash
       kbcli cluster describe-ops mycluster-volumeexpansion-8257f -n demo
       ```

    - 查看集群状态。

       ```bash
       kbcli cluster list mycluster -n demo
       >
       NAME             NAMESPACE     CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
       mycluster        demo          kafka                     kafka-3.3.2              Delete                    Updating        Sep 27,2024 15:27 UTC+0800
       ```

       * STATUS=Updating 表示扩容正在进行中。
       * STATUS=Running 表示扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>
