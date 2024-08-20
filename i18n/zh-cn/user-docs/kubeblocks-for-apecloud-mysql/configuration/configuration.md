---
title: 配置集群参数
description: 如何配置集群参数
keywords: [mysql, 参数, 配置]
sidebar_position: 1
---

# 配置集群参数

本文档将说明如何配置集群参数。

从 v0.9.0 开始，KubeBlocks 支持数据库参数配置动态渲染。当数据库实例的规格发生变化时（例如，用户进行了实例的升降配），KubeBlocks 会根据新的规格自动匹配适用的参数模板。这是因为不同规格的数据库实例可能需要不同的最佳参数配置以优化性能和资源利用率。当用户选择不同的数据库实例规格时，KubeBlocks 会自动检测并确定适用于新规格的最佳数据库参数配置，以确保数据库在新规格下具有最优的性能和配置。

配置动态渲染功能简化了数据库规格调整的过程。用户无需手动更改数据库参数，KubeBlocks 会自动处理参数的更新和配置，以适应新的规格。这样可以节省时间和精力，并减少由于参数设置不正确而导致的性能问题。

需要注意的是，配置动态渲染功能并不适用于所有参数。有些参数可能需要手动进行调整和配置。此外，如果你对数据库参数进行了手动修改，KubeBlocks 在更新数据库参数模板时可能会覆盖手动修改。因此，在使用动态调整功能时，建议先备份和记录自定义的参数设置，以便在需要时进行恢复。

## 查看参数信息

查看集群的配置文件。

```bash
kbcli cluster describe-config mysql-cluster  
```

从元信息中可以看到，集群 `mysql-cluster` 有一个名为 `my.cnf` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mysql-cluster --show-detail
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mysql-cluster | head -n 20
  ```

* 查看指定参数的使用文档。
  
  ```bash
  kbcli cluster explain-config mysql-cluster --param=innodb_buffer_pool_size --config-specs=mysql-consensusset-config
  ```

  ApeCloud MySQL 目前支持多个模板，你可以通过 `--config-specs` 来指定一个配置模板。执行 `kbcli cluster describe-config mysql-cluster` 查看所有模板的名称。

  <details>

  <summary>输出</summary>

  ```bash
  template meta:
    ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mysql-cluster

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

## 配置参数

### 使用 configure 命令配置参数

以下示例以配置 `max_connections` 和 `innodb_buffer_pool_size` 为例。

1. 查看 `max_connections` 和 `innodb_buffer_pool_size` 的当前值。

   ```bash
   kbcli cluster connect mysql-cluster
   ```

   ```bash
   mysql> show variables like '%max_connections%';
   >
   +-----------------+-------+
   | Variable_name   | Value |
   +-----------------+-------+
   | max_connections | 167   |
   +-----------------+-------+
   1 row in set (0.04 sec)
   ```

   ```bash
   mysql> show variables like '%innodb_buffer_pool_size%';
   >
   +-------------------------+-----------+
   | Variable_name           | Value     |
   +-------------------------+-----------+
   | innodb_buffer_pool_size | 134217728 |
   +-------------------------+-----------+
   1 row in set (0.00 sec)
   ```

2. 调整 `max_connections` 和 `innodb_buffer_pool_size` 的值。

   ```bash
   kbcli cluster configure mysql-cluster --set=max_connections=600,innodb_buffer_pool_size=512M
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。如果设置的值不符合取值范围，系统会提示错误。例如：

   ```bash
   kbcli cluster configure mysql-cluster  --set=max_connections=200,innodb_buffer_pool_size=2097152
   error: failed to validate updated config: [failed to cue template render configure: [mysqld.innodb_buffer_pool_size: invalid value 2097152 (out of bound >=5242880):
    343:34
   ]
   ]
   ```

   :::

3.  查看参数配置状态。

    `Status.Progress` 展示参数配置的整体状态，而 `Conditions` 展示详细信息。

    ```bash
    kbcli cluster describe-ops mysql-cluster-reconfiguring-z2wvn -n default
    ```

    <details>

    <summary>输出</summary>

    ```bash
    Spec:
      Name: mysql-cluster-reconfiguring-z2wvn        NameSpace: default        Cluster: mysql-cluster        Type: Reconfiguring

      Command:
        kbcli cluster configure mysql-cluster --components=mysql --template-name=mysql-consensusset-config --config-file=my.cnf --set innodb_buffer_pool_size=512M --set max_connections=600

      Status:
        Start Time:         Mar 13,2023 02:55 UTC+0800
        Completion Time:    Mar 13,2023 02:55 UTC+0800
        Duration:           1s
        Status:             Succeed
        Progress:           1/1

      Conditions:
      LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
      Mar 13,2023 02:55 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
      Mar 13,2023 02:55 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mysql-cluster-reconfiguring-z2wvn is validated
      Mar 13,2023 02:55 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mysql-cluster, Component: mysql
      Mar 13,2023 02:55 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
      Mar 13,2023 02:55 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: mysql-cluster, Component: mysql, ConfigTpl: mysql-consensusset-config, info: updated policy: <autoReload>, updated: map[my.cnf:{"mysqld":{"innodb_buffer_pool_size":"512M","max_connections":"600"}}], added: map[], deleted:map[]
      Mar 13,2023 02:55 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mysql-cluster-reconfiguring-z2wvn in Cluster: mysql-cluster
      ```

      </details>

4. 连接到数据库，验证参数是否按预期配置。

   整体搜索过程有 30 秒的延迟，因为 kubelet 需要一些时间同步对 Pod 卷的修改。

   ```bash
   kbcli cluster connect mysql-cluster
   ```

   ```bash
   mysql> show variables like '%max_connections%';
   >
   +-----------------+-------+
   | Variable_name   | Value |
   +-----------------+-------+
   | max_connections | 600   |
   +-----------------+-------+
   1 row in set (0.04 sec)
   ```
  
   ```bash
   mysql> show variables like '%innodb_buffer_pool_size%';
   >
   +-------------------------+-----------+
   | Variable_name           | Value     |
   +-------------------------+-----------+
   | innodb_buffer_pool_size | 536870912 |
   +-------------------------+-----------+
   1 row in set (0.00 sec)
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

下面以配置 MySQL 单机版为例。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config mysql-cluster --config-spec=mysql-consensusset-config
   ```

   :::note

   * ApeCloud MySQL 目前支持多个模板，需通过 `--config-spec` 指定一个配置模板。执行 `kbcli cluster describe-config mysql-cluster` 查看所有模板的名称。
   * 如果集群中有多个组件，请使用 `--components` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. 连接到数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect mysql-cluster
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2.  KubeBlocks 未来将支持删除参数。

   :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mysql-cluster
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE     ENABLED   TEMPLATE                   CONSTRAINT                    RENDERED                                  COMPONENT   CLUSTER                
mysql-consensusset-config   my.cnf   true      mysql8.0-config-template   mysql8.0-config-constraints   mysql-cluster-mysql-mysql-config   mysql       mysql-cluster   

History modifications:
OPS-NAME                            CLUSTER         COMPONENT   CONFIG-SPEC-NAME            FILE     STATUS    POLICY   PROGRESS   CREATED-TIME                 VALID-UPDATED                                                                                                                     
mysql-cluster-reconfiguring-4q5kv   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed   reload   -/-        Mar 16,2023 15:44 UTC+0800   {"my.cnf":"{\"mysqld\":{\"max_connections\":\"3000\",\"read_buffer_size\":\"24288\"}}"}                                           
mysql-cluster-reconfiguring-cclvm   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed   reload   -/-        Mar 16,2023 17:28 UTC+0800   {"my.cnf":"{\"mysqld\":{\"innodb_buffer_pool_size\":\"1G\",\"max_connections\":\"600\"}}"}   
mysql-cluster-reconfiguring-gx58r   mysql-cluster   mysql       mysql-consensusset-config   my.cnf   Succeed            -/-        Mar 16,2023 17:28 UTC+0800                       
```

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mysql-cluster-reconfiguring-4q5kv mysql-cluster-reconfiguring-gx58r
>
DIFF-CONFIGURE RESULT:
  ConfigFile: my.cnf    TemplateName: mysql-consensusset-config ComponentName: mysql    ClusterName: mysql-cluster       UpdateType: update      

PARAMETERNAME             MYSQL-CLUSTER-RECONFIGURING-4Q5KV   MYSQL-CLUSTER-RECONFIGURING-GX58R   
max_connections           3000                                600                                        
innodb_buffer_pool_size   128M                                1G 
```
