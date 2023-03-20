---
title: Configure cluster parameters
description: Configure cluster parameters
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter change interface to facilitate managing parameter reconfiguration, searching the parameter user guide, and validating parameter effectiveness.

## Before you start

1. Install KubeBlocks. For details, refer to [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md). 
2. Create a MySQL standalone and wait until the cluster status is Running.

## View the parameter information

1. Run the command below to search for parameter information.
   
   ```bash
   kbcli cluster explain-config mysql-cluster |head -n 20
   >
   template meta:
   ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mysql-cluster

   Parameter Explain:
   +----------------------------------------------------------+--------------------------------------------+--------+---------+---------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | PARAMETER NAME                                           | ALLOWED VALUES                             | SCOPE  | DYNAMIC | TYPE    | DESCRIPTION                                                                                                                                                                                                                                                                                                                                                                      |
   +----------------------------------------------------------  +--------------------------------------------+--------+---------+---------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | activate_all_roles_on_login                              | "0","1","OFF", "ON"                         | Global | false   | string  | Automatically set all granted roles as active after the user has authenticated successfully.                                                                                                                                                                                                                                                                                    |
   | allow-suspicious-udfs                                    | "0","1","OFF","ON"                         | Global | false   | string  | Controls whether user-defined functions that have only an xxx symbol for the main function can be loaded                                                                                                                                                                                                                                                                         |
   | auto_generate_certs                                      | "0","1","OFF","ON"                         | Global | false   | string  | Controls whether the server autogenerates SSL key and certificate files in the data directory, if they do not already exist.                                                                                                                                                                                                                                                     |
   | auto_increment_increment                                 | [1-65535]                                  | Global | false   | integer | Intended for use with master-to-master replication, and can be used to control the operation of AUTO_INCREMENT columns                                                                                                                                                                                                                                                           |
   | auto_increment_offset                                    | [1-65535]                                  | Global | false   | integer | Determines the starting point for the AUTO_INCREMENT column value                                                                                                                                                                                                                                                                                                                |
   | autocommit                                               | "0","1","OFF","ON"                         | Global | false   | string  | Sets the autocommit mode                                                                                                                                                                                                                                                                                                                                                         |
   | automatic_sp_privileges                                  | "0","1","OFF","ON"                         | Global | false   | string  | When this variable has a value of 1 (the default), the server automatically grants the EXECUTE and ALTER ROUTINE privileges to the creator of a stored routine, if the user cannot already execute and alter or drop the routine.                                                                                                                                                |
   | avoid_temporal_upgrade                                   | "0","1","OFF","ON"                         | Global | false   | string  | This variable controls whether ALTER TABLE implicitly upgrades temporal columns found to be in pre-5.6.4 format.                                                                                                                                                                                                                                                                 |
   | back_log                                                 | [1-65535]                                  | Global | false   | integer | The number of outstanding connection requests MySQL can have                                                                                                                                                                                                                                                                                                                     |
   | basedir                                                  |                                            | Global | false   | string  | The MySQL installation base directory.                                                                                                                                                                                                                                                                                                                                           |
   | big_tables                                               | "0","1","OFF","ON"                         | Global | false   | string  |                                                                                                                                                                                                                                                                                                                                                                                  |
   | bind_address                                             |                                            | Global | false   | string  |                                                                                                                                                                                                                                                                                                                                                                                  |
   | binlog_cache_size                                        | [4096-18446744073709548000]                | Global | false   | integer | The size of the cache to hold the SQL statements for the binary log during a transaction.
   ```

2. View the user guide of a parameter.
   ```bash
   kbcli cluster explain-config mysql-cluster --param=innodb_buffer_pool_size
   template meta:
      ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mysql-cluster

   Configure Constraint:
     Parameter Name:     innodb_buffer_pool_size
     Allowed Values:     [5242880-18446744073709552000]
     Scope:              Global
     Dynamic:            false
     Type:               integer
     Description:        The size in bytes of the memory buffer innodb uses to cache data and indexes of its tables  
    ```
    * Allowed Values: It defines the valid value of this parameter.
    * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter reconfiguration takes effect. As mentioned in [How KubeBlocks configuration works](#how-kubeblocks-configuration-works), there are two different reconfiguration strategies based on the effectiveness type of changed parameters, i.e. **dynamic** and **static**. 
      * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and you can follow the instructions in [Reconfigure dynamic parameters](#reconfigure-dynamic-parameters).
      * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and you can follow the instructions in [Reconfigure static parameters](#reconfigure-static-parameters).
    * Description: It describes the parameter definition.


## Reconfigure dynamic parameters

Here we take reconfiguring `max_connection` and `beb_buffer_pool_size` as an example.

1. Run the command to view the current values of `max_connection` and `beb_buffer_pool_size`.
   ```bash
   kbcli cluster connect mysql-cluster
   ```

   ```bash
   mysql> show variables like '%max_connections%';
   >
   +-----------------+-------+
   | Variable_name   | Value |
   +-----------------+-------+
   | max_connections | 167   |
   +-----------------+-------+
   1 row in set (0.04 sec)
   ```

   ```bash
   mysql> show variables like '%innodb_buffer_pool_size%';
   >
   +-------------------------+-----------+
   | Variable_name           | Value     |
   +-------------------------+-----------+
   | innodb_buffer_pool_size | 134217728 |
   +-------------------------+-----------+
   1 row in set (0.00 sec)
   ```

2. Adjust the values of `max_connections` and `innodb_buffer_pool_size`.
   ```bash
   kbcli cluster configure rose15  --set=max_connections=600,innodb_buffer_pool_size=512M
   ```
   
   :::note

   Make sure the value you set is within the Allowed Values of this parameters. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure mysql-cluster  --set=max_connections=200,innodb_buffer_pool_size=2097152
   error: failed to validate updated config: [failed to cue template render configure: [mysqld.innodb_buffer_pool_size: invalid value 2097152 (out of bound >=5242880):
    343:34
   ]
   ]
   ```

   :::

3. Search the status of the parameter reconfiguration.
   `Status.Progress` shows the overall status of the parameter change and `Conditions` show the details.

   ```bash
   kbcli cluster describe-ops mysql-cluster-reconfiguring-z2wvn
   >
   Spec:
     Name: mysql-cluster-reconfiguring-z2wvn        NameSpace: default        Cluster: mysql-cluster        Type: Reconfiguring

    Command:
      kbcli cluster configure mysql-cluster --component-names=mysql --template-name=mysql-consensusset-config --config-file=my.cnf --set innodb_buffer_pool_size=512M --set max_connections=600

    Status:
      Start Time:         Mar 13,2023 02:55 UTC+0800
      Completion Time:    Mar 13,2023 02:55 UTC+0800
      Duration:           1s
      Status:             Succeed
      Progress:           -/-

    Conditions:
    LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
    Mar 13,2023 02:55 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
    Mar 13,2023 02:55 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mysql-cluster-reconfiguring-z2wvn is validated
    Mar 13,2023 02:55 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mysql-cluster, Component: mysql
    Mar 13,2023 02:55 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
    Mar 13,2023 02:55 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated policy: <autoReload>, updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
    Mar 13,2023 02:55 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
    ```

4. Verify whether the parameters are modified. 
   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize changes to the volume of the pod.

   ```bash
   kbcli cluster connect mysql-cluster
   ```

   ```bash
   mysql> show variables like '%max_connections%';
   >
   +-----------------+-------+
   | Variable_name   | Value |
   +-----------------+-------+
   | max_connections | 600   |
   +-----------------+-------+
   1 row in set (0.04 sec)
   ```
  
   ```bash
   mysql> show variables like '%innodb_buffer_pool_size%';
   >
   +-------------------------+-----------+
   | Variable_name           | Value     |
   +-------------------------+-----------+
   | innodb_buffer_pool_size | 536870912 |
   +-------------------------+-----------+
   1 row in set (0.00 sec)
   ```

## Reconfigure static parameters

Static parameter reconfiguring requires restarting the pod. Here we take reconfiguring `ngram_token_size` as an example.

1. Search the current value of `ngram_token_size` and the default value is 2.
   ```bash
   kbcli cluster explain-config mysql-cluster --param=ngram_token_size
   >
   template meta:
     ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mysql-cluster

   Configure Constraint:
     Parameter Name:     ngram_token_size
     Allowed Values:     [1-10]
     Scope:              Global
     Dynamic:            false
     Type:               integer
     Description:        Defines the n-gram token size for the n-gram full-text parser.
    ```

    ```bash
    kbcli cluster connect mysql-cluster
    ```

    ```bash
    mysql> show variables like '%ngram_token_size%';
    >
    +------------------+-------+
    | Variable_name    | Value |
    +------------------+-------+
    | ngram_token_size | 2     |
    +------------------+-------+
    1 row in set (0.01 sec)
    ```

2. Adjust the value of `ngram_token_size`.
   ```bash
   kbcli cluster configure mysql-cluster  --set=ngram_token_size=6
   >
   Will updated configure file meta:
     TemplateName: mysql-consensusset-config          ConfigureFile: my.cnf        ComponentName: mysql        ClusterName: mysql-cluster
   OpsRequest mysql-cluster-reconfiguring-nrnpf created
   ```
   
   :::note

   Make sure the value you set is within the Allowed Values of this parameters. Otherwise, the configuration may fail.

   :::

3. Watch the progress of searching parameter reconfiguration and pay attention to the output of `Status.Progress` and `Status.Status`.
   ```bash
   # In progress
   kbcli cluster describe-ops mysql-cluster-reconfiguring-nrnpf
   >
   Spec:
     Name: mysql-cluster-reconfiguring-nrnpf        NameSpace: default        Cluster: mysql-cluster        Type: Reconfiguring

   Command:
     kbcli cluster configure mysql-cluster --component-names=mysql --template-name=mysql-consensusset-config --config-file=my.cnf --set ngram_token_size=6

   Status:
     Start Time:         Mar 13,2023 03:37 UTC+0800
     Duration:           22s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   ```bash
   # Parameter change is completed
   kbcli cluster describe-ops mysql-cluster-reconfiguring-nrnpf
   >
   Spec:
     Name: mysql-cluster-reconfiguring-nrnpf        NameSpace: default        Cluster: mysql-cluster        Type: Reconfiguring

   Command:
     kbcli cluster configure mysql-cluster --component-names=mysql --template-name=mysql-consensusset-config --config-file=my.cnf --set ngram_token_size=6

   Status:
     Start Time:         Mar 13,2023 03:37 UTC+0800
     Completion Time:    Mar 13,2023 03:37 UTC+0800
     Duration:           26s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

4. After the reconfiguration is completed, connect to the database and verify the changes.
   ```bash
   kbcli cluster connect mysql-cluster

   Copyright (c) 2000, 2022, Oracle and/or its affiliates.

   Oracle is a registered trademark of Oracle Corporation and/or its
   affiliates. Other names may be trademarks of their respective
   owners.

   Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.
   ```

   ```bash
   mysql> show variables like '%ngram_token_size%';
   >
   +------------------+-------+
   | Variable_name    | Value |
   +------------------+-------+
   | ngram_token_size | 6     |
   +------------------+-------+
   1 row in set (0.09 sec)
   ```