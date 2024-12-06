---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mongodb, parameter, configuration, reconfiguration]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

<Tabs>

<TabItem value="Edit config file" label="Edit config file" default>

KubeBlocks supports configuring cluster parameters by editing its configuration file.

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

:::note

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

KubeBlocks supports configuring cluster parameters with OpsRequest.

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

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                      COMPONENT    CLUSTER           
mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mycluster-replicaset-mongodb-config           replicaset   mycluster   
mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mycluster-replicaset-mongodb-config           replicaset   mycluster   
mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mycluster-replicaset-mongodb-metrics-config   replicaset   mycluster   

History modifications:
OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED 
```

From the meta information, the cluster `mycluster` has a configuration file named `mongodb.conf`.

You can also view the details of this configuration file and parameters.

```bash
kbcli cluster describe-config mycluster --show-detail -n demo
```

## Configure parameters

### Configure parameters with configure command

The example below configures systemLog.verbosity to 1.

1. Adjust the values of `systemLog.verbosity` to 1.

   ```bash
   kbcli cluster configure mycluster -n demo --components mongodb --config-specs mongodb-config --config-file mongodb.conf --set systemLog.verbosity=1
   >
   Warning: The parameter change you modified needs to be restarted, which may cause the cluster to be unavailable for a period of time. Do you need to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
   ConfigSpec: mongodb-config      ConfigFile: mongodb.conf      ComponentName: mongodb  ClusterName: mycluster
   OpsRequest mycluster-reconfiguring-q8ndn created successfully, you can view the progress:
          kbcli cluster describe-ops mycluster-reconfiguring-q8ndn -n default
   ```

2. Check the configuration history and view whether the configuration is successful.

   ```bash

    kbcli cluster describe-config mycluster -n demo
    >
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                   COMPONENT   CLUSTER
    mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mycluster-mongodb-mongodb-config           mongodb     mycluster
    mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mycluster-mongodb-mongodb-config           mongodb     mycluster
    mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mycluster-mongodb-mongodb-metrics-config   mongodb     mycluster

    History modifications:
    OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME   FILE           STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
    mycluster-reconfiguring-q8ndn   mycluster   mongodb     mongodb-config     mongodb.conf   Succeed   restart   3/3        Apr 21,2023 18:56 UTC+0800   {"mongodb.conf":"{\"systemLog\":{\"verbosity\":\"1\"}}"}```
   ```

3. Verify configuration result.

   ```bash
    root@mycluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
    verbosity: "1"
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you to configure parameter in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   If there are multiple components in a cluster, use `--components` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops xxx -n demo
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mycluster -n demo
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
```

If you perform several parameter configurations, you can compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-q8ndn mycluster-reconfiguring-hxqfx -n demo
```

</TabItem>

</Tabs>
