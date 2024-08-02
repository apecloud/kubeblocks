---
title: 高可用
description: PostgreSQL 高可用
keywords: [postgresql, 高可用]
sidebar_position: 1
---

# 高可用

KubeBlocks 集成[开源的 Patroni 方案](https://patroni.readthedocs.io/en/latest/)以实现高可用性，主要采用 Noop 切换策略。

## 开始之前

* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* [创建 PostgreSQL 主备版](./../cluster-management/create-and-connect-a-postgresql-cluster.md#create-a-postgresql-cluster)。
* 检查角色探测参数，验证角色探测是否已启用。

    ```bash
    kubectl get cd postgresql -o yaml
    >
    probes:
      roleProbe:
        failureThreshold: 2
        periodSeconds: 1
        timeoutSeconds: 1
    ```

## 步骤

1. 查看 PostgreSQL 集群的初始状态。

   ```bash
   kubectl get cluster mycluster -n demo

   kubectl -n demo get pod -L kubeblocks.io/role
   ```

   ![PostgreSQL 集群原始状态](./../../../img/api-ha-pg-original-status.png)

   Currently, `mycluster-postgresql-0` is the primary pod and `mycluster-postgresql-1` is the secondary pod.
   当前 `mycluster-postgresql-0` 是主节点，`mycluster-postgresql-1` 是从节点。

2. 模拟主节点异常。

   ```bash
   # 进入主节点
   kubectl exec -it mycluster-postgresql-0 -n demo -- bash

   # 删除 PostgreSQL 的数据目录，模拟异常
   root@mycluster-postgresql-0:/home/postgres# rm -fr /home/postgres/pgdata/pgroot/data
   ```

3. 查看日志，检查发生异常情况时节点角色的切换情况。

   ```bash
   # 查看主节点日志
   kubectl logs mycluster-postgresql-0 -n demo
   ```

   在日志中可以看到，主节点释放了 Leader 锁并进行了高可用切换。

   ```bash
   2024-05-17 02:41:23,523 INFO: Lock owner: mycluster-postgresql-0; I am mycluster-postgresql-0
   2024-05-17 02:41:23,702 INFO: Leader key released
   2024-05-17 02:41:23,904 INFO: released leader key voluntarily as data dir empty and currently leader
   2024-05-17 02:41:23,905 INFO: Lock owner: mycluster-postgresql-1; I am mycluster-postgresql-0
   2024-05-17 02:41:23,906 INFO: trying to bootstrap from leader 'mycluster-postgresql-1'
   ```

   ```bash
   # 查看从节点日志
   kubectl logs mycluster-postgresql-1 -n demo
   ```

   原来的从节点获取了锁并成为了新的主节点。

   ```bash
   2024-05-17 02:41:35,806 INFO: no action. I am (mycluster-postgresql-1), the leader with the lock
   2024-05-17 02:41:45,804 INFO: no action. I am (mycluster-postgresql-1), the leader with the lock
   ```

4. 连接到 PostgreSQL 集群，查看集群信息。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   postgres

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   shgkz4z9

   kubectl exec -ti -n demo mycluster-postgresql-1 -- bash

   root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: shgkz4z9
   ```

   ```bash
   postgres=# select * from pg_stat_replication;
   ```

   ![PostgreSQL 集群信息](./../../../img/api-ha-pg-replication-info.png)

   从输出可以看到，`mycluster-postgresql-0` 被指定为从节点。

5. 查看集群，检查实例角色。

   ```bash
   kubectl get cluster mycluster -n demo

   kubectl -n demo get pod -L kubeblocks.io/role
   ```

   ![PostgreSQL 高可用切换后集群状态](./../../../img/api-ha-pg-after.png)

   故障切换后，`mycluster-postgresql-0` 变成了从节点，`mycluster-postgresql-1` 变成了主节点。
