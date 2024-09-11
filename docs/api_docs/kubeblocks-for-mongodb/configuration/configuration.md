---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mongodb, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

This guide shows how to configure cluster parameters.

## Before you start

1. [Install KubeBlocks](../../../user_docs/installation/install-with-helm/install-kubeblocks.md).
2. [Create a MongoDB cluster](./../cluster-management/create-and-connect-to-a-mongodb-cluster.md).

## Configure cluster parameters by configuration file

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-mongodb -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `systemLog.verbosity`.

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: mongodb
     configItemDetails:
     - configFileParams:
         mongodb.cnf:
           parameters:
             systemLog.verbosity: "1"
       configSpec:
         constraintRef: mongodb-config-constraints
         name: mongodb-configuration
         namespace: kb-system
         templateRef: mongodb5.0-config-template
         volumeName: mongodb-config
       name: mongodb-config
     - configSpec:
         defaultMode: 292
   ```

3. Connect to this cluster to verify whether the configuration takes effect as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

      root@mycluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
      >
        verbosity: 1
      ```

## Configure cluster parameters with OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `systemLog.verbosity` is configured as `1`.

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: mongodb
       configurations:
       - keys:
         - key: mongodb.conf
           parameters:
           - key: systemLog.verbosity
             value: "1"
         name: mongodb-config
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

3. Connect to this cluster to verify whether the configuration takes effect as expected.

   ```bash
   kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

   root@mycluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
   >
     verbosity: 1
   ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `mongodb.conf`.

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
  kbcli cluster explain-config mycluster --param=systemLog.verbosity --config-specs=mongodb-config -n demo
  ```

  `--config-specs` is required to specify a configuration template if there are multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: mongodb-config        ComponentName: mongodb        ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     systemLog.verbosity
    Allowed Values:     [0-5]
    Scope:              Global
    Dynamic:            false
    Type:               integer
    Description:          
  ```
  
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

:::
