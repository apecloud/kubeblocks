---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mysql, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports both `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mysql-cluster  
```

From the meta information, the cluster `mysql-cluster` has a configuration file named `my.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mysql-cluster --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mysql-cluster | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mysql-cluster --param=innodb_buffer_pool_size --config-spec=mysql-consensusset-config
  ```

  `--config-spec` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mysql-cluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
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
  
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

## Configure parameters

### Configure parameters with configure command

The example below takes configuring `max_connection` and `innodb_buffer_pool_size` as an example.

1. View the current values of `max_connection` and `innodb_buffer_pool_size`.

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
   kbcli cluster configure mysql-cluster --set=max_connections=600,innodb_buffer_pool_size=512M
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure mysql-cluster  --set=max_connections=200,innodb_buffer_pool_size=2097152
   error: failed to validate updated config: [failed to cue template render configure: [mysqld.innodb_buffer_pool_size: invalid value 2097152 (out of bound >=5242880):
    343:34
   ]
   ]
   ```

   :::

3. Search the status of the parameter configuration.

   `Status.Progress` shows the overall status of the parameter configuration and `Conditions` show the details.

   ```bash
   kbcli cluster describe-ops mysql-cluster-reconfiguring-z2wvn -n default
   ```

   <details>

   <summary>Output</summary>

   ```bash
   Spec:
     Name: mysql-cluster-reconfiguring-z2wvn        NameSpace: default        Cluster: mysql-cluster        Type: Reconfiguring

    Command:
      kbcli cluster configure mysql-cluster --component-names=mysql --template-name=mysql-consensusset-config --config-file=my.cnf --set innodb_buffer_pool_size=512M --set max_connections=600

    Status:
      Start Time:         Mar 13,2023 02:55 UTC+0800
      Completion Time:    Mar 13,2023 02:55 UTC+0800
      Duration:           1s
      Status:             Succeed
      Progress:           1/1

    Conditions:
    LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
    Mar 13,2023 02:55 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
    Mar 13,2023 02:55 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mysql-cluster-reconfiguring-z2wvn is validated
    Mar 13,2023 02:55 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mysql-cluster, Component: mysql
    Mar 13,2023 02:55 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
    Mar 13,2023 02:55 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated policy: <autoReload>, updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
    Mar 13,2023 02:55 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
    ```

    </details>

4. Connect to the database to verify whether the parameters are configured as expected.

   The whole searching process has a 30-second delay since it takes some time for kubelet to synchronize modifications to the volume of the pod.

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

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you configure parameters in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on the notepad.

The following steps take configuring MySQL Standalone as an example.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mysql-cluster --config-spec=mysql-consensusset-config
   ```

:::note

* `--config-spec` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mysql-cluster` to view all template names.
* If there are multiple components in a cluster, use `--component` to specify a component.

:::

1. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

2. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mysql-cluster
   ```

:::note

1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
2. Deleting a parameter will be supported later.

:::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config mysql-cluster
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE     ENABLED   TEMPLATE                   CONSTRAINT                    RENDERED                                  COMPONENT   CLUSTER                
mysql-consensusset-config   my.cnf   true      mysql8.0-config-template   mysql8.0-config-constraints   mysql-cluster-mysql-mysql-config   mysql       mysql-cluster   

History modifications:
OPS-NAME                            CLUSTER         COMPONENT   CONFIG-SPEC-NAME            FILE     STATUS    POLICY   PROGRESS   CREATED-TIME                 VALID-UPDATED                                                                                                                     
mysql-cluster-reconfiguring-4q5kv   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed   reload   -/-        Mar 16,2023 15:44 UTC+0800   {"my.cnf":"{\"mysqld\":{\"max_connections\":\"3000\",\"read_buffer_size\":\"24288\"}}"}                                           
mysql-cluster-reconfiguring-cclvm   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed   reload   -/-        Mar 16,2023 17:28 UTC+0800   {"my.cnf":"{\"mysqld\":{\"innodb_buffer_pool_size\":\"1G\",\"max_connections\":\"600\"}}"}   
mysql-cluster-reconfiguring-gx58r   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed            -/-        Mar 16,2023 17:28 UTC+0800                       
```

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mysql-cluster-reconfiguring-4q5kv mysql-cluster-reconfiguring-gx58r
>
DIFF-CONFIGURE RESULT:
  ConfigFile: my.cnf    TemplateName: mysql-consensusset-config ComponentName: mysql    ClusterName: mysql-cluster       UpdateType: update      

PARAMETERNAME             MYSQL-CLUSTER-RECONFIGURING-4Q5KV   MYSQL-CLUSTER-RECONFIGURING-GX58R   
max_connections           3000                                600                                        
innodb_buffer_pool_size   128M                                1G 
```
