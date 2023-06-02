---
title: Use ZEngine with MySQL
description: Use ZEngine with MySQL
keywords: [zengine, use zengine]
sidebar_position: 2
sidebar_label: Use ZEngine with MySQL
---

# Use ZEngine with MySQL

ZEngine coexists with other storage engines when enabled. However, please note that enabling ZEngine will reduce the bufferpool of InnoDB to 128 MB, which significantly affects the performance of InnoDB tables.

## Enable ZEngine

***Steps:***

1. Use the ```kbcli cluster configure``` command to enable ZEngine.

   ```bash
   kbcli cluster configure {cluster-name} --set Zengine_enabled=ON 
   ```

2. Check the ZEngine status.

   ```
   kbcli cluster describe-config {cluster-name} --show-detail | grep ZEngine_enabled
   ```

After the ZEngine is enabled, the default storage engine is ZEngine, that is to say, the newly created table is stored with ZEngine. If you want to change the default storage engine you can specify it in `CREATE` command line. See the command line below.

```bash
CREATE TABLE t1 (
c1 int primary key,
c2 varchar(64)
) engine=ZEngine;

# SHOW CREATE TABLE t1 
+-------+-------------------------------------------------------------
| Table | Create Table                                                
+-------+-------------------------------------------------------------
| t1    | CREATE TABLE `t1` (
  `c1` int NOT NULL,
  `c2` varchar(64) COLLATE utf8mb4_general_ci DEFAULT NULL,
  PRIMARY KEY (`c1`)
) ENGINE=ZEngine DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci |
+-------+-------------------------------------------------------------
```

For the tables created before enabling ZEngine, the storage engine will not change. To change the engine to ZEngine, you can use the `ALTER` command.

```bash
ALTER TABLE {table_name} engine=ZEngine;
```

## Disable ZEngine

Use the ```kbcli cluster configure``` command to disable ZEngine.

```bash
kbcli cluster configure {cluster-name} --set Zengine_enabled=OFF 
```

:::note

- Disabling ZEngine restarts the cluster.

- Once ZEngine is disabled, the configuration of the cluster changes to default. And  tables using ZEngine as storage backend is not accessible. To access these tables, you need to enable ZEngine again, or change the storage backend of these tables to other storage backend, such as InnoDB.

:::
