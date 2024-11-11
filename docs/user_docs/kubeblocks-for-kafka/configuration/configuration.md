---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [kafka, parameter, configuration, reconfiguration]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

KubeBlocks also supports configuring a cluster by editing the configuration file or applying an OpsRequest.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `server.properties`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster -n demo --param=log.cleanup.policy
  ```

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: broker	ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     log.cleanup.policy
    Allowed Values:     "compact","delete"
    Scope:              Global
    Dynamic:            false
    Type:               string
    Description:        The default cleanup policy for segments beyond the retention window. A comma separated list of valid policies. 
  ```
  
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. Currently, Kafka only supports static strategy, i.e. `Dynamic` is `false`. Restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

## Configure parameters

### Configure parameters with configure command

1. View the current value of `log.cleanup.policy`.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy=delete
   ```

2. Adjust the value of `log.cleanup.policy`.

   ```bash
   kbcli cluster configure mycluster -n demo --set=log.cleanup.policy=compact
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. Otherwise, the configuration may fail.

   :::

3. View the status of the parameter configuration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter configuration and Conditions show the details.

   When the `Status.Status` shows `Succeed`, the configuration is completed.

   <details>

   <summary>Output</summary>

   ```bash
   # In progress
   kbcli cluster describe-ops mycluster-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mycluster-reconfiguring-wvqns	NameSpace: default	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=broker --config-specs=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Duration:           5s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   ```bash
   # Parameter reconfiguration is completed
   kbcli cluster describe-ops mycluster-reconfiguring-wvqns -n demo
   >
   Spec:
     Name: mycluster-reconfiguring-wvqns	NameSpace: default	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=broker --config-specs=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Completion Time:    Sep 14,2023 16:28 UTC+0800
     Duration:           25s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   </details>

4. View the configuration file to verify whether the parameter is configured as expected.

   It takes about 30 seconds for the configuration to take effect because the kubelet requires some time to sync changes in the ConfigMap to the Pod's volume.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mycluster-reconfiguring-wvqns   mycluster   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        Sep 14,2023 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
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
   kbcli cluster describe-ops xxx -n default
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

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-wvqns mycluster-reconfiguring-hxqfx -n demo
>
DIFF-CONFIG RESULT:
  ConfigFile: server.properties	TemplateName: kafka-configuration-tpl	ComponentName: broker	ClusterName: mycluster	UpdateType: update

PARAMETERNAME         MYCLUSTER-RECONFIGURING-WVQNS   MYCLUSTER-RECONFIGURING-HXQFX
log.retention.hours   168                             200
```

</TabItem>

<TabItem value="Edit config file" label="Edit config file">

KubeBlocks supports configuring cluster parameters by editing its configuration file.

1. Get the configuration file of this cluster.

   ```bash
   kubectl get configurations.apps.kubeblocks.io -n demo

   kubectl edit configurations.apps.kubeblocks.io mycluster-kafka-combine -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `log.cleanup.policy`.

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: kafka
     configItemDetails:
     - configFileParams:
         server.properties:
           parameters:
             log.cleanup.policy: "compact"
       configSpec:
         constraintRef: kafka-cc
         name: kafka-configuration-tpl
         namespace: kb-system
         templateRef: kafka-configuration-tpl
         volumeName: kafka-config
       name: kafka-configuration-tpl
   ```

3. Connect to this cluster to verify whether the configuration takes effect as expected.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   ```

:::note

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

KubeBlocks supports configuring cluster parameters with OpsRequest.

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `log.cleanup.policy` is configured as `compact`.

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: kafka
       configurations:
       - keys:
         - key: server.properties
           parameters:
           - key: log.cleanup.policy
             value: "compact"
         name: kafka-configuration-tpl
     preConditionDeadlineSeconds: 0
     type: Reconfiguring
   EOF
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

3. Verify whether the configuration takes effect as expected.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   ```

:::note

Just in case you cannot find the configuration file of your cluster, you can switch to the kbcli tab to view the current configuration file of a cluster.

:::

</TabItem>

</Tabs>
