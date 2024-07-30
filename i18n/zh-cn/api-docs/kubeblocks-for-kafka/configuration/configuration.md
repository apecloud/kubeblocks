---
title: 配置集群参数
description: 如何配置集群参数
keywords: [kafka, 参数, 配置, 再配置]
sidebar_position: 1
sidebar_label: 配置
---

# 配置集群参数

本教程演示了如何配置集群参数。

## 开始之前

1. [安装 KubeBlocks](./.././../installation/install-kubeblocks.md).
2. [创建 Kafka 集群](./../cluster-management/create-a-kafka-cluster.md).

## 通过编辑配置文件配置参数

1. 获取集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-kafka-combine -n demo
   ```

2. 按需配置参数。以下实例中添加了 `spec.configFileParams`，用于配置 `log.cleanup.policy` 参数。

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
     - configSpec:
         defaultMode: 292
   ```

3. 确认配置是否生效。

   ```bash
   kbcli cluster describe-config mycluster --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mycluster-reconfiguring-wvqns   mycluster   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        May 10,2024 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
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

From the meta information, the cluster `mycluster` has a configuration file named `kafka.conf`.

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
  kbcli cluster explain-config mykafka --param=log.cleanup.policy
  ```

  `--config-specs` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: kafka-configuration-tpl   ComponentName: broker   ClusterName: mykafka

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
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

:::
