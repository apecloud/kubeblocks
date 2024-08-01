---
title: 配置集群参数
description: 如何配置集群参数
keywords: [mongodb, 参数, 配置]
sidebar_position: 1
siderbar_label: 配置
---

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

## 开始之前

1. [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
2. [创建 MongoDB 集群](./../cluster-management/create-and-connect-to-a-mongodb-cluster.md)。

## 通过编辑配置文件配置参数

1. 获取集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-mongodb -n demo
   ```

2. 按需配置参数。以下实例中添加了 `spec.configFileParams`，用于配置 `systemLog.verbosity` 参数。

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

3. 连接集群，确认配置是否生效。

      ```bash
      kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

      root@mycluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
      >
        verbosity: 1
      ```

## 通过 OpsRerquest 配置参数

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`systemLog.verbosity` 参数修改为 `1`。

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

3. 连接集群，确认配置是否生效。

   ```bash
   kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

   root@mycluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
   >
     verbosity: 1
   ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

从元信息中可以看到，集群 `mycluster` 的配置文件名称。

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
  kbcli cluster explain-config mycluster --param=systemLog.verbosity --config-specs=mongodb-config -n demo
  ```

  如果集群支持多个模板，你可以通过 `--config-specs` 来指定一个配置模板。执行 `kbcli cluster describe-config mycluster` 查看所有模板的名称。

  <details>

  <summary>输出</summary>

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

  * Allowed Values：定义了参数的有效值范围。
  * Dynamic: 决定了参数配置的生效方式。根据被修改参数的生效类型，有**动态**和**静态**两种不同的配置策略。
    * `Dynamic` 为 `true` 时，参数**动态**生效，可在线配置。
    * `Dynamic` 为 `false` 时，参数**静态**生效，需要重新启动 Pod 才能生效。
  * Description：描述了参数的定义。

:::
