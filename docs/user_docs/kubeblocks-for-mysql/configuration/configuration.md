---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mysql, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter reconfiguration, searching the parameter user guide, and validating parameter effectiveness.

## Before you start

1. [Install KubeBlocks](./../../installation/introduction.md): Choose one guide that fits your actual environments.
2. [Create a MySQL cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md#create-a-mysql-cluster) and wait until the cluster status is Running.

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
  kbcli cluster explain-config mysql-cluster |head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mysql-cluster --param=innodb_buffer_pool_size
  ```

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
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter reconfiguration takes effect. There are two different reconfiguration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be updated online. Follow the instructions in [Reconfigure dynamic parameters](#reconfigure-dynamic-parameters).
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make reconfiguration effective. Follow the instructions in [Reconfigure static parameters](#reconfigure-static-parameters).
  * Description: It describes the parameter definition.

## Reconfigure dynamic parameters

The example below reconfigures `max_connection` and `innodb_buffer_pool_size`.

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

3. Search the status of the parameter reconfiguration.

   `Status.Progress` shows the overall status of the parameter reconfiguration and `Conditions` show the details.

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

4. Connect to the database to verify whether the parameters are modified.

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

## Reconfigure static parameters

Static parameter reconfiguring requires restarting the pod. The following example reconfigures `ngram_token_size`.

1. Search the current value of `ngram_token_size` and the default value is 2.

    ```bash
    kbcli cluster explain-config mysql-cluster --param=ngram_token_size
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
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. Otherwise, the reconfiguration may fail.

   :::

3. View the status of the parameter reconfiguration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter reconfiguration and Conditions show the details.

   When the `Status.Status` shows `Succeed`, the reconfiguration is completed.

   <details>

   <summary>Output</summary>

   ```bash
   # In progress
   kbcli cluster describe-ops mysql-cluster-reconfiguring-nrnpf -n default
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
   # Parameter reconfiguration is completed
   kbcli cluster describe-ops mysql-cluster-reconfiguring-nrnpf -n default
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

   </details>

4. Connect to the database to verify whether the parameters are modified.

   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect mysql-cluster
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

## View history and compare differences

After the reconfiguration is completed, you can search the reconfiguration history and compare the parameter differences.

View the parameter reconfiguration history.

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
