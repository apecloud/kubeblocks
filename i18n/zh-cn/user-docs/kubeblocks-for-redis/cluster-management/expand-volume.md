---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [redis, 磁盘扩容]
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 磁盘存储扩容。

:::note

磁盘扩容将触发 Pod 重启。重启后，主节点可能会发生变化。

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

1. 更改配置。

   配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

   ```bash
   kbcli cluster volume-expand redis-cluster --components="redis" \
   --volume-claim-templates="data" --storage="2Gi"
   ```

   - `--components` 表示需扩容的组件名称。
   - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
   - `--storage` 表示磁盘需扩容至的大小。

2. 验证扩容操作是否成功。

   ```bash
   kbcli cluster list <name>
   ```

   ***示例***

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    Updating        Apr 10,2023 16:27 UTC+0800
   ```

   - STATUS=Updating 表示扩容正在进行中。
   - STATUS=Running 表示扩容已完成。
