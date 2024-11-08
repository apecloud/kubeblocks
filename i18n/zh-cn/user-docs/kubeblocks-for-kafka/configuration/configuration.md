---
title: 配置集群参数
description: 如何配置集群参数
keywords: [kafka, 参数, 配置, 再配置]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure` 可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## 查看参数信息

查看集群的当前配置文件。

```bash
kbcli cluster describe-config mycluster -n demo 
```

从元数据中可以看到，集群 `mycluster` 有一个名为 `server.properties` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* 查看指定参数的用户指南。
  
  ```bash
  kbcli cluster explain-config mycluster -n demo --param=log.cleanup.policy
  ```

  <details>

  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: broker	ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     log.cleanup.policy
    Allowed Values:     "compact","delete"
    Scope:              Global
    Dynamic:            false
    Type:               string
    Description:        The default cleanup policy for segments beyond the retention window. A comma separated list of valid policies. 
  ```
  
  </details>

  * Allowed Values： 定义了参数的有效值范围。
  * Dynamic：决定了参数配置的生效方式。目前，Kafka 仅支持 `Dynamic` 为 `false` 的情况，参数的生效类型是**静态**的，需要重新启动 Pod 才能生效。
  * Description：描述了参数的定义。

## 配置参数

### 使用 configure 命令配置参数

1. 查看 `log.cleanup.policy` 的值。

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy=delete
   ```

2. 调整 `log.cleanup.policy` 的值。

   ```bash
   kbcli cluster configure mycluster -n demo --set=log.cleanup.policy=compact
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。否则，配置可能会失败。

   :::

3. 查看参数配置状态。

   `Status.Progress` 和 `Status.Status` 展示参数配置的整体状态，而 `Conditions` 展示详细信息。

   当 `Status.Status` 为 `Succeed` 时，配置完成。

   <details>

   <summary>输出</summary>

   ```bash
   # 参数配置进行中
   kbcli cluster describe-ops mycluster-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mycluster-reconfiguring-wvqns	NameSpace: default	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=broker --config-specs=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Duration:           5s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   ```bash
   # 参数配置已完成
   kbcli cluster describe-ops mycluster-reconfiguring-wvqns -n demo
   >
   Spec:
     Name: mycluster-reconfiguring-wvqns	NameSpace: default	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=broker --config-specs=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Completion Time:    Sep 14,2023 16:28 UTC+0800
     Duration:           25s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   </details>

4. 查看配置文件，验证参数是否按预期配置。

   配置生效过程约需要 30 秒，这是由于 kubelet 需要一定时间才能将对 ConfigMap 的修改同步到 Pod 的卷。

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mycluster-reconfiguring-wvqns   mycluster   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        Sep 14,2023 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   如果集群中有多个组件，请使用 `--component` 参数指定一个组件。

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

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mycluster-reconfiguring-wvqns mycluster-reconfiguring-hxqfx -n demo
>
DIFF-CONFIG RESULT:
  ConfigFile: server.properties	TemplateName: kafka-configuration-tpl	ComponentName: broker	ClusterName: mycluster	UpdateType: update

PARAMETERNAME         MYCLUSTER-RECONFIGURING-WVQNS   MYCLUSTER-RECONFIGURING-HXQFX
log.retention.hours   168                             200
```

</TabItem>

<TabItem value="编辑配置文件" label="编辑配置文件">

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

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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

3. 确认配置是否生效。

   ```bash
   kbcli cluster describe-config mycluster --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

</Tabs>
