---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 扩缩容。

:::note

磁盘扩容会触发 Pod 按照 Learner -> Follower -> Leader 的顺序重启。重启后，主节点可能会发生变化。

:::

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

## 步骤

1. 更改配置。

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    ```bash
    kbcli cluster volume-expand mysql-cluster --components="mysql" \
    --volume-claim-templates="data" --storage="40Gi"
    ```

    - `--components` 表示需扩容的组件名称。
    - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
    - `--storage` 表示磁盘需扩容至的大小。

2. 验证扩容操作是否成功。

   ```bash
   kbcli cluster list mysql-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30          Delete                    Updating        Jan 29,2023 14:35 UTC+0800
   ```

   * STATUS=Updating 表示扩容正在进行中。
   * STATUS=Running 表示扩容已完成。

3. 检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mysql-cluster
    ```
