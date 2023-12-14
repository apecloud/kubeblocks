---
title: 配置集群参数
description: 如何配置集群参数
keywords: [kafka, 参数, 配置, 再配置]
sidebar_position: 1
---

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure` 可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

## 查看参数信息

查看集群的当前配置文件。

```bash
kbcli cluster describe-config mykafka  
```

从元数据中可以看到，集群 `mykafka` 有一个名为 `server.properties` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mykafka --show-detail
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mykafka | head -n 20
  ```

* 查看指定参数的用户指南。
  
  ```bash
  kbcli cluster explain-config mykafka --param=log.cleanup.policy
  ```

  <details>

  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: broker	ClusterName: mykafka

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
   kbcli cluster describe-config mykafka --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy=delete
   ```

2. 调整 `log.cleanup.policy` 的值。

   ```bash
   kbcli cluster configure mykafka --set=log.cleanup.policy=compact
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
   kbcli cluster describe-ops mykafka-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mykafka-reconfiguring-wvqns	NameSpace: default	Cluster: mykafka	Type: Reconfiguring

   Command:
     kbcli cluster configure mykafka --components=broker --config-spec=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Duration:           5s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   ```bash
   # 参数配置已完成
   kbcli cluster describe-ops mykafka-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mykafka-reconfiguring-wvqns	NameSpace: default	Cluster: mykafka	Type: Reconfiguring

   Command:
     kbcli cluster configure mykafka --components=broker --config-spec=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

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

   整体搜索过程有 30 秒的延迟。

   ```bash
   kbcli cluster describe-config mykafka --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mykafka-reconfiguring-wvqns   mykafka   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        Sep 14,2023 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config mykafka
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
   kbcli cluster connect mykafka
   ```

    :::note

    1. `edit-config` 不能同时编辑静态参数和动态参数。
    2. KubeBlocks 未来将支持删除参数。

    :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mykafka                 
```

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mykafka-reconfiguring-wvqns mykafka-reconfiguring-hxqfx
>
DIFF-CONFIG RESULT:
  ConfigFile: server.properties	TemplateName: kafka-configuration-tpl	ComponentName: broker	ClusterName: mykafka	UpdateType: update

PARAMETERNAME         MYKAFKA-RECONFIGURING-WVQNS   MYKAFKA-RECONFIGURING-HXQFX
log.retention.hours   168                           200
```