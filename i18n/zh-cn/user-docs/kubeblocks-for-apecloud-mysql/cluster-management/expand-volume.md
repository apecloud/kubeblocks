---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
sidebar_position: 3
sidebar_label: 磁盘扩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 磁盘扩容

KubeBlocks 支持 Pod 存储磁盘扩容。

:::note

磁盘扩容会触发 Pod 重启。重启后，主节点可能会发生变化。

:::

## 开始之前

确保集群处于 `Running` 状态，否则后续操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       Delete               Running   3m40s
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        apecloud-mysql       Delete               Running   Jan 20,2025 16:27 UTC+0800
```

</TabItem>

</Tabs>

## 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-volumeexpansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: mysql
       volumeClaimTemplates:
       - name: data
         storage: 30Gi
   EOF
   ```

2. 可通过以下任意一种方式验证磁盘扩容操作是否完成。

   ```bash
   kubectl get ops -n demo
   >
   NAME                      TYPE              CLUSTER     STATUS    PROGRESS   AGE
   acmysql-volumeexpansion   VolumeExpansion   mycluster   Succeed   1/1        3m8s
   ```

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

3. 当 Ops 状态为 `Succeed`或集群状态再次回到 `Running` 后，查看对应的集群资源是否按需变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 更改集群 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` 的值。

   `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` 定义了 Pod 的存储资源信息，更改此值会触发磁盘扩容。

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   更改 `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` 的值。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
   ...
   spec:
     componentSpecs:
       - name: mysql
         volumeClaimTemplates:
           - name: data
             spec:
               storageClassName: "<you-preferred-sc>"
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 30Gi  # 指定新的磁盘容量，确保该数值大于当前值
   ```

2. 当集群状态再次回到 `Running` 后，查看对应的集群资源是否按需变更。

   ```bash
   kubectl get cluster mycluster -n demo

   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    ```bash
    kbcli cluster volume-expand mycluster --components="mysql" --volume-claim-templates="data" --storage="40Gi" -n demo
    ```

    - `--components` 表示需扩容的组件名称。
    - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
    - `--storage` 表示磁盘需扩容至的大小。

2. 可通过以下任意一种方式验证扩容操作是否完成。

   - 查看 OpsRequest 进度。

      执行磁盘扩容命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、PVC 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

      ```bash
      kbcli cluster describe-ops mycluster-volumeexpansion-8257f -n demo
      ```

   - 查看集群状态。

      ```bash
      kbcli cluster list mycluster -n demo
      >
      NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS     CREATED-TIME
      mycluster   demo        apecloud-mysql       Delete               Updating   Jan 20,2025 16:27 UTC+0800
      ```

     * STATUS=Updating 表示扩容正在进行中。
     * STATUS=Running 表示扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已按要求变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>
