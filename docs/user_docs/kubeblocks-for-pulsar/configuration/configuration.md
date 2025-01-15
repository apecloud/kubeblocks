---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [pulsar, parameter, configuration, reconfiguration]
sidebar_position: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

There are 3 types of parameters:

1. Environment parameters, such as GC-related parameters, `PULSAR_MEM`, and `PULSAR_GC`, changes will apply to all components;
2. Configuration parameters, such as `zookeeper` or `bookies.conf` configuration files, can be changed through `env` and changes restart the pod;
3. Dynamic parameters, such as configuration files in `brokers.conf`, `broker` supports two types of change modes:
    a. Parameter change requires a restart, such as `zookeeperSessionExpiredPolicy`;
    b. For parameters that support dynamic parameters, you can obtain a list of all dynamic parameters with `pulsar-admin brokers list-dynamic-config`.

:::note

`pulsar-admin` is a management tool built in the Pulsar cluster. You can log in to the corresponding pod with `kubectl exec -it <pod-name> -- bash` (pod-name can be checked by `kubectl get pods` command, and you can choose any pod with the word `broker` in its name ), and there are corresponding commands in the `/pulsar/bin path` of the pod. For more information about pulsar-admin, please refer to the [official documentation](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/
).
:::

<Tabs>

<TabItem value="Edit config file" label="Edit config file" default>

KubeBlocks supports configuring cluster parameters by configuration file.

1. Modify the Pulsar `broker.conf` file, in this case, it is `pulsar-broker-broker-config`.

   ```bash
   kubectl edit cm pulsar-broker-broker-config -n demo
   ```

2. Check whether the configuration is done.

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar-broker -n dmo
   ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

KubeBlocks supports configuring cluster parameters with OpsRequest.

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `lostBookieRecoveryDelay` is configured as `1000`.

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: bookies
       configurations:
       - keys:
         - key: bookkeeper.conf
           parameters:
           - key: lostBookieRecoveryDelay
             value: "1000"
         name: bookies-config
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

2. Apply the configuration OpsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Verify the configuration.

   1. Check the progress of configuration:

      ```bash
      kubectl get ops -n demo
      ```

   2. Check whether the configuration is done.

      ```bash
      kubectl get pod -l app.kubernetes.io/name=pulsar -n demo
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo  
```

* View the details of the current configuration file.

  ```bash
  kbcli cluster describe-config mycluster -n demo --show-detail
  ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

## Configure parameters

### Configure parameters with configure command

#### Configure environment parameters

***Steps***

1. You need to specify the component name to configure parameters. Get the pulsar cluster component name.

   ```bash
   kbcli cluster list-components mycluster -n demo 
   >
   NAME               NAMESPACE   CLUSTER      TYPE               IMAGE
   proxy              demo        mycluster    pulsar-proxy       docker.io/apecloud/pulsar:2.11.2
   broker             demo        mycluster    pulsar-broker      docker.io/apecloud/pulsar:2.11.2
   bookies-recovery   demo        mycluster    bookies-recovery   docker.io/apecloud/pulsar:2.11.2
   bookies            demo        mycluster    bookies            docker.io/apecloud/pulsar:2.11.2
   zookeeper          demo        mycluster    zookeeper          docker.io/apecloud/pulsar:2.11.2
   ```

2. Configure parameters.

   We take `zookeeper` as an example.

   ```bash
   kbcli cluster configure mycluster -n demo --components=zookeeper --set PULSAR_MEM="-XX:MinRAMPercentage=50 -XX:MaxRAMPercentage=70" 
   ```

3. Verify the configuration.

   a. Check the progress of configuration:

   ```bash
   kubectl get ops -n demo
   ```

   b. Check whether the configuration is done.

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar -n demo
   ```

#### Configure other parameters

The following steps take the configuration of dynamic parameter `brokerShutdownTimeoutMs` as an example.

***Steps***

1. Get configuration information.

   ```bash
   kbcli cluster desc-config mycluster -n demo --components=broker
   >
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                                  COMPONENT   CLUSTER
   agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                mycluster-broker-agamotto-configuration   broker      mycluster
   broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       mycluster-broker-broker-env               broker      mycluster
   broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   mycluster-broker-broker-config            broker      mycluster
   ```

2. Configure parameters.

   ```bash
   kbcli cluster configure mycluster -n demo --components=broker --config-specs=broker-config --set brokerShutdownTimeoutMs=66600
   >
   Will updated configure file meta:
     ConfigSpec: broker-config          ConfigFile: broker.conf        ComponentName: broker        ClusterName: mycluster
   OpsRequest mycluster-reconfiguring-qxw8s created successfully, you can view the progress:
           kbcli cluster describe-ops mycluster-reconfiguring-qxw8s -n demo
   ```

3. Check the progress of configuration.

   The ops name is printed with the command above.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-qxw8s -n demo
   >
   Spec:
     Name: mycluster-reconfiguring-qxw8s        NameSpace: demo        Cluster: mycluster        Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster --components=broker --config-specs=broker-config --config-file=broker.conf --set brokerShutdownTimeoutMs=66600 --namespace=demo

   Status:
     Start Time:         Jul 20,2023 09:53 UTC+0800
     Completion Time:    Jul 20,2023 09:53 UTC+0800
     Duration:           1s
     Status:             Succeed
     Progress:           2/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool called `edit-config` to help you to configure parameter in a visualized way.

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
   kbcli cluster describe-ops mycluster-reconfiguring-nqxw8s -n demo
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   :::note

   1. When using the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config mycluster -n demo --components=zookeeper
```

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-qxw8s mycluster-reconfiguring-mwbnw
```

</TabItem>

</Tabs>
