---
title: 重启集群
description: 如何重启集群
keywords: [kafka, 重启]
sidebar_position: 4
sidebar_label: 重启
---


# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

重启后，节点角色可能会发生变化。

:::

## 步骤

1. 创建 OpsRequest，重启集群。

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
    - componentName: broker
  EOF
  ```

1. 查看 pod 和运维操作状态，验证重启操作。

   ```bash
   kubectl get pod -n demo

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。
