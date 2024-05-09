---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mongodb, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

This guide shows how to configure cluster parameters by creating an opsRequest.

KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g., a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process ofconfiguring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when refreshing the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

## Before you start

1. [Install KubeBlocks](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
2. [Create a MongoDB cluster](./../cluster-management/create-and-connect-to-a-mongodb-cluster.md#create-a-mongodb-cluster).

## Configure cluster parameters with OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace:demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: mongodb
       configurations:
       - keys:
         - key: mongodb.conf
           parameters:
           - key: max_connections
             value:"600"
         name: mongodb-config
     ttlSecondBeforeAbort: 0
     type: Reconfiguring
   EOF
   ```

   * `metadata.name` specifies the name of this OpsRequest.
   * `metadata.namespace` specifies the namespace where this cluster is created.
   * `spec.clusterName` specifies the cluster name.
   * `spec.reconfigure` specifies the configuration information. `componentName`specifies the component name of this cluster. `configurations.keys.key` specifies the configuration file. `configurations.keys.parameters` specifies the parameters you want to edit. `configurations.keys.name` specifies the configuration spec name.

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

## Configure cluster parameters by configuration file

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-mysql -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `- configFileParams` part to configure `max_connections`.

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
  kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-spec=mysql-consensusset-config -n demo
  ```

  `--config-spec` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

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







---------------------------

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mongodb-cluster
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                            COMPONENT    CLUSTER           
mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-replicaset-mongodb-metrics-config   replicaset   mongodb-cluster   

History modifications:
OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED 
```

From the meta information, the cluster `mongodb-cluster` has a configuration file named `mongodb.conf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mongodb-cluster --show-detail
   ```

## Configure parameters

### Configure parameters with configure command

The example below configures systemLog.verbosity to 1.

1. Adjust the values of `systemLog.verbosity` to 1.

   ```bash
   kbcli cluster configure mongodb-cluster --component mongodb --config-spec mongodb-config --config-file mongodb.conf --set systemLog.verbosity=1
   >
   Warning: The parameter change you modified needs to be restarted, which may cause the cluster to be unavailable for a period of time. Do you need to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
   ConfigSpec: mongodb-config      ConfigFile: mongodb.conf      ComponentName: mongodb  ClusterName: mongodb-cluster
   OpsRequest mongodb-cluster-reconfiguring-q8ndn created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-reconfiguring-q8ndn -n default
   ```

2. Check the configuration history.

   ```bash

    kbcli cluster describe-config mongodb-cluster
    >
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                         COMPONENT   CLUSTER
    mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-mongodb-mongodb-metrics-config   mongodb     mongodb-cluster

    History modifications:
    OPS-NAME                              CLUSTER           COMPONENT   CONFIG-SPEC-NAME   FILE           STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
    mongodb-cluster-reconfiguring-q8ndn   mongodb-cluster   mongodb     mongodb-config     mongodb.conf   Succeed   restart   3/3        Apr 21,2023 18:56 UTC+0800   {"mongodb.conf":"{\"systemLog\":{\"verbosity\":\"1\"}}"}```
   ```

3. Verify configuration result.

   ```bash
    root@mongodb-cluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
    verbosity: "1"
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you to configure parameter in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mongodb-cluster
   ```

   :::note

   If there are multiple components in a cluster, use `--component` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mongodb-cluster
   ```

   :::note

   1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::
