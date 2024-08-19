---
title: 配置集群参数
description: 如何配置集群参数
keywords: [postgresql, 参数, 配置]
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
kbcli cluster describe-config pg-cluster 
```

从元数据中可以找到 PostgreSQL 集群的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config pg-cluster --show-detail
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config pg-cluster | head -n 20
  ```

* 查看指定参数的使用文档。
  
  ```bash
  kbcli cluster explain-config pg-cluster --param=max_connections
  ```
  
  <details>

  <summary>输出</summary>
  
  ```bash
  template meta:
    ConfigSpec: postgresql-configuration ComponentName: postgresql ClusterName: pg-cluster

  Configure Constraint:
    Parameter Name:     max_connections
    Allowed Values:     [6-8388607]
    Scope:              Global
    Dynamic:            true
    Type:               integer
    Description:        Sets the maximum number of concurrent connections.
  ```
  </details>

  * Allowed Values：定义了参数的有效值范围。
  * Dynamic：决定了参数配置的生效方式。根据被修改参数的生效类型，有**动态**和**静态**两种不同的配置策略。
    * `Dynamic` 为 `true` 时，参数的生效类型是**动态**的，可以在线配置。
    * `Dynamic` 为 `false` 时，参数的生效类型是**静态**的，需要重新启动 Pod 才能生效。
  * Description：描述了参数的定义。

## 配置参数

### 使用 configure 命令配置参数

下面以配置 `max_connection` 为例。

1. 查看当前 `max_connection` 的值。

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show max_connections;
    max_connections
   -----------------
    100
   (1 row)
   ```

2. 调整 `max_connections` 的值。

   ```bash
   kbcli cluster configure pg-cluster --set=max_connections=200
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。如果设置的值不符合取值范围，系统会提示错误。例如：

   ```bash
   kbcli cluster configure pg-cluster  --set=max_connections=5
   error: failed to validate updated config: [failed to cue template render configure: [pg.acllog-max-len: invalid value 5 (out of bound 6-8388607):
    343:34
   ]
   ]
   ```

   :::

3. 查看参数配置状态。

   `Status.Progress` 和 `Status.Status` 展示参数配置的整体状态，而 `Conditions` 展示详细信息。

   当 `Status.Status` 为` Succeed` 时，配置完成。

   ```bash
   kbcli cluster describe-ops pg-cluster-reconfiguring-fq6q7 -n default
   ```

   <details>

   <summary>输出</summary>

   ```bash
   Spec:
     Name: pg-cluster-reconfiguring-fq6q7 NameSpace: default Cluster: pg-cluster Type: Reconfiguring

   Command:
     kbcli cluster configure pg-cluster --components=postgresql --config-spec=postgresql-configuration --config-file=postgresql.conf --set max_connections=100 --namespace=default

   Status:
     Start Time:         Mar 17,2023 19:25 UTC+0800
     Completion Time:    Mar 17,2023 19:25 UTC+0800
     Duration:           2s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Mar 17,2023 19:25 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: pg-cluster-reconfiguring-fq6q7 in Cluster: pg-cluster
   Mar 17,2023 19:25 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: pg-cluster-reconfiguring-fq6q7 is validated
   Mar 17,2023 19:25 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: pg-cluster, Component: postgresql
   Mar 17,2023 19:25 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated: map[postgresql.conf:{"max_connections":"200"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated policy: <operatorSyncUpdate>, updated: map[postgresql.conf:{"max_connections":"100"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: pg-cluster-reconfiguring-fq6q7 in Cluster: pg-cluster
   ```

   </details>

4. 连接至数据库，验证参数是否按预期配置。

   整体搜索过程有 30 秒的延迟，kubelet 需要一些时间同步对 Pod 卷的修改。

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show max_connections;
    max_connections
   -----------------
    200
   (1 row)
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config pg-cluster
   ```

   :::note

   如果集群中有多个组件，请使用 `--components` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. 连接至数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect pg-cluster
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2. KubeBlocks 将在后续版本支持参数删除。

   :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config pg-cluster
```

<details>

<summary>输出</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE                  ENABLED   TEMPLATE                    CONSTRAINT        RENDERED                                          COMPONENT    CLUSTER
postgresql-configuration    kb_restore.conf       false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    pg_hba.conf           false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    postgresql.conf       true      postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    kb_pitr.conf          false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-custom-metrics   custom-metrics.yaml   false     postgresql-custom-metrics                     pg-cluster-postgresql-postgresql-custom-metrics   postgresql   pg-cluster

History modifications:
OPS-NAME                         CLUSTER      COMPONENT    CONFIG-SPEC-NAME           FILE              STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
pg-cluster-reconfiguring-fq6q7   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:25 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"100\"}"}
pg-cluster-reconfiguring-bm84z   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:27 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"200\"}"}
pg-cluster-reconfiguring-cbqxd   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:35 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"500\"}"}
pg-cluster-reconfiguring-rcnzb   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed   restart   1/1        Mar 17,2023 19:38 UTC+0800   {"postgresql.conf":"{\"shared_buffers\":\"512MB\"}"}
```

</details>

从上面可以看到，有三个参数被修改过。

通过比较这些改动，可以查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config pg-cluster-reconfiguring-bm84z pg-cluster-reconfiguring-rcnzb
>
DIFF-CONFIG RESULT:
  ConfigFile: postgresql.conf TemplateName: postgresql-configuration ComponentName: postgresql ClusterName: pg-cluster UpdateType: update

PARAMETERNAME     PG-CLUSTER-RECONFIGURING-BM84Z   PG-CLUSTER-RECONFIGURING-RCNZB
max_connections   200                              500
shared_buffers    256MB                            512MB
```
