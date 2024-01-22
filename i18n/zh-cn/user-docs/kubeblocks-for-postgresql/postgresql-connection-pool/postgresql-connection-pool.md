---
title: PostgreSQL 连接池
description: 通过连接 PgBouncer，减少过多的 PostgreSQL 连接，提高数据库的吞吐量和稳定性。
keywords: [postgresql, 连接池, pgbouncer]
sidebar_position: 1
sidebar_label: 连接池
---

# PostgreSQL 连接池

PostgreSQL 是多进程架构，它会为每个用户连接创建一个单独的后端进程，当用户连接数较多时，会占用大量内存，降低数据库的吞吐量和稳定性。为解决 PostgreSQL 连接过多而导致的问题，KubeBlocks 为 PostgreSQL 集群引入了连接池，PgBouncer。

使用 KubeBlocks 创建 PostgreSQL 集群时，会默认安装 PgBouncer。

## 步骤

1. 查看 PostgreSQL 集群的状态，确保为 `Running`。

   ```bash
   kbcli cluster list mycluster
   ```

2. 查看 PostgreSQL 集群详细信息，其中会包含两条连接信息。

    其中 `5432` 端口用于直接连接数据库主节点，`6432` 端口用于连接 PgBouncer。

    ```bash
    kbcli cluster describe mycluster
    >
    Endpoints:
    COMPONENT    MODE        INTERNAL                                              EXTERNAL   
    postgresql   ReadWrite   mycluster-postgresql.default.svc.cluster.local:5432   <none>     
                             mycluster-postgresql.default.svc.cluster.local:6432         
    ```

3. 通过 PgBouncer 连接集群。

   该命令会展示访问集群的方式，默认使用 `5432` 端口访问集群，可以将端口替换为 `6432` 通过 PgBouncer 访问集群。

    ```bash
    kbcli cluster connect --client=cli --show-example mycluster
    >
    kubectl port-forward service/mycluster-postgresql 6432:6432
    PGPASSWORD=***** psql -h127.0.0.1 -p 6432 -U postgres postgres
    ```

4. 执行 `port-forward` 命令。

   ```bash
   kubectl port-forward service/mycluster-postgresql 6432:6432
   ```

5. 在另一个终端窗口中执行 `psql` 命令进行连接。

   ```bash
   PGPASSWORD=***** psql -h127.0.0.1 -p 6432 -U postgres postgres
   ```

6. 在 `psql` 中执行如下命令进行验证。

   如果可以连接至 `pgbouncer` 库并正确执行 `show help` 命令，说明已经成功连接 PgBouncer。

   ```bash
   postgres=# \c pgbouncer
   ```

   ```bash
   pgbouncer=# show help;
   >
   NOTICE:  Console usage
   DETAIL:  
           SHOW HELP|CONFIG|DATABASES|POOLS|CLIENTS|SERVERS|USERS|VERSION
           SHOW PEERS|PEER_POOLS
           SHOW FDS|SOCKETS|ACTIVE_SOCKETS|LISTS|MEM|STATE
           SHOW DNS_HOSTS|DNS_ZONES
           SHOW STATS|STATS_TOTALS|STATS_AVERAGES|TOTALS
           SET key = arg
           RELOAD
           PAUSE [<db>]
           RESUME [<db>]
           DISABLE <db>
           ENABLE <db>
           RECONNECT [<db>]
           KILL <db>
           SUSPEND
           SHUTDOWN
           WAIT_CLOSE [<db>]
   SHOW
   ```
