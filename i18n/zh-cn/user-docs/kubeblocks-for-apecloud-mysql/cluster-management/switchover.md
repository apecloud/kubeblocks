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

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
   mycluster   apecloud-mysql       Delete               Running   45m
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        apecloud-mysql       Delete               Running   Jan 20,2025 16:27 UTC+0800
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

<TabItem value="kubectl" label="kubectl" default>

* 不指定 Leader 实例进行切换。

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-switchover
     namespace: demo
   spec:
     # 定义了本次运维操作指向的集群名称
     clusterName: mycluster
     type: Switchover
     # 列出切换的对象，指定执行本次切换任务面向的 Component
     switchover:
       # 定义了 Component 的名称
     - componentName: mysql
       # 定义了本次切换任务将哪个实例切换为主实例。`instanceName` 可设置为以下任一值：
       # - "*" （通配符）: 表示不指定主实例
       # - 某一实例的名称（即 pod 名称）
       instanceName: '*'
   EOF
   ```

* 指定一个新的 Leader 实例进行切换。

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-switchover-specify
     namespace: demo
   spec:
     # 定义了本次运维操作指向的集群名称
     clusterName: mycluster
     type: Switchover
     # 列出切换的对象，指定执行本次切换任务面向的 Component
     switchover:
       # 定义了 Component 的名称
     - componentName: mysql
       # 定义了本次切换任务将哪个实例切换为主实例。`instanceName` 可设置为以下任一值：
       # - "*" （通配符）: 表示不指定主实例
       # - 某一实例的名称（即 pod 名称）
       instanceName: acmysql-cluster-mysql-2
   EOF
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

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

</Tabs>

## 验证集群切换

查看实例状态，验证切换是否成功。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get pods -n demo
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list-instances -n demo
```

</TabItem>

</Tabs>

## 处理异常情况

如果报错，请参考[异常处理](./../../handle-an-exception/handle-a-cluster-exception.md)排查问题。
