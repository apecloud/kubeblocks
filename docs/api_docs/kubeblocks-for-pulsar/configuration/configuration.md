---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [pulsar, parameter, configuration, reconfiguration]
sidebar_position: 4
---

# Configure cluster parameters

This guide shows how to configure cluster parameters by creating an opsRequest.

KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g., a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process ofconfiguring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when refreshing the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

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

1. [Install KubeBlocks](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
2. [Create a Kafka cluster](./../cluster-management/create-a-kafka-cluster.md).

## Configure cluster parameters by configuration file

Using kubectl to configure pulsar cluster requires modifying the configuration file.

***Steps***

1. Get the configmap where the configuration file is located. Take `broker` component as an example.

    ```bash
    kbcli cluster desc-config pulsar --component=broker

    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                               COMPONENT   CLUSTER
    agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                pulsar-broker-agamotto-configuration   broker      pulsar
    broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       pulsar-broker-broker-env               broker      pulsar
    broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   pulsar-broker-broker-config            broker      pulsar
    ```

    In the rendered column of the above output, you can check the broker's configmap is `pulsar-broker-broker-config`.

2. Modify the `broker.conf` file, in this case, it is `pulsar-broker-broker-config`.

    ```bash
    kubectl edit cm pulsar-broker-broker-config
    ```

## Configure cluster parameters with OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

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
         - key: kafka.conf
           parameters:
           - key: log.cleanup.policy
             value: "compact"
         name: kafka-config
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

3. Connect to this cluster to verify whether the configuration takes effect as expected.

   ```bash
   kbcli cluster describe-config mykafka --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mykafka-reconfiguring-wvqns   mykafka   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        May 10,2024 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
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
