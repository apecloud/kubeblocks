---
title: 使用 kbcli 迁移 MySQL 数据
description: 如何使用 kbcli 迁移 MySQL 数据
keywords: [mysql, 迁移, kbcli migration]
sidebar_position: 2
sidebar_label: 使用 kbcli 迁移 MySQL 数据
---

# 使用 kbcli 迁移 MySQL 数据

## 开始之前

### 启用 kbcli migration

1. 安装 KubeBlocks: 你可以用 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md) 进行安装。
2. [启用迁移功能](./../../overview/database-engines-supported.md)。

   ```bash
   kbcli addon list

   kbcli addon enable migration
   ```

### 配置源

修改源的配置以支持 CDC。

1. 打开 'log_bin' 配置。
2. 将 'binlog_format' 配置为 'row'。
3. 将 'binlog_row_image' 配置为 'full'。

:::note

1. 修改 'log_bin' 会重启数据库，请确保在非高峰时段进行修改。
2. 修改 'binlog_format' 和 'binlog_row_image' 不会影响现有的 binlog 格式。确保 CDC 拉取的日志时间戳（通常是 CDC 的启动时间）发生在更改完成之后。

:::

### 检查账号权限

确保源账号和目标账号满足以下权限要求。

* 源库
  * REPLICATION SLAVE
  * REPLICATION CLIENT
  * SELECT
* 目标库
  * SELECT
  * INSERT
  * UPDATE
  * DELETE
  * CREATE
  * ALTER
  * DROP

### 初始化目标数据库

创建一个名为 `db_test` 的数据库。

```bash
create database if not exists db_test
```

### 数据采样

为了确保正确性，建议在迁移后准备数据采样进行验证。

## 迁移数据

### 步骤

1. 创建迁移任务。

   ```bash
   kbcli migration create mytask --template apecloud-mysql2mysql \
   --source user:123456@127.0.0.1:5432/db_test \
   --sink user:123456@127.0.0.2:5432/db_test \
   --migration-object '"public.table_test_1","public.table_test_2"'
   ```

   :paperclip: 表 1. 选项详情

   | 选项        | 详情        |
   | :--------- | :---------- |
   | mystask    | 迁移任务的名称，可以自定义。 |
   | `--template` | 指定迁移模板。`--template apecloud-mysql2mysql` 表示此迁移任务使用由 KubeBlocks 创建的从 MySQL 到 MySQL 的模板。执行 `kbcli migration templates` 可查看所有可用的模板和支持的数据库信息。   |
   | `--source`  | 指定源。上例中的 `user:123456@127.0.0.1:5432/db_test` 遵循 `${user_name}:${password}@${database connection url}/${database}` 的格式。在本文档中，连接 URL 使用的是公网地址。 |
   | `--sink`     | 指定目标。上例中的 `user:123456@127.0.0.2:5432/db_test` 遵循 $`{user_name}:${password}@${database connection url}/${database}` 的格式。在本文档中，连接 URL 使用的是 Kubernetes 集群内部的服务地址 |
   | `--migration-object`  | 指定迁移对象。也就是上例中 "public.table_test_1" 和 "public.table_test_2" 中的数据，包括结构数据和库存数据，在迁移期间生成的增量数据将被迁移到目标位置。    |

2. （可选）通过 `--steps` 指定迁移步骤。

   默认按照预检查 -> 结构初始化 -> 数据初始化 -> 增量迁移的顺序进行迁移。你可以使用 `--steps` 参数来指定迁移步骤。例如，按照预检查 -> 数据初始化 -> 增量迁移的顺序执行任务。

   ```bash
   kbcli migration create mytask --template apecloud-mysql2mysql \
   --source user:123456@127.0.0.1:5432/db_test \
   --sink user:123456@127.0.0.2:5432/db_test \
   --migration-object '"public.table_test_1","public.table_test_2"'
   --steps precheck=true,init-struct=false,init-data=true,cdc=true
   ```

3. 查看任务状态。

   ```bash
   # 查看迁移任务列表
   kbcli migration list

   # 查看指定任务的详细信息
   kbcli migration describe ${migration-task-name}
   ```

   有关初始化、CDC 和 CDC Metrics 有几点需要说明。

   * 初始化
     * Precheck：预检查。如果状态显示为 `Failed`，表示初始化预检查未通过。请参考[故障排除](#故障排除)中的示例解决问题。
     * Init-struct：结构初始化。采用幂等处理逻辑，只有在发生严重问题（例如无法连接数据库）时才会失败。
     * Init-data：数据初始化。如果存在大量库存数据，该步骤需要花费较长时间，请注意查看 `Status`。
   * CDC：增量迁移。基于 init-data 步骤之前系统记录的时间戳，系统将按照最终一致性的原则开始数据迁移，并执行源库的 WAL（预写式日志）变更捕获 -> 写入到目标库。正常情况下，如果迁移链路没有被主动终止，CDC 会持续进行。
   * CDC Metrics：增量迁移指标。目前主要提供源库的 WAL LSN（日志序列号）和 CDC 完成“捕获 -> 写入”过程的相应时间戳（请注意时间戳显示的是 Pod 容器运行时的本地时区）。

     :::note

     系统每 10 分钟更新一次 CDC Metrics。即，如果源库存在连续的数据写入，metrics.timestamp 会相对于当前时间延迟 10 分钟。

     :::

4. 使用准备好的数据采样对迁移进行验证。

### 故障排除

如果上述任何步骤失败，可执行以下命令排查失败原因。

```bash
# --step: Specify the step. Allowed values: precheck,init-struct,init-data,cdc
kbcli migration logs ${migration-task-name} --step ${step-name}
```

## 切换应用程序

### 开始之前

* 确保 KubeBlocks 迁移任务正常运行。
* 为了区分对话信息并提高数据安全性，建议创建和授权另一个专用于数据迁移的账号。
* 切换过程保险起见需要停止业务写入，建议在业务低峰期做切换。
* 切换前建议进行抽样数据验证，以确保正确性。

### 步骤

1. 检查迁移任务状态，确保任务正常进行。
   1. 查看详细信息，确保初始化的所有步骤均为 `Complete`，且 CDC 处于 `Running` 状态。

      ```bash
      kbcli migration describe ${migration-task-name}
      ```

   2. 在源库连续写入的前提下，观察位点是否仍在持续推进切几乎没有延迟。例如：

      ```bash
      kbcli migration logs ${migration-task-name} --step cdc | grep current_position
      ```

      输出结果每 10 秒刷新一次。

      ![Timestamp](../../../img/pgsql-migration-timestamp.png)
2. 将业务暂时中断，禁止新的业务数据写入源库。
3. 再次判断传输任务状态，确认任务正常，保持至少 1 分钟。

   参考步骤 1，观察链路是否正常且位点是否符合预期。
4. 使用目标数据库恢复业务。
5. 使用准备好的数据采样验证切换是否正确。

## 清理环境

在迁移任务完成后，可以终止迁移任务和相关功能。

### 删除任务

终止迁移任务不会影响源数据库和目标数据库中的数据。

```bash
kbcli migration terminate ${migration-task-name}
```

### 停用 kbcli migration

1. 检查是否有正在运行的迁移任务。

   ```bash
   kbcli migration list
   ```

2. 禁用迁移插件。

   ```bash
   kbcli addon disable migration
   ```

3. 手动删除 Kubernetes CRD（自定义资源定义）。

   ```bash
   kubectl delete crd migrationtasks.datamigration.apecloud.io migrationtemplates.datamigration.apecloud.io serialjobs.common.apecloud.io
   ```
