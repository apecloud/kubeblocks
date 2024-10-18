---
title: PostgreSQL connection pool
description: Connect to PgBouncer to reduce too many PostgreSQL connections and to improve the throughput and stability of the database.
keywords: [postgresql, connection pool, pgbouncer]
sidebar_position: 1
sidebar_label: Connection pool
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# PostgreSQL connection pool

PostgreSQL adopts a multi-process architecture, which creates a separate backend process for each user connection. When there are too many user connections, it occupies a large amount of memory, which reduces the throughput and stability of the database. To solve these problems, KubeBlocks introduces a connection pool, PgBouncer, for PostgreSQL database clusters.

When creating a PostgreSQL cluster with KubeBlocks, PgBouncer is installed by default.

## Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. View the status of the created PostgreSQL cluster and ensure this cluster is `Running`.

   ```bash
   kbcli cluster list mycluster -n demo
   ```

2. Describe this cluster and there are two connection links in Endpoints.

    Port `5432` is used to connect to the primary pod of this database and port `6432` is used to connect to PgBouncer.

    ```bash
    kbcli cluster describe mycluster -n demo
    >
    Endpoints:
    COMPONENT    MODE        INTERNAL                                              EXTERNAL   
    postgresql   ReadWrite   mycluster-postgresql.default.svc.cluster.local:5432   <none>     
                             mycluster-postgresql.default.svc.cluster.local:6432         
    ```

3. Connect the cluster with PgBouncer.

   This command shows how to connect to a cluster with CLI. The default example uses port `5432` and you can replace it with port `6432`.

    ```bash
    kbcli cluster connect --client=cli --show-example mycluster -n demo
    >
    kubectl port-forward service/mycluster-postgresql 6432:6432
    PGPASSWORD=***** psql -h127.0.0.1 -p 6432 -U postgres postgres
    ```

4. Run `port-forward`.

   ```bash
   kubectl port-forward service/mycluster-postgresql 6432:6432
   ```

5. Open a new terminal window and run the `psql` command to connect to PgBouncer.

   ```bash
   PGPASSWORD=***** psql -h127.0.0.1 -p 6432 -U postgres postgres
   ```

6. Run the following command in `psgl` to verify the connection.

   If you can connect to `pgbouncer` and execute `show help` with the expected results below, this cluster connects to PgBouncer successfully.

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. View the status of the created PostgreSQL cluster and ensure this cluster is `Running`.

   ```bash
   kubectl get cluster mycluster -n demo
   ```

2. Describe the services and there are two connection links in Endpoints.

    Port `5432` is used to connect to the primary pod of this database and port `6432` is used to connect to PgBouncer.

    ```bash
    kubectl get services mycluster-postgresql -n demo
    >
    NAME                   TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
    mycluster-postgresql   ClusterIP   10.97.123.178   <none>        5432/TCP,6432/TCP   39m       
    ```

3. Run the command below to get the `username` and `password` for the `kubectl exec` command.

    ```bash
    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
    >
    postgres

    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
    >
    shgkz4z9
   ```

4. Connect the cluster with PgBouncer. The default example uses port `6432` and you can replace it with port `5432`.

    ```bash
    kubectl -n demo port-forward service/mycluster-postgresql 6432:6432
    ```

5. Open a new terminal window and run the `psql` command to connect to PgBouncer.

    Fill the password obtained from step 3 into the `PGPASSWORD`.

    ```bash
    PGPASSWORD=shgkz4z9 psql -h127.0.0.1 -p 6432 -U postgres postgres
    ```

6. Run the following command in `psgl` to verify the connection.

   If you can connect to `pgbouncer` and execute `show help` with the expected results below, this cluster connects to PgBouncer successfully.

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

</TabItem>

</Tabs>
