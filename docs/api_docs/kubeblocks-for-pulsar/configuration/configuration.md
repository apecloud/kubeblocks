---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [pulsar, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

# Configure cluster parameters

This guide shows how to configure cluster parameters.

For Pulsar, there are 3 types of parameters:

1. Environment parameters, such as GC-related parameters, `PULSAR_MEM`, and `PULSAR_GC`, changes will apply to all components;
2. Configuration parameters, such as `zookeeper` or `bookies.conf` configuration files, can be changed through `env` and changes restart the pod;
3. Dynamic parameters, such as configuration files in `brokers.conf`, `broker` supports two types of change modes:
    a. Parameter change requires a restart, such as `zookeeperSessionExpiredPolicy`;
    b. For parameters that support dynamic parameters, you can obtain a list of all dynamic parameters with `pulsar-admin brokers list-dynamic-config`.

:::note

`pulsar-admin` is a management tool built in the Pulsar cluster. You can log in to the corresponding pod with `kubectl exec -it <pod-name> -- bash` (pod-name can be checked by `kubectl get pods` command, and you can choose any pod with the word `broker` in its name ), and there are corresponding commands in the `/pulsar/bin path` of the pod. For more information about pulsar-admin, please refer to the [official documentation](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/
).
:::

## Before you start

1. [Install KubeBlocks](../../../user_docs/installation/install-with-helm/install-kubeblocks.md).
2. [Create a Pulsar cluster](./../cluster-management/create-pulsar-cluster-on-kubeblocks.md).

## Configure cluster parameters by configuration file

Using kubectl to configure pulsar cluster requires modifying the configuration file.

1. Modify the Pulsar `broker.conf` file, in this case, it is `pulsar-broker-broker-config`.

   ```bash
   kubectl edit cm pulsar-broker-broker-config -n demo
   ```

2. Check whether the configuration is done.

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar-broker
   ```

## Configure cluster parameters with OpsRequest

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

2. Apply the configuration opsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Verify the configuration.

   1. Check the progress of configuration:

      ```bash
      kubectl get ops
      ```

   2. Check whether the configuration is done.

      ```bash
      kubectl get pod -l app.kubernetes.io/name=pulsar
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `broker.conf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

:::
