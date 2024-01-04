---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [pulsar, parameter, configuration, reconfiguration]
sidebar_position: 4
---

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

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config pulsar  
```

* View the details of the current configuration file.

  ```bash
  kbcli cluster describe-config pulsar --show-detail
  ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config pulsar | head -n 20
  ```

## Configure parameters

### Configure parameters with configure command

#### Configure environment parameters

***Steps***

1. You need to specify the component name to configure parameters. Get the pulsar cluster component name.

  ```bash
  kbcli cluster list-components pulsar 
  ```

  ***Example***

  ```bash
  kbcli cluster list-components pulsar 

  NAME               NAMESPACE   CLUSTER   TYPE               IMAGE
  proxy              default     pulsar    pulsar-proxy       docker.io/apecloud/pulsar:2.11.2
  broker             default     pulsar    pulsar-broker      docker.io/apecloud/pulsar:2.11.2
  bookies-recovery   default     pulsar    bookies-recovery   docker.io/apecloud/pulsar:2.11.2
  bookies            default     pulsar    bookies            docker.io/apecloud/pulsar:2.11.2
  zookeeper          default     pulsar    zookeeper          docker.io/apecloud/pulsar:2.11.2
  ```

2. Configure parameters.

   We take `zookeeper` as an example.

   ```bash
   kbcli cluster configure pulsar --component=zookeeper --set PULSAR_MEM="-XX:MinRAMPercentage=50 -XX:MaxRAMPercentage=70" 
   ```

3. Verify the configuration.

   a. Check the progress of configuration:

   ```bash
   kubectl get ops 
   ```

   b. Check whether the configuration is done.

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar
   ```

#### Configure other parameters

The following steps take the configuration of dynamic parameter `brokerShutdownTimeoutMs` as an example.

***Steps***

1. Get configuration information.

   ```bash
   kbcli cluster desc-config pulsar --component=broker
   
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                               COMPONENT   CLUSTER
   agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                pulsar-broker-agamotto-configuration   broker      pulsar
   broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       pulsar-broker-broker-env               broker      pulsar
   broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   pulsar-broker-broker-config            broker      pulsar
   ```

2. Configure parameters.

   ```bash
   kbcli cluster configure pulsar --component=broker --config-spec=broker-config --set brokerShutdownTimeoutMs=66600
   >
   Will updated configure file meta:
     ConfigSpec: broker-config          ConfigFile: broker.conf        ComponentName: broker        ClusterName: pulsar
   OpsRequest pulsar-reconfiguring-qxw8s created successfully, you can view the progress:
           kbcli cluster describe-ops pulsar-reconfiguring-qxw8s -n default
   ```

3. Check the progress of configuration.

   The ops name is printed with the command above.

   ```bash
   kbcli cluster describe-ops pulsar-reconfiguring-qxw8s -n default
   >
   Spec:
     Name: pulsar-reconfiguring-qxw8s        NameSpace: default        Cluster: pulsar        Type: Reconfiguring

   Command:
     kbcli cluster configure pulsar --components=broker --config-spec=broker-config --config-file=broker.conf --set brokerShutdownTimeoutMs=66600 --namespace=default

   Status:
     Start Time:         Jul 20,2023 09:53 UTC+0800
     Completion Time:    Jul 20,2023 09:53 UTC+0800
     Duration:           1s
     Status:             Succeed
     Progress:           2/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you to configure parameter in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config pulsar
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
   kbcli cluster connect pulsar
   ```

   :::note

   1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::

### Configure parameters with kubectl

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
