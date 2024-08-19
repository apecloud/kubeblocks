---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 存储磁盘扩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list pulsar
```

## 步骤

1. 更改配置。

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    :::note

    请先扩展 `journal` 卷，然后再扩展 `Ledger` 卷。

    :::

      - 扩展 `journal` 卷。

        ```bash
        kbcli cluster volume-expand pulsar --storage=40Gi --components=bookies -t journal  
        ```

        - `--components` 表示需扩容的组件名称。
        - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
        - `--storage` 表示磁盘需扩容至的大小。

      - 扩展 `ledger` 卷。

        ```bash
        kbcli cluster volume-expand pulsar --storage=200Gi --components=bookies -t ledgers  
        ```

2. 验证扩容操作是否成功。

   ```bash
   kubectl get ops  
   ```

   * STATUS=Updating 表示扩容正在进行中。
   * STATUS=Running 表示扩容已完成。

3. 检查资源规格是否已变更。

    ```bash
    kbcli cluster describe pulsar
    ```
