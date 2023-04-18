# Use SmartEngine with MySQL

SmartEngine coexists with other storage engines when enabled. However, please note that enabling SmartEngine will reduce the bufferpool of InnoDB to 128 MB, which significantly affects the performance of InnoDB tables.

## Enable  SmartEngine

***Steps:***

1. Use the ```kbcli cluster configure``` command to enable SmartEngine.

```bash
kbcli cluster configure {cluster-name} --set smartengine_enabled=ON 
```

2. Check the SmartEngine status.

```
kbcli cluster describe-config {cluster-name} --show-detail | grep ZEngine_enabled
```

After the SmartEngine is enabled, the default storage engine is SmartEngine, that is to say, the newly created table is stored with SmartEngine. If you want to change the default storage engine you can specify it in ```CREATE`` command line. See the command line below.

```
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

For the tables created before enbaling SmartEngine, the storage engine will not change. To change the engine to SmartEngine, you can use the ```ALTER``` command.

```
ALTER TABLE {table_name} engine=ZEngine;
```

## Disable SmartEngine

Use the ```kbcli cluster configure``` command to disable SmartEngine.

```bash
kbcli cluster configure {cluster-name} --set smartengine_enabled=OFF 
```

... note

- Disabling SmartEngine restarts the cluster.

- Once SmartEngine is disabled, the configuration of the cluster changes to default. And  tables using SmartEngine as storage backend is not accessible. To access these tables, you need to enable SmartEngine again, or change the storage backend of these tables to other storage backend, such as InnoDB.
...
