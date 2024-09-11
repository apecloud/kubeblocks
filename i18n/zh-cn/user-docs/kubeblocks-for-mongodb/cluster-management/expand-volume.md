---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [mongodb, 磁盘扩容]
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 存储磁盘扩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list mongodb-cluster
```

## 步骤

1. 更改配置。

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    ```bash

    kbcli cluster volume-expand --storage=30Gi --components=mongodb --volume-claim-templates=data mongodb-cluster

    ```

    - `--components` 表示需扩容的组件名称。
    - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
    - `--storage` 表示磁盘需扩容至的大小。

2. 验证扩容操作是否成功。

   ```bash
   kbcli cluster list mongodb-cluster
   >
   NAME                   NAMESPACE        CLUSTER-DEFINITION        VERSION              TERMINATION-POLICY        STATUS          CREATED-TIME
   mongodb-cluster        default          mongodb                   mongodb-5.0          Delete                    Updating        Jan 29,2023 14:35 UTC+0800
   ```

   * STATUS=Updating 表示扩容正在进行中。
   * STATUS=Running 表示扩容已完成。

3. 检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mongodb-cluster
    ```
