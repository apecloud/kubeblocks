---
title: 配置集群参数
description: 配置集群参数
keywords: [redis, 参数, 配置, 再配置]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure `可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## 查看参数信息

查看集群的当前配置文件。

```bash
kbcli cluster describe-config mycluster --components=redis -n demo
```

从元信息中可以看到，集群 `mycluster` 有一个名为 `redis.cnf` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

  ```bash
  kbcli cluster describe-config mycluster -n demo --components=redis --show-detail
  ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mycluster -n demo --components=redis |head -n 20
  ```

* 查看指定参数的用户指南。

  ```bash
  kbcli cluster explain-config mycluster -n demo --components=redis --param=acllog-max-len
  ```

  <details>
  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: redis-replication-config	ComponentName: redis	ClusterName: mycluster -n demo

  Configure Constraint:
    Parameter Name:     acllog-max-len
    Allowed Values:     [1-10000]
    Scope:              Global
    Dynamic:            true
    Type:               integer
    Description: 
  ```

  </details>

  * Allowed Values: 定义了参数的有效值范围。
  * Dynamic: 决定了参数配置的生效方式。根据被修改参数的生效类型，有**动态**和**静态**两种不同的配置策略。
    * 当 `Dynamic` 为 `true` 时，参数的生效类型是**动态**的，可以在线配置。
    * 当 `Dynamic` 为 `false` 时，参数的生效类型是**静态**的，需要重新启动 Pod 才能生效。
  * Description: 描述了参数的定义。

## 配置参数

### 使用 configure 命令配置参数

下面以配置 `acllog-max-len` 为例。

1. 查看 `acllog-max-len` 的值。

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "128"
   ```

2. 调整 `acllog-max-len` 的值。

   ```bash
   kbcli cluster configure mycluster -n demo --components=redis --set=acllog-max-len=256
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。如果设置的值不符合取值范围，系统会提示错误。例如：

   ```bash
   kbcli cluster configure mycluster -n demo  --set=acllog-max-len=1000000
   >
   error: failed to validate updated config: [failed to cue template render configure: [configuration."acllog-max-len": 2 errors in empty disjunction:
   configuration."acllog-max-len": conflicting values 128 and 1000000:
       20:43
       155:16
   configuration."acllog-max-len": invalid value 1000000 (out of bound <=10000):
       20:32
   ]
   ]
   ```

   :::

3. 查看参数配置状态。

   `Status.Progress` 和 `Status.Status` 展示参数配置的整体状态，而 `Conditions` 展示详细信息。

   当 `Status.Status` 为 `Succeed` 时，配置完成。

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-zjztm -n demo
   ```

   <details>
   <summary>输出</summary>

   ```bash
   Spec:
     Name: mycluster-reconfiguring-zjztm	NameSpace: demo	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=redis --config-specs=redis-replication-config --config-file=redis.conf --set acllog-max-len=256 --namespace=demo

   Status:
     Start Time:         Sep 29,2024 10:46 UTC+0800
     Duration:           10s
     Status:             Running
     Progress:           1/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Sep 29,2024 10:46 UTC+0800   Progressing          Progressing                    True     wait for the controller to process the OpsRequest: mycluster-reconfiguring-zjztm in Cluster: mycluster
   Sep 29,2024 10:46 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: mycluster-reconfiguring-zjztm is validated
   Sep 29,2024 10:46 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: mycluster, Component: redis
   ```

   </details>

4. 连接到数据库，验证参数是否按预期配置。

   配置生效过程约需要 30 秒，这是由于 kubelet 需要一定时间才能将对 ConfigMap 的修改同步到 Pod 的卷。

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "256"
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
   kbcli cluster describe-ops mycluster-reconfiguring-nflq8 -n demo
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

配置完成后，您可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mycluster -n demo --components=redis
```

<details>
<summary>输出</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME           FILE         ENABLED   TEMPLATE                 CONSTRAINT                  RENDERED                                       COMPONENT   CLUSTER
redis-replication-config   redis.conf   true      redis7-config-template   redis7-config-constraints   mycluster-redis-redis-replication-config   redis       mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME           FILE         STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-zjztm   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 10:46 UTC+0800
mycluster-reconfiguring-zrkq7   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 11:08 UTC+0800   {"redis.conf":"{\"databases\":\"32\",\"maxclients\":\"20000\"}"}
mycluster-reconfiguring-mwbnw   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 11:20 UTC+0800   {"redis.conf":"{\"maxclients\":\"40000\"}"}
```

</details>

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mycluster-reconfiguring-zrkq7 mycluster-reconfiguring-mwbnw
>
DIFF-CONFIG RESULT:
  ConfigFile: redis.conf	TemplateName: redis-replication-config	ComponentName: redis	ClusterName: mycluster	UpdateType: update

PARAMETERNAME   MYCLUSTER-RECONFIGURING-ZRKQ7   MYCLUSTER-RECONFIGURING-MWBNW
maxclients      20000                           40000
```

</TabItem>

<TabItem value="Edit config file" label="Edit config file">

1. 获取集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-redis -n demo
   ```

2. 按需配置参数。以下实例中添加了 `spec.configFileParams`，用于配置 `acllog-max-len` 参数。

    ```yaml
    spec:
      clusterRef: mycluster
      componentName: redis
      configItemDetails:
      - configSpec:
          constraintRef: redis7-config-constraints
          name: redis-replication-config
          namespace: kb-system
          reRenderResourceTypes:
          - vscale
          templateRef: redis7-config-template
          volumeName: redis-config
      - configFileParams:
          redis.conf:
            parameters:
              acllog-max-len: "256"
        name: mycluster-redis-redis-replication-config
    ```

3. 连接集群，确认配置是否生效。

   1. 获取用户名和密码。

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
      >
      kpz77mcs
      ```

   2. 连接集群，验证参数是否按预期配置。

      ```bash
      kubectl exec -ti -n demo mycluster-redis-0 -- bash

      root@mycluster-redis-0:/# redis-cli -a kpz77mcs  --user default

      127.0.0.1:6379> config get parameter acllog-max-len
      1) "acllog-max-len"
      2) "256"
      ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`acllog-max-len` 参数修改为 `256`。

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: mycluster-configuring-demo
      namespace: demo
    spec:
      clusterName: mycluster
      reconfigure:
        componentName: redis
        configurations:
        - keys:
          - key: redis.conf
            parameters:
            - key: acllog-max-len
              value: "256"
          name: redis-replication-config
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

   1. 获取用户名和密码。

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
      >
      kpz77mcs
      ```

   2. 连接集群，验证参数是否按预期配置。

      ```bash
      kubectl exec -ti -n demo mycluster-redis-0 -- bash

      root@mycluster-redis-0:/# redis-cli -a kpz77mcs  --user default
      
      127.0.0.1:6379> config get parameter acllog-max-len
      1) "acllog-max-len"
      2) "256"
      ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

</Tabs>
