---
title: 配置集群参数
description: 配置集群参数
keywords: [mongodb, 参数, 配置, 再配置]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure` 可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

<Tabs>

<TabItem value="编辑配置文件" label="编辑配置文件" default>

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
    kubectl exec -n demo mycluster-mongodb-0 -- bash -c "cat /etc/mongodb/mongodb.conf | grep verbosity"
    >
      verbosity: 1
    ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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
   | `spec.reconfigure.componentName`                       | 定义了该集群的 component 名称。  |
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
    kubectl exec -n demo mycluster-mongodb-0 -- bash -c "cat /etc/mongodb/mongodb.conf | grep verbosity"
    >
      verbosity: 1
   ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

## 查看参数信息

查看集群的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                      COMPONENT    CLUSTER           
mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mycluster-replicaset-mongodb-config           replicaset   mycluster   
mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mycluster-replicaset-mongodb-config           replicaset   mycluster   
mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mycluster-replicaset-mongodb-metrics-config   replicaset   mycluster   

History modifications:
OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED 
```

从元信息中可以看到，集群 `mycluster` 有一个名为 `mongodb.conf` 的配置文件。

您也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

## 配置参数

### 使用 configure 命令配置参数

下面展示如何将 `systemLog.verbosity` 配置为 1。

1. 将 `systemLog.verbosity` 设置为 1。

   ```bash
   kbcli cluster configure mycluster -n demo --components mongodb --config-specs mongodb-config --config-file mongodb.conf --set systemLog.verbosity=1
   >
   Warning: The parameter change you modified needs to be restarted, which may cause the cluster to be unavailable for a period of time. Do you need to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
   ConfigSpec: mongodb-config      ConfigFile: mongodb.conf      ComponentName: mongodb  ClusterName: mycluster
   OpsRequest mycluster-reconfiguring-q8ndn created successfully, you can view the progress:
          kbcli cluster describe-ops mycluster-reconfiguring-q8ndn -n default
   ```

2. 检查配置历史，查看配置任务是否成功。

   ```bash

    kbcli cluster describe-config mycluster -n demo
    >
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                   COMPONENT   CLUSTER
    mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mycluster-mongodb-mongodb-config           mongodb     mycluster
    mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mycluster-mongodb-mongodb-config           mongodb     mycluster
    mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mycluster-mongodb-mongodb-metrics-config   mongodb     mycluster

    History modifications:
    OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME   FILE           STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
    mycluster-reconfiguring-q8ndn   mycluster   mongodb     mongodb-config     mongodb.conf   Succeed   restart   3/3        Apr 21,2023 18:56 UTC+0800   {"mongodb.conf":"{\"systemLog\":{\"verbosity\":\"1\"}}"}```
   ```

3. 验证配置结果。

   ```bash
   kubectl exec -n demo mycluster-mongodb-0 -- bash -c "cat /etc/mongodb/mongodb.conf | grep verbosity"
   >
     verbosity: 1
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   如果集群中有多个组件，请使用 `--components` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n demo
   ```

3. 连接到数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2. KubeBlocks 未来将支持删除参数。

   :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mycluster -n demo                  
```

如果您执行了多次变更，可比较这些变更，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mycluster-reconfiguring-q8ndn mycluster-reconfiguring-hxqfx -n demo
```

</TabItem>

</Tabs>
