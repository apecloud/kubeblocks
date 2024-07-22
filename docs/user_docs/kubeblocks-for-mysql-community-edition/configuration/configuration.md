---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mysql, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

# Configure cluster parameters

This guide shows how to configure cluster parameters.

From v0.9.0, KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g. a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects it and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process of configuring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when updating the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster  
```

From the meta information, the cluster `mycluster` has a configuration file named `my.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-specs=mysql-replication-config
  ```

  `--config-specs` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
  component: mysql
  template meta:
    ConfigSpec: mysql-replication-config	ComponentName: mysql	ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     innodb_buffer_pool_size
    Allowed Values:     [5242880-18446744073709552000]
    Scope:              Global
    Dynamic:            true
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

The example below takes configuring `max_connections` and `innodb_buffer_pool_size` as an example.

1. View the current values of `max_connections` and `innodb_buffer_pool_size`.

   ```bash
   kbcli cluster connect mycluster
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
   kbcli cluster configure mycluster --set=max_connections=600,innodb_buffer_pool_size=512M
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure mycluster  --set=max_connections=200,innodb_buffer_pool_size=2097152
   error: failed to validate updated config: [failed to cue template render configure: [mysqld.innodb_buffer_pool_size: invalid value 2097152 (out of bound >=5242880):
    343:34
   ]
   ]
   ```

   :::

3. Search the status of the parameter configuration.

   `Status.Progress` shows the overall status of the parameter configuration and `Conditions` show the details.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-pxs46 -n default
   ```

   <details>

   <summary>Output</summary>

   ```bash
   Spec:
   Name: mycluster-reconfiguring-pxs46	NameSpace: default	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster --components=mysql --config-specs=mysql-replication-config --config-file=my.cnf --set innodb_buffer_pool_size=512M --set max_connections=600 --namespace=default

   Status:
     Start Time:         Jul 05,2024 19:00 UTC+0800
     Completion Time:    Jul 05,2024 19:00 UTC+0800
     Duration:           2s
     Status:             Succeed
     Progress:           2/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Jul 05,2024 19:00 UTC+0800   WaitForProgressing   WaitForProgressing                True     wait for the controller to process the OpsRequest: mycluster-reconfiguring-pxs46 in Cluster: mycluster
   Jul 05,2024 19:00 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mycluster-reconfiguring-pxs46 is validated
   Jul 05,2024 19:00 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mycluster, Component: mysql
   Jul 05,2024 19:00 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mycluster-reconfiguring-pxs46 in Cluster: mycluster

   Warning Events: <none>
   ```

   </details>

4. Connect to the database to verify whether the parameters are configured as expected.

   The whole searching process has a 30-second delay since it takes some time for kubelet to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect mycluster
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

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mycluster --config-specs=mysql-replication-config
   ```

   :::note

   * Since MySQL currently supports multiple templates, it is required to use `--config-specs` to specify a configuration template. You can run `kbcli cluster describe-config mycluster` to view all template names.
   * If there are multiple components in a cluster, use `--components` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mycluster
   ```

   :::note

   1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config mycluster
>
component: mysql

ConfigSpecs Meta:
CONFIG-SPEC-NAME           FILE                              ENABLED   TEMPLATE                          CONSTRAINT                           RENDERED                                   COMPONENT   CLUSTER
mysql-replication-config   my.cnf                            true      oracle-mysql8.0-config-template   oracle-mysql8.0-config-constraints   mycluster-mysql-mysql-replication-config   mysql       mycluster
agamotto-configuration     agamotto-config.yaml              false     mysql-agamotto-configuration                                           mycluster-mysql-agamotto-configuration     mysql       mycluster
agamotto-configuration     agamotto-config-with-proxy.yaml   false     mysql-agamotto-configuration                                           mycluster-mysql-agamotto-configuration     mysql       mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME           FILE     STATUS    POLICY              PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-pxs46   mycluster   mysql       mysql-replication-config   my.cnf   Succeed   syncDynamicReload   2/2        Jul 05,2024 19:00 UTC+0800   {"my.cnf":"{\"mysqld\":{\"innodb_buffer_pool_size\":\"512M\",\"max_connections\":\"600\"}}"}
mycluster-reconfiguring-x52fb   mycluster   mysql       mysql-replication-config   my.cnf   Succeed   syncDynamicReload   2/2        Jul 05,2024 19:04 UTC+0800   {"my.cnf":"{\"mysqld\":{\"max_connections\":\"1000\"}}"}                    
```

From the above results, there are two parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-pxs46 mycluster-reconfiguring-x9zsf
>
DIFF-CONFIGURE RESULT:
  ConfigFile: my.cnf    TemplateName: mysql-replication-config ComponentName: mysql    ClusterName: mycluster       UpdateType: update      

PARAMETERNAME             mycluster-reconfiguring-pxs46   mycluster-reconfiguring-x9zsf   
max_connections           600                             2000                                      
innodb_buffer_pool_size   512M                            1G 
```
