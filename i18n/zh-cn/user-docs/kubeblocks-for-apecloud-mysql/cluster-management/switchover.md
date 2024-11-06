---
title: 切换 MySQL 集群
description: 如何切换 MySQL 集群
keywords: [mysql, 切换集群, switchover]
sidebar_position: 6
sidebar_label: 切换
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 切换 ApeCloud MySQL 集群

数据库 switchover 是指在数据库集群中将主数据库的角色切换到备用数据库的过程，使备用数据库成为新的主数据库实例。通常在主数据库故障、维护或升级时执行 switchover 操作，以确保数据库服务的高可用性和连续性。可使用命令对 ApeCloud MySQL 集群版执行切换，KubeBlocks 将切换实例角色。

## 开始之前

* 确保集群正常运行。

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Sep 19,2024 16:01 UTC+0800
   ```

   </TabItem>

   <TabItem value="kubectl" label="kubectl">

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
   mycluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   27m
   ```

   </TabItem>

   </Tabs>

* 检查以下角色探针参数是否存在，确认是否已启用探针。

   ```bash
   kubectl get cd apecloud-mysql -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 2
       periodSeconds: 1
       timeoutSeconds: 1
   ```

## 切换集群

将 ApeCloud MySQL 集群版集群的一个 Follower 切换为 Leader，并将原 Leader 实例切换为 Follower。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

* 不指定 Leader 实例进行切换。

    ```bash
    kbcli cluster promote mycluster -n demo
    ```

* 指定一个新的 Leader 实例进行切换。

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' -n demo
    ```

* 如果有多个组件，可以使用 `--components` 参数指定一个组件。

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' --components='apecloud-mysql' -n demo
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

* 不指定 Leader 实例进行切换。

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
    type: Switchover
    switchover:
    - componentName: apecloud-mysql
      instanceName: '*'
  EOF
  ```

* 指定一个新的 Leader 实例进行切换。

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
    type: Switchover
    switchover:
    - componentName: apecloud-mysql
      instanceName: 'mycluster-mysql-2'
  EOF
  ```

</TabItem>

</Tabs>

## 验证集群切换

查看实例状态，验证切换是否成功。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list-instances -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get pods -n demo
```

</TabItem>

</Tabs>

## 处理异常情况

如果报错，请参考[异常处理](./../../handle-an-exception/handle-a-cluster-exception.md)排查问题。
