---
title: 配置集群参数
description: 如何配置集群参数
keywords: [postgresql, 参数, 配置]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 配置集群参数

本文档将说明如何配置集群参数。

从 v0.9.0 开始，KubeBlocks 支持数据库参数配置动态渲染。当数据库实例的规格发生变化时（例如，用户进行了实例的升降配），KubeBlocks 会根据新的规格自动匹配适用的参数模板。这是因为不同规格的数据库实例可能需要不同的最佳参数配置以优化性能和资源利用率。当用户选择不同的数据库实例规格时，KubeBlocks 会自动检测并确定适用于新规格的最佳数据库参数配置，以确保数据库在新规格下具有最优的性能和配置。

配置动态渲染功能简化了数据库规格调整的过程。用户无需手动更改数据库参数，KubeBlocks 会自动处理参数的更新和配置，以适应新的规格。这样可以节省时间和精力，并减少由于参数设置不正确而导致的性能问题。

需要注意的是，配置动态渲染功能并不适用于所有参数。有些参数可能需要手动进行调整和配置。此外，如果您对数据库参数进行了手动修改，KubeBlocks 在更新数据库参数模板时可能会覆盖手动修改。因此，在使用动态调整功能时，建议先备份和记录自定义的参数设置，以便在需要时进行恢复。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## 查看参数信息

查看集群的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

从元数据中可以找到 PostgreSQL 集群的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* 查看指定参数的使用文档。
  
  ```bash
  kbcli cluster explain-config mycluster -n demo --param=max_connections
  ```
  
  <details>

  <summary>输出</summary>
  
  ```bash
  template meta:
    ConfigSpec: postgresql-configuration ComponentName: postgresql ClusterName: mycluster

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
   kbcli cluster connect mycluster -n demo
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
   kbcli cluster configure mycluster -n demo --set=max_connections=200
   ```

   :::note

   确保设置的值在该参数的 Allowed Values 范围内。如果设置的值不符合取值范围，系统会提示错误。例如：

   ```bash
   kbcli cluster configure mycluster -n demo  --set=max_connections=5
   >
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
   kbcli cluster describe-ops mycluster-reconfiguring-fq6q7 -n demo
   ```

   <details>

   <summary>输出</summary>

   ```bash
   Spec:
     Name: mycluster-reconfiguring-fq6q7 NameSpace: demo Cluster: mycluster Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=postgresql --config-specs=postgresql-configuration --config-file=postgresql.conf --set max_connections=100 --namespace=demo

   Status:
     Start Time:         Mar 17,2023 19:25 UTC+0800
     Completion Time:    Mar 17,2023 19:25 UTC+0800
     Duration:           2s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Mar 17,2023 19:25 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: mycluster-reconfiguring-fq6q7 in Cluster: mycluster
   Mar 17,2023 19:25 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mycluster-reconfiguring-fq6q7 is validated
   Mar 17,2023 19:25 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mycluster, Component: postgresql
   Mar 17,2023 19:25 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: mycluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated: map[postgresql.conf:{"max_connections":"200"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: mycluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated policy: <operatorSyncUpdate>, updated: map[postgresql.conf:{"max_connections":"100"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mycluster-reconfiguring-fq6q7 in Cluster: mycluster
   ```

   </details>

4. 连接至数据库，验证参数是否按预期配置。

   整体搜索过程有 30 秒的延迟，kubelet 需要一些时间同步对 Pod 卷的修改。

   ```bash
   kbcli cluster connect mycluster -n demo
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
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   如果集群中有多个组件，请使用 `--components` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-njk23 -n demo
   ```

3. 连接至数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2. KubeBlocks 将在后续版本支持参数删除。

   :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mycluster -n demo
```

<details>

<summary>输出</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE                  ENABLED   TEMPLATE                    CONSTRAINT        RENDERED                                         COMPONENT    CLUSTER
postgresql-configuration    kb_restore.conf       false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    pg_hba.conf           false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    postgresql.conf       true      postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    kb_pitr.conf          false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-custom-metrics   custom-metrics.yaml   false     postgresql-custom-metrics                     mycluster-postgresql-postgresql-custom-metrics   postgresql   mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT    CONFIG-SPEC-NAME           FILE              STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-fq6q7   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:25 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"100\"}"}
mycluster-reconfiguring-bm84z   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:27 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"200\"}"}
mycluster-reconfiguring-cbqxd   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:35 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"500\"}"}
mycluster-reconfiguring-rcnzb   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed   restart   1/1        Mar 17,2023 19:38 UTC+0800   {"postgresql.conf":"{\"shared_buffers\":\"512MB\"}"}
```

</details>

从上面可以看到，有三个参数被修改过。

通过比较这些改动，可以查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mycluster-reconfiguring-bm84z mycluster-reconfiguring-rcnzb -n demo
>
DIFF-CONFIG RESULT:
  ConfigFile: postgresql.conf TemplateName: postgresql-configuration ComponentName: postgresql ClusterName: mycluster UpdateType: update

PARAMETERNAME     MYCLUSTER-RECONFIGURING-BM84Z    MYCLUSTER-RECONFIGURING-RCNZB
max_connections   200                              500
shared_buffers    256MB                            512MB
```

</TabItem>

<TabItem value="修改配置文件" label="修改配置文件">

1. 获取集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-postgresql -n demo
   ```

2. 按需配置参数。以下实例中添加了 `spec.configFileParams`，用于配置 `max_connections` 参数。

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: postgresql
     configItemDetails:
     - configFileParams:
         my.cnf:
           parameters:
             max_connections: "600"
       configSpec:
         constraintRef: postgresql14-cc
         defaultMode: 292
         keys:
         - postgresql.conf
         name: postgresql-configuration
         namespace: kb-system
         templateRef: postgresql-configuration
         volumeName: postgresql-config
       name: postgresql-configuration
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
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`max_connections` 参数修改为 `600`。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: postgresql
       configurations:
       - keys:
         - key: postgresql.conf
           parameters:
           - key: max_connections
             value: "600"
         name: postgresql-configuration
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

   1. 获取用户名和密码。

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      postgres

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      tf8fhsv2
      ```

   2. 连接集群，验证参数是否按预期配置。

      ```bash
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

:::note

如果您无法找到集群的配置文件，您可以切换到 `kbcli` 页签，使用相关命令查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

</Tabs>
