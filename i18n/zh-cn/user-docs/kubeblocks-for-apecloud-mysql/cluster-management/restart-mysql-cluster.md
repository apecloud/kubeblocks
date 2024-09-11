---
title: 重启集群
description: 如何重启集群
keywords: [mysql, 重启, 集群重启]
sidebar_position: 4
sidebar_label: 重启
---

# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

所有 Pod 按照 Learner -> Follower -> Leader 的顺序重启。集群重启后，主节点可能会发生变化。

:::

## 步骤

1. 重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart <name> --components="mysql" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list mysql-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
   mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。
