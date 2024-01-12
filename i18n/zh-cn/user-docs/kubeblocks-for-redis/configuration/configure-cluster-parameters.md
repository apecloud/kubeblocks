---
title: 配置集群参数
description: 配置集群参数
keywords: [redis, 参数, 配置, 再配置]
sidebar_position: 1
---

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure `可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

## 查看参数信息

查看集群的当前配置文件。
 
```bash
kbcli cluster describe-config redis-cluster --component=redis
```

从元信息中可以看到，集群 `redis-cluster` 有一个名为 `redis.cnf` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

  ```bash
  kbcli cluster describe-config redis-cluster --component=redis --show-detail
  ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config redis-cluster --component=redis |head -n 20
  ```

* 查看指定参数的用户指南。

  ```bash
  kbcli cluster explain-config redis-cluster --component=redis --param=acllog-max-len
  ```

  <details>
  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: redis-replication-config	ComponentName: redis	ClusterName: redis-cluster

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
   kbcli cluster connect redis-cluster
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "128"
   ```

2. 调整 `acllog-max-len` 的值。

   ```bash
   kbcli cluster configure redis-cluster --component=redis --set=acllog-max-len=256
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。如果设置的值不符合取值范围，系统会提示错误。例如：

   ```bash
   kbcli cluster configure redis-cluster  --set=acllog-max-len=1000000
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
   kbcli cluster describe-ops redis-cluster-reconfiguring-zjztm -n default
   ```

   <details>
   <summary>输出</summary>

   ```bash
   Spec:
     Name: redis-cluster-reconfiguring-zjztm	NameSpace: default	Cluster: redis-cluster	Type: Reconfiguring

   Command:
     kbcli cluster configure redis-cluster --components=redis --config-spec=redis-replication-config --config-file=redis.conf --set acllog-max-len=256 --namespace=default

   Status:
     Start Time:         Apr 17,2023 17:22 UTC+0800
     Duration:           10s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Apr 17,2023 17:22 UTC+0800   Progressing          OpsRequestProgressingStarted   True     Start to process the OpsRequest: redis-cluster-reconfiguring-zjztm in Cluster: redis-cluster
   Apr 17,2023 17:22 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: redis-cluster-reconfiguring-zjztm is validated
   Apr 17,2023 17:22 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: redis-cluster, Component: redis
   Apr 17,2023 17:22 UTC+0800   ReconfigureRunning   ReconfigureRunning             True     Reconfiguring in Cluster: redis-cluster, Component: redis, ConfigSpec: redis-replication-config
   ```

   </details>

4. 连接到数据库，验证参数是否按预期配置。

   整体搜索过程有 30 秒的延迟，kubelet 需要一些时间同步对 Pod 卷的修改。

   ```bash
   kbcli cluster connect redis-cluster
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
   kbcli cluster edit-config redis-cluster
   ```

    :::note

    如果集群中有多个组件，请使用 `--component` 参数指定一个组件。

    :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. 连接到数据库，验证参数是否按预期配置。
   ```bash
   kbcli cluster connect redis-cluster
   ```

    :::note

    1. `edit-config` 不能同时编辑静态参数和动态参数。
    2. KubeBlocks 未来将支持删除参数。

    :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config redis-cluster --component=redis
```

<details>
<summary>输出</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME           FILE         ENABLED   TEMPLATE                 CONSTRAINT                  RENDERED                                       COMPONENT   CLUSTER
redis-replication-config   redis.conf   true      redis7-config-template   redis7-config-constraints   redis-cluster-redis-redis-replication-config   redis       redis-cluster

History modifications:
OPS-NAME                            CLUSTER         COMPONENT   CONFIG-SPEC-NAME           FILE         STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
redis-cluster-reconfiguring-zjztm   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:22 UTC+0800
redis-cluster-reconfiguring-zrkq7   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:28 UTC+0800   {"redis.conf":"{\"databases\":\"32\",\"maxclients\":\"20000\"}"}
redis-cluster-reconfiguring-mwbnw   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:35 UTC+0800   {"redis.conf":"{\"maxclients\":\"40000\"}"}
```

</details>

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config redis-cluster-reconfiguring-zrkq7 redis-cluster-reconfiguring-mwbnw
>
DIFF-CONFIG RESULT:
  ConfigFile: redis.conf	TemplateName: redis-replication-config	ComponentName: redis	ClusterName: redis-cluster	UpdateType: update

PARAMETERNAME   REDIS-CLUSTER-RECONFIGURING-ZRKQ7   REDIS-CLUSTER-RECONFIGURING-MWBNW
maxclients      20000                               40000
```
