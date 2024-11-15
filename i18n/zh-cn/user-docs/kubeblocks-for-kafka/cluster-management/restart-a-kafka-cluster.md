---
title: 重启集群
description: 如何重启集群
keywords: [kafka, 重启]
sidebar_position: 5
sidebar_label: 重启
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

重启集群将触发 Pod 并行重启。重启后，主节点可能会发生变化。

:::

## 步骤

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

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

2. 查看 pod 和运维操作状态，验证重启操作。

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

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart mycluster -n demo --components="kafka" \
     --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令，检查集群状态，验证重启操作。

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME    CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
   kafka   kafka                kafka-3.3.2    Delete               Running    19m
   ```

   * STATUS=Updating 表示集群正在重启中。
   * STATUS=Running 表示集群已重启。

</TabItem>

</Tabs>
