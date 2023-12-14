---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [postgresql, 磁盘扩容]
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
kbcli cluster list pg-cluster
>
NAME              NAMESPACE        CLUSTER-DEFINITION    VERSION                  TERMINATION-POLICY        STATUS         CREATED-TIME
pg-cluster        default          postgresql            postgresql-14.7.0        Delete                    Running        Mar 3,2023 10:29 UTC+0800
```

## 步骤

1. 更改配置。共有 3 种方式。

    **选项 1.** （**推荐**） 使用 kbcli

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    ```bash
    kbcli cluster volume-expand pg-cluster --components="pg-replication" \
    --volume-claim-templates="data" --storage="2Gi"
    ```

    - `--components` 表示需扩容的组件名称。
    - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
    - `--storage` 表示磁盘需扩容至的大小。

    **选项 2.** 创建 OpsRequest

    执行以下命令，扩展集群容量。

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
    spec:
      clusterRef: pg-cluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: pg-replication
        volumeClaimTemplates:
        - name: data
          storage: "2Gi"
    EOF
    ```

    **选项 3.** 更改集群的 YAML 文件

    在集群的 YAML 文件中更改 `spec.components.volumeClaimTemplates.spec.resources` 的值。
    
    `spec.components.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发磁盘扩容。

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: pg-cluster
      namespace: default
    spec:
      clusterDefinitionRef: postgresql
      clusterVersionRef: postgresql-14.7.0
      componentSpecs:
      - name: pg-replication
        componentDefRef: postgresql
        replicas: 1
        volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi # 修改磁盘容量
      terminationPolicy: Halt
    ```

2. 验证扩容操作是否成功。

   ```bash
   kbcli cluster list <name>
   ```

   ***示例***

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME              NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   pg-cluster        default          postgresql                postgresql-14.7.0        Delete                    VolumeExpanding        Apr 10,2023 16:27 UTC+0800
   ```

   * STATUS=VolumeExpanding 表示扩容正在进行中。
   * STATUS=Running 表示扩容已完成。
