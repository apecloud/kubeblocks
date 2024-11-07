---
title: 重启集群
description: 如何重启集群
keywords: [mongodb, 重启集群]
sidebar_position: 4
sidebar_label: 重启
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

## 步骤

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 使用 `kbcli cluster restart` 命令重启集群，然后再次输入集群名称。

    ```bash
    kbcli cluster restart mycluster -n demo
    >
    OpsRequest mongodb-cluster-restart-pzsbj created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n demo
    ```

2. 查看集群状态，验证重启操作。

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME                   NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   mongodb-cluster        default          mongodb                   mongodb-5.0        Delete                    Running        Apr 26,2023 12:50 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

   您也可以使用在步骤 1 中随机生成的请求代码（本例中为 `pzsbj`）验证重启操作是否成功。

    ```bash
    kbcli cluster describe-ops mycluster-restart-pzsbj -n demo
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
     - componentName: mongodb
   EOF
   ```

2. 查看 Pod 和重启操作的状态，验证该操作是否成功。

   ```bash
   kubectl get pod -n demo
   >
   NAME                  READY   STATUS            RESTARTS   AGE
   mycluster-mongodb-0   3/4     Terminating       0          5m32s

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

</TabItem>

</Tabs>
