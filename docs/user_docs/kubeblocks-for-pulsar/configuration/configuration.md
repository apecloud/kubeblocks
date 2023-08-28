---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [pulsar, parameter, configuration, reconfiguration]
sidebar_position: 4
---

# Configure cluster parameters

There are 3 types of parameters:

1. Environment parameters, such as GC-related parameters, `PULSAR_MEM`, and `PULSAR_GC`, changes will apply to all components;
2. Configuration parameters, such as `zookeeper` or `bookies.conf` configuration files, can be changed through `env` and changes restart the pod;
3. Dynamic parameters, such as configuration files in `brokers.conf`, `broker` supports two types of change modes:
    a. Parameter change requires a restart, such as `zookeeperSessionExpiredPolicy`;
    b. For parameters that support dynamic parameters, you can obtain a list of all dynamic parameters with `pulsar-admin brokers list-dynamic-config`.

:::note

`pulsar-admin` is a management tool built in the Pulsar cluster. You can log in to the corresponding pod with `kubectl exec -it <pod-name> -- bash` (pod-name can be checked by `kubectl get pods` command, and you can choose any pod with the word `broker` in its name ), and there are corresponding commands in the `/pulsar/bin path` of the pod. For more information about pulsar-admin, please refer to the [official documentation](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/
)
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
  kbcli cluster explain-config pulsar |head -n 20
  ```

## Reconfigure environment parameters

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

   b.Check whether the configuration is done.

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar
   ```

## Reconfigure dynamic parameters

The following steps take the reconfiguration of dynamic parameter `brokerShutdownTimeoutMs` as an example.

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

2. Reconfigure parameters.

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

## Reconfigure static parameters

Static parameter reconfiguring requires restarting the pod. The following example reconfigures `lostBookieRecoveryDelay`.

1. Get the current configuration.

    ```bash
    kbcli cluster explain-config pulsar --component=broker
    >
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                               COMPONENT   CLUSTER
    agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                pulsar-broker-agamotto-configuration   broker      pulsar
    broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       pulsar-broker-broker-env               broker      pulsar
    broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   pulsar-broker-broker-config            broker      pulsar
    ```

2. Adjust the value of `lostBookieRecoveryDelay`.

   ```bash
   kbcli cluster configure pulsar --component=broker --config-spec=broker-config --set lostBookieRecoveryDelay=1000
   ```

   :::note

   The change of parameters may cause the restart of the cluster. Enter `yes` to confirm it. 

   :::

   ***Example***

   ```bash
   kbcli cluster configure pulsar --component=broker --config-spec=broker-config --set lostBookieRecoveryDelay=1000
   >
   Warning: The parameter change incurs a cluster restart, which brings the cluster down for a while. Enter to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
     ConfigSpec: broker-config          ConfigFile: broker.conf        ComponentName: broker        ClusterName: pulsar
   OpsRequest pulsar-reconfiguring-gmz7w created successfully, you can view the progress:
           kbcli cluster describe-ops pulsar-reconfiguring-gmz7w -n default
   ```

3. View the status of the parameter reconfiguration.

   ```bash
   kbcli cluster describe-ops pulsar-reconfiguring-gmz7w -n default
   >
   Spec:
     Name: pulsar-reconfiguring-gmz7w        NameSpace: default        Cluster: pulsar        Type: Reconfiguring

   Command:
     kbcli cluster configure pulsar --components=broker --config-spec=broker-config --config-file=broker.conf --set lostBookieRecoveryDelay=1000 --namespace=default

   Status:
     Start Time:         Jul 20,2023 09:57 UTC+0800
     Duration:           57s
     Status:             Running
     Progress:           1/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

## Reconfigure with kubectl

Using kubectl to reconfigure pulsar cluster requires modifying the configuration file.

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

    In the rendered colume of the above output, you can check the broker's configmap is `pulsar-broker-broker-config`.

2. Modify the `broker.conf` file, in this case, it is `pulsar-broker-broker-config`.

    ```bash
    kubectl edit cm pulsar-broker-broker-config
    ```
