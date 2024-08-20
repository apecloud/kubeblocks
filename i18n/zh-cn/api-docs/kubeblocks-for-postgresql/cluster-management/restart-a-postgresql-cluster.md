---
title: 重启集群
description: 如何重启集群
keywords: [postgresql, 重启, 重启集群]
sidebar_position: 4
sidebar_label: 重启
---


# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。重启后，主节点可能会发生变化。

## 步骤

1. 创建 OpsRequest 重启集群。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: postgresql
   EOF
   ```

2. 查看 Pod 和重启操作的状态，验证该操作是否成功。

   ```bash
   kubectl get pod -n demo
   >
   NAME                     READY   STATUS            RESTARTS   AGE
   mycluster-postgresql-0   3/4     Terminating       0          5m32s
   mycluster-postgresql-1   4/4     Running           0          6m36s

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。
