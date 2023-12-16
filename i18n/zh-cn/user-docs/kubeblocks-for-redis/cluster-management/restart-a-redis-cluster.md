---
title: 重启集群
description: 如何重启集群
keywords: [redis, 集群重启]
sidebar_position: 4
sidebar_label: 重启
---

# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

重启集群将触发 Pod 并行重启。重启后，主节点可能会发生变化。

:::

## 步骤

1. 重启集群。

   你可以通过使用 `kbcli` 或创建 OpsRequest 来重启集群。
  
   **选项 1.**（**推荐**）使用 kbcli

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart redis-cluster --components="redis" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

   **选项 2.** 创建 OpsRequest

   重启集群：

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterRef: redis-cluster
     type: Restart 
     restart:
     - componentName: redis
   EOF
   ```

2. 验证重启操作。
   
   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list <name>
   ```

   - STATUS=Restarting 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

   ***示例***

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.x        Delete                    Running        Apr 10,2023 19:20 UTC+0800
   ```
