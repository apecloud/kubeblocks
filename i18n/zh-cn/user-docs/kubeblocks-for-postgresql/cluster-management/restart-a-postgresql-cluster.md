---
title: 重启集群
description: 如何重启集群
keywords: [postgresql, 重启]
sidebar_position: 4
sidebar_label: 重启
---


# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

重启后，主节点可能会发生变化。

:::

## 步骤

1. 重启集群。

   配置 `--components` 和 `--ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart NAME --components="postgresql" \
   --ttlSecondsAfterSucceed=30
   ```

   - `--components` 表示需要重启的组件名称。
   - `--ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list <name>
   ```

   ***示例***

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME         NAMESPACE   CLUSTER-DEFINITION          VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   pg-cluster   default     postgresql-cluster          postgresql-14.7.0   Delete               Running   Mar 03,2023 18:28 UTC+0800
   ```

   * STATUS=Updating 表示集群正在重启中。
   * STATUS=Running 表示集群已重启。
