---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mysql, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

This guide shows how to configure cluster parameters.

From v0.9.0, KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g. a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects it and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process of configuring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when updating the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## View parameter information

View the details of a cluster ConfigSpecs, including config-spec name, configure file, template, and more.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `my.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-specs=mysql-consensusset-config -n demo
  ```

  `--config-specs` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates.

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mycluster

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

The example below takes configuring `max_connections` and `innodb_buffer_pool_size` as an example.

1. View the current values of `max_connections` and `innodb_buffer_pool_size`.

   ```bash
   kbcli cluster connect mycluster -n demo
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
   kbcli cluster configure mycluster -n demo --set=max_connections=600,innodb_buffer_pool_size=512M
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure mycluster  --set=max_connections=200,innodb_buffer_pool_size=2097152 -n demo
   error: failed to validate updated config: [failed to cue template render configure: [mysqld.innodb_buffer_pool_size: invalid value 2097152 (out of bound >=5242880):
    343:34
   ]
   ]
   ```

   :::

3. Search the status of the parameter configuration.

   `Status.Progress` shows the overall status of the parameter configuration and `Conditions` show the details.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-zk4b4 -n demo
   ```

   <details>

   <summary>Output</summary>

   ```bash
   Spec:
     Name: mycluster-reconfiguring-zk4b4	NameSpace: demo	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster --components=mysql --config-spec=mysql-consensusset-config --config-file=my.cnf --set innodb_buffer_pool_size=512M --set max_connections=600 --namespace=demo

   Status:
     Start Time:         Sep 19,2024 18:26 UTC+0800
     Completion Time:    Sep 19,2024 18:26 UTC+0800
     Duration:           3s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Sep 19,2024 18:19 UTC+0800   WaitForProgressing   WaitForProgressing                True     wait for the controller to process the OpsRequest: mycluster-reconfiguring-zk4b4 in Cluster: mycluster
   Sep 19,2024 18:19 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mycluster-reconfiguring-zk4b4 is validated
   Sep 19,2024 18:19 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mycluster, Component: mysql
   Sep 19,2024 18:19 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mycluster-reconfiguring-zk4b4 in Cluster: mycluster

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

The following steps take configuring MySQL Standalone as an example.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mycluster --config-spec=mysql-consensusset-config -n demo
   ```

   :::note

   * Since ApeCloud MySQL currently supports multiple templates, it is required to use `--config-spec` to specify a configuration template. You can run `kbcli cluster describe-config mycluster` to view all template names.
   * If there are multiple components in a cluster, use `--components` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-djdfc -n demo
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
kbcli cluster describe-config mycluster -n demo
>
component: mysql

ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE           ENABLED   TEMPLATE                   CONSTRAINT                                RENDERED                                    COMPONENT   CLUSTER
mysql-consensusset-config   my.cnf         true      mysql8.0-config-template   mysql8.0-config-constraints               mycluster-mysql-mysql-consensusset-config   mysql       mycluster
vttablet-config             vttablet.cnf   true      vttablet-config-template   mysql-scale-vttablet-config-constraints   mycluster-mysql-vttablet-config             mysql       mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME            FILE     STATUS    POLICY              PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-zk4b4   mycluster   mysql       mysql-consensusset-config   my.cnf   Succeed   syncDynamicReload   1/1        Sep 19,2024 18:26 UTC+0800   {"my.cnf":"{\"mysqld\":{\"innodb_buffer_pool_size\":\"512M\",\"max_connections\":\"600\"}}"}
mycluster-reconfiguring-djdfc   mycluster   mysql       mysql-consensusset-config   my.cnf   Succeed   syncDynamicReload   1/1        Sep 19,2024 18:31 UTC+0800   {"my.cnf":"{\"mysqld\":{\"max_connections\":\"610\"}}"}                     
```

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-zk4b4 mycluster-reconfiguring-djdfc -n demo
>
DIFF-CONFIGURE RESULT:
  ConfigFile: my.cnf    TemplateName: mysql-consensusset-config ComponentName: mysql    ClusterName: mycluster       UpdateType: update      

PARAMETERNAME             MYCLUSTER-RECONFIGURING-ZK4B4   MYCLUSTER-RECONFIGURING-DJDFC  
max_connections           600                             610                                        
innodb_buffer_pool_size   512M                            512M
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

## Configure cluster parameters by editing configuration file

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-mysql -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `max_connections`.

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: mysql
     configItemDetails:
     - configFileParams:
         my.cnf:
           parameters:
             max_connections: "600"
       configSpec:
         constraintRef: mysql8.0-config-constraints
         name: mysql-consensusset-config
         namespace: kb-system
         templateRef: mysql8.0-config-template
         volumeName: mysql-config
       name: mysql-consensusset-config
     - configSpec:
         defaultMode: 292
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      root

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      2gvztbvz
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-mysql-0 -- bash

      mysql -uroot -p2gvztbvz
      >
      mysql> show variables like 'max_connections';
      +-----------------+-------+
      | Variable_name   | Value |
      +-----------------+-------+
      | max_connections | 600   |
      +-----------------+-------+
      1 row in set (0.00 sec)
      ```

## Configure cluster parameters with an OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a YAML file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: mysql
       configurations:
       - keys:
         - key: my.cnf
           parameters:
           - key: max_connections
             value: "600"
         name: mysql-consensusset-configuration
     preConditionDeadlineSeconds: 0
     type: Reconfiguring
   ```

   | Field                                                  | Definition     |
   |--------------------------------------------------------|--------------------------------|
   | `metadata.name`                                        | It specifies the name of this OpsRequest. |
   | `metadata.namespace`                                   | It specifies the namespace where this cluster is created. |
   | `spec.clusterName`                                     | It specifies the cluster name that this operation is targeted at. |
   | `spec.reconfigure`                                     | It specifies a component and its configuration updates. |
   | `spec.reconfigure.componentName`                       | It specifies the component name of this cluster.  |
   | `spec.configurations`                                  | It contains a list of ConfigurationItem objects, specifying the component's configuration template name, upgrade policy, and parameter key-value pairs to be updated. |
   | `spec.reconfigure.configurations.keys.key`             | It specifies the configuration map. |
   | `spec.reconfigure.configurations.keys.parameters`      | It defines a list of key-value pairs for a single configuration file. |
   | `spec.reconfigure.configurations.keys.parameter.key`   | It represents the name of the parameter you want to edit. |
   | `spec.reconfigure.configurations.keys.parameter.value` | It represents the parameter values that are to be updated. If set to nil, the parameter defined by the Key field will be removed from the configuration file.  |
   | `spec.reconfigure.configurations.name`                 | It specifies the configuration template name.  |
   | `preConditionDeadlineSeconds`                          | It specifies the maximum number of seconds this OpsRequest will wait for its start conditions to be met before aborting. If set to 0 (default), the start conditions must be met immediately for the OpsRequest to proceed. |

2. Apply the configuration opsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      root

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      2gvztbvz
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-mysql-0 -- bash

      mysql -uroot -p2gvztbvz
      >
      mysql> show variables like 'max_connections';
      +-----------------+-------+
      | Variable_name   | Value |
      +-----------------+-------+
      | max_connections | 600   |
      +-----------------+-------+
      1 row in set (0.00 sec)
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `my.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* View the parameter description.

   ```bash
   kbcli cluster explain-config mycluster -n demo | head -n 20
   ```

* View the user guide of a specified parameter.
  
   ```bash
   kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-specs=mysql-consensusset-config -n demo
   ```

   `--config-specs` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mycluster

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

:::

</TabItem>

</Tabs>