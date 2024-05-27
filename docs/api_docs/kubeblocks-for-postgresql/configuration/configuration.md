---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [postgresql, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

This guide shows how to configure cluster parameters by creating an opsRequest.

KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g., a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process of configuring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when refreshing the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

## Before you start

1. [Install KubeBlocks](./../../installation/install-kubeblocks.md).
2. [Create a PostgreSQL cluster](./../cluster-management/create-and-connect-a-postgresql-cluster.md).

## Configure cluster parameters by editing configuration file

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-postgresql -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `max_connections`.

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: postgresql
     configItemDetails:
     - configFileParams:
         my.cnf:
           parameters:
             max_connections: "600"
       configSpec:
         constraintRef: postgresql14-cc
         defaultMode: 292
         keys:
         - postgresql.conf
         name: postgresql-configuration
         namespace: kb-system
         templateRef: postgresql-configuration
         volumeName: postgresql-config
       name: postgresql-configuration
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
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

## Configure cluster parameters with OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a YAML file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: postgresql
       configurations:
       - keys:
         - key: postgresql.conf
           parameters:
           - key: max_connections
             value: "600"
         name: postgresql-configuration
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

2. Apply this OpsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      postgres

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      tf8fhsv2
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `postgresql.conf`.

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
  kbcli cluster explain-config mycluster --param=max_connections
  ```
  
  <details>

  <summary>Output</summary>
  
  ```bash
  template meta:
    ConfigSpec: postgresql-configuration ComponentName: postgresql ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     max_connections
    Allowed Values:     [6-8388607]
    Scope:              Global
    Dynamic:            true
    Type:               integer
    Description:        Sets the maximum number of concurrent connections.
  ```

  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

:::
