---
title: 配置集群参数
description: 配置集群参数
keywords: [mysql, 参数, 配置]
sidebar_position: 1
sidebar_label: 配置
---

# 配置集群参数

本文档演示了如何配置集群参数。

KubeBlocks 支持动态配置。当数据库实例的规格发生变化时（例如对实例进行升降配操作），KubeBlocks 会根据新的规格自动匹配适用的参数模板，因为不同规格的数据库实例可能需要不同的最佳参数配置以优化性能和资源利用率。当您选择不同的数据库实例规格时，KubeBlocks 会自动检测并确定适用于新规格的最佳数据库参数配置，以确保数据库在新规格下具有最优的性能和配置。

动态配置功能简化了配置的过程。您无需手动修改数据库参数，KubeBlocks 会自动处理参数的更新和配置，以适应新的规格。这样可以节省时间和精力，并减少由于参数设置不正确而导致的性能问题。

但需要注意的是，数据库参数自动刷新功能并不适用于所有参数。有些参数可能需要手动进行调整和配置。此外，如果您之前曾手动修改了数据库参数，KubeBlocks 在刷新数据库参数模板时可能会覆盖您的修改。因此，在使用动态配置功能时，建议先备份和记录自定义的参数设置，以便在需要时进行恢复。

## 开始之前

1. [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
2. [创建 MySQL 集群](./../cluster-management/create-and-connect-a-mysql-cluster.md)。

## 通过编辑配置文件配置参数

1. 获取集群的配置文件。

   ```bash
   kubectl get configurations.apps.kubeblocks.io

   kubectl edit configurations.apps.kubeblocks.io mycluster-mysql -n demo
   ```

2. 按需配置参数。以下实例中添加了 `spec.configFileParams`，用于配置 `max_connections` 参数。

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: mysql
     configItemDetails:
     - configFileParams:
         my.cnf:
           parameters:
             max_connections: "600"
       configSpec:
         constraintRef: oracle-mysql8.0-config-constraints
         name: mysql-replication-config
         namespace: kb-system
         templateRef: oracle-mysql8.0-config-template
         volumeName: mysql-config
       name: mysql-replication-config
     - configSpec:
         defaultMode: 292
   ```

3. 连接集群，确认配置是否生效。

   1. 获取用户名和密码。

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      root

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      2gvztbvz
      ```

   2. 连接集群，验证参数是否按预期配置。

      ```bash
      kubectl exec -ti -n demo mycluster-mysql-0 -- bash

      mysql -uroot -p2gvztbvz
      >
      mysql> show variables like 'max_connections';
      +-----------------+-------+
      | Variable_name   | Value |
      +-----------------+-------+
      | max_connections | 600   |
      +-----------------+-------+
      1 row in set (0.00 sec)
      ```

## 通过 OpsRerquest 配置参数

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`max_connections` 参数修改为 `600`。

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: mysql
       configurations:
       - keys:
         - key: my.cnf
           parameters:
           - key: max_connections
             value: "600"
         name: mysql-replication-configuration
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

3. 连接集群，确认配置是否生效。

   1. 获取用户名和密码。

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      root

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      2gvztbvz
      ```

   2. 连接集群，验证参数是否按预期配置。

      ```bash
      kubectl exec -ti -n demo mycluster-mysql-0 -- bash

      mysql -uroot -p2gvztbvz
      >
      mysql> show variables like 'max_connections';
      +-----------------+-------+
      | Variable_name   | Value |
      +-----------------+-------+
      | max_connections | 600   |
      +-----------------+-------+
      1 row in set (0.00 sec)
      ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

从元信息中可以看到，集群 `mycluster` 的配置文件。

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
  kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-specs=mysql-replication-config -n demo
  ```

  如果集群支持多个模板，你可以通过 `--config-specs` 来指定一个配置模板。执行 `kbcli cluster describe-config mysql-cluster` 查看所有模板的名称。

  <details>

  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: mysql-replication-config        ComponentName: mysql        ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     innodb_buffer_pool_size
    Allowed Values:     [5242880-18446744073709552000]
    Scope:              Global
    Dynamic:            false
    Type:               integer
    Description:        The size in bytes of the memory buffer innodb uses to cache data and indexes of its tables  
  ```
  
  </details>

  * Allowed Values：定义了参数的有效值范围。
  * Dynamic: 决定了参数配置的生效方式。根据被修改参数的生效类型，有**动态**和**静态**两种不同的配置策略。
    * `Dynamic` 为 `true` 时，参数**动态**生效，可在线配置。
    * `Dynamic` 为 `false` 时，参数**静态**生效，需要重新启动 Pod 才能生效。
  * Description：描述了参数的定义。

:::
