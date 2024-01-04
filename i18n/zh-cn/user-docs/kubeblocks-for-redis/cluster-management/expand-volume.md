---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [redis, 磁盘扩容]
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 扩缩容。

:::note

磁盘扩容将触发 Pod 并行重启。重启后，主节点可能会发生变化。

:::

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list <name>
```

***示例***

```bash
kbcli cluster list redis-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
redis-cluster        default          redis                     redis-7.0.6            Delete                    Running        Apr 10,2023 19:00 UTC+0800
```

## 步骤

1. 更改配置，共有 3 种方式。

   **选项 1**.（**推荐**） 使用 kbcli

   配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

   ```bash
   kbcli cluster volume-expand redis-cluster --components="redis" \
   --volume-claim-templates="data" --storage="2Gi"
   ```

   - `--components` 表示需扩容的组件名称。
   - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
   - `--storage` 表示磁盘需扩容至的大小。

   **选项 2**. 创建 OpsRequest

   根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volume-expansion
   spec:
     clusterRef: redis-cluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: redis
       volumeClaimTemplates:
       - name: data
         storage: "2Gi"
   EOF
   ```

   **选项 3**. 更改集群的 YAML 文件

   在集群的 YAML 文件中更改 `spec.components.volumeClaimTemplates.spec.resources` 的值。
   
   `spec.components.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发磁盘扩容。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: redis-cluster
     namespace: default
   spec:
     clusterDefinitionRef: redis
     clusterVersionRef: redis-7.0.6
     componentSpecs:
     - componentDefRef: redis
       name: redis
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # 修改磁盘容量
     terminationPolicy: Delete
   ```

2. 验证扩容操作是否成功。

   ```bash
   kbcli cluster list <name>
   ```

   ***示例***

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    VolumeExpanding        Apr 10,2023 16:27 UTC+0800
   ```

   - STATUS=VolumeExpanding 表示扩容正在进行中。
   - STATUS=Running 表示扩容已完成。
