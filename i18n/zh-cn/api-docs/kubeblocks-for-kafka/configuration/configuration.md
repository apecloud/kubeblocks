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
     componentName: kafka-combine
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

## 通过 OpsRerquest 配置参数

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`log.cleanup.policy` 参数修改为 `compact`。

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

   | 字段                                                    | 定义     |
   |--------------------------------------------------------|--------------------------------|
   | `metadata.name`                                        | 定义了 Opsrequest 的名称。 |
   | `metadata.namespace`                                   | 定义了集群所在的 namespace。 |
   | `spec.clusterName`                                     | 定义了本次运维操作指向的集群名称。 |
   | `spec.reconfigure`                                     | 定义了需配置的 component 及相关配置更新内容。 |
   | `spec.reconfigure.componentName`                       | 定义了改集群的 component 名称。  |
   | `spec.configurations`                                  | 包含一系列 ConfigurationItem 对象，定义了 component 的配置模板名称、更新策略、参数键值对。 |
   | `spec.reconfigure.configurations.keys.key`             | 定义了 configuration map。 |
   | `spec.reconfigure.configurations.keys.parameters`      | 定义了单个参数文件的键值对列表。 |
   | `spec.reconfigure.configurations.keys.parameter.key`   | 代表您需要编辑的参数名称。|
   | `spec.reconfigure.configurations.keys.parameter.value` | 代表了将要更新的参数值。如果设置为 nil，Key 字段定义的参数将会被移出配置文件。  |
   | `spec.reconfigure.configurations.name`                 | 定义了配置模板名称。  |
   | `preConditionDeadlineSeconds`                          | 定义了本次 OpsRequest 中止之前，满足其启动条件的最长等待时间（单位为秒）。如果设置为 0（默认），则必须立即满足启动条件，OpsRequest 才能继续。|

2. 应用配置 OpsRequest。

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. 确认配置是否生效。

   ```bash
   kbcli cluster describe-config mycluster --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

从元信息中可以看到，集群 `mycluster` 有一个名为 `server.properties` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* 查看参数描述。

   ```bash
   kbcli cluster explain-config mycluster -n demo | head -n 20
   ```

* 查看指定参数的使用文档。

   ```bash
   kbcli cluster explain-config mycluster --param=log.cleanup.policy -n demo
   ```

   Kafka 目前支持多个模板，你可以通过 `--config-specs` 来指定一个配置模板。执行 `kbcli cluster describe-config mycluster` 查看所有模板的名称。

  <details>

  <summary>输出</summary>

  ```bash
  component: kafka-combine
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: kafka-combine	ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     log.cleanup.policy
    Allowed Values:     "compact","delete"
    Scope:              Global
    Dynamic:            false
    Type:               string
    Description:        The default cleanup policy for segments beyond the retention window. A comma separated list of valid policies.
  component: kafka-exporter
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: kafka-exporter	ClusterName: mycluster
  ```
  
  </details>

  * Allowed Values：定义了参数的有效值范围。
  * Dynamic: 决定了参数配置的生效方式。根据被修改参数的生效类型，有**动态**和**静态**两种不同的配置策略。
    * `Dynamic` 为 `true` 时，参数**动态**生效，可在线配置。
    * `Dynamic` 为 `false` 时，参数**静态**生效，需要重新启动 Pod 才能生效。
  * Description：描述了参数的定义。

:::
