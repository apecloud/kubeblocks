---
title: 备份恢复
description: 创建备份并恢复
keywords: [数据库引擎, 备份, 恢复]
sidebar_position: 3
sidebar_label: 备份恢复
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 备份恢复

本文档以 Oracle MySQL 为例，介绍如何在 KubeBlocks 中创建备份并恢复（点击参考[完整 PR](https://github.com/apecloud/learn-kubeblocks-addon/tree/main/tutorial-2-backup-restore/)）。

备份可以分为多种类型。从方式上分，有卷快照备份和文件备份；从内容上分，有数据备份和日志备份；从量上分，有全量备份和增量备份；从时间上分，有定时备份和按需备份等等。

本文档主要介绍如何在 KubeBlocks 上实现最常见的全量快照备份和文件备份。首先，需要明确两个前提：

- 快照备份依赖 Kubernetes 的卷快照能力。
- 文件备份依赖各个数据库引擎的备份工具。
  
Table 1. 展示了 KubeBlocks 中与备份相关的常见概念，后文会通过示例说明它们的作用和使用方法。

:paperclip: Table 1. 术语表

| 术语 | 说明 | 范围 |
| :--- | :---------- | :---- |
| Backup | 备份对象 <br /> 即备份对象的实体。 | 命名空间 |
| BackupPolicy | 备份策略 <br /> 定义各种备份类型的相关策略， 比如调度、备份保留时间、使用哪种备份工具等。 | 命名空间 |
| BackupTool | 备份工具 <br /> 即 KubeBlocks 中备份工具的载体。每个 BackupTool 都应该实现对应备份工具的备份逻辑和恢复逻辑。 | 集群 |
| BackupPolicyTemplate | 备份策略模板<br />即备份与 ClusterDefinition 结合的桥梁。在创建集群时，KubeBlocks 会根据 BackupPolicyTemplate 自动为每个集群对象生成一个默认的备份策略。 | 集群 |

## 开始之前

- 阅读[添加数据库引擎](./how-to-add-an-add-on.md)文档。
- 了解 K8s 的基本概念，例如 Pod、PVC、PV、VolumeSnapshot 等。

## 步骤 1. 准备环境

1. 安装 CSI Driver。

   因为卷快照只支持 CSI Driver，请确保你的 Kubernetes 已经正确配置。

   - 如果在本地环境，你可以通过 KubeBlocks Add-on 功能快速安装 `csi-host-driver`：

     ```bash
     kbcli addon enable csi-hostpath-driver
     ```

   - 如果是云环境，则需要根据各个云环境配置相应的 CSI Driver。

2. 将该 `storageclass` 设置为默认值，方便后续创建集群。

   ```bash
   kubectl get sc
   >
   NAME                        PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   csi-hostpath-sc (default)   hostpath.csi.k8s.io     Delete          WaitForFirstConsumer   true                   35s
   ```

## 步骤 2. 指定卷类型

在 ClusterDefinition 中指定卷类型 [必须配置]。

```yaml
  componentDefs:
    - name: mysql-compdef
      characterType: mysql
      workloadType: Stateful
      service:
        ports:
          - name: mysql
            port: 3306
            targetPort: mysql
      volumeTypes:
        - name: data
          type: data
```

`volumeTypes` 用于说明卷类型和卷名称。卷类型（`volumeTypes.type`）分为两种：

- `data`：数据信息；
- `log`：日志信息。

KubeBlocks 支持对数据和日志的不同备份方式。这里只配置了数据卷的信息。

## 步骤 3. 添加备份配置

准备两个文件 `BackupPolicyTemplate.yml` 和 `BackupTool.yml`。

### BackupPolicy template

这是备份策略的模版，主要描述：

1. 为 Cluster 的哪个组件备份；
2. 是否定时备份；
3. 如何设置快照备份；
4. 如何设置文件备份；

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: oracle-mysql-backup-policy-template
  labels:
    clusterdefinition.kubeblocks.io/name: oracle-mysql # 通过 label 指定作用域，必填
spec:
  clusterDefinitionRef: oracle-mysql  # 指定作用域，是哪个 ClusterDef 生成的集群
  backupPolicies:
  - componentDefRef: mysql-compdef    # 指定作用域，是哪一个组件相关的
    schedule:                         # schedule 用于指定定时备份的时间和启动情况
      snapshot:
        enable: true                  # 启动定时快照备份
        cronExpression: "0 18 * * *"
      datafile:                       # 禁用定时文件备份
        enable: false
        cronExpression: "0 18 * * *"        
    snapshot:                         # 快照备份，默认保留最新的 5 个版本
      backupsHistoryLimit: 5
    datafile:                         # 数据文件备份，依赖备份工具
      backupToolName: oracle-mysql-xtrabackup
```
如果启用了定时任务，KubeBlocks 会在后台创建一个 CronJob。

在一个新的集群创建后，KubeBlocks 会通过 `clusterdefinition.kubeblocks.io/name` 标签来查找对应的 template 名，并创建相应的 BackupPolicy。

:::note

如果你成功添加了 `BackupPolicyTemplate`，但是新建的集群没有默认的 BackupPolicy，请检查：

1. `ClusterDefinionRef` 是否正确；
2. `BackupPolicyTemplate` 的 lable 是否正确；
3. 是否有多个关联的 `BackupPolicyTemplate`。
   如果是，需要通过 annotation 标记其中一个为默认模板。

   ```yaml
     annotations:
      dataprotection.kubeblocks.io/is-default-policy-template: "true"
   ```

:::

### BackupTool

:::note

`BackupTool` 主要是为文件备份服务。如果你只需要快照备份，不需要文件备份，则不需要配置 `BackupTool`。

:::

`BackTool.yml` 用来描述备份工具的具体执行逻辑，主要服务于文件备份（datafile）。它包括：

1. 备份工具 image；
2. backup 的脚本；
3. restore 的脚本。

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupTool
metadata:
  name: oracle-mysql-xtrabackup
  labels:
spec:
  image: docker.io/perconalab/percona-xtrabackup:8.0.32  # 通过 xtrabackup 备份
  env:                         # 注入依赖的环境变量名称
    - name: DATA_DIR
      value: /var/lib/mysql
  physical:
    restoreCommands:           # restore 命令
      - sh
      - -c
      ...
  backupCommands:             # backup 命令
    - sh
    - -c
    ...
```

`BackupTool` 的配置和备份工具强相关。比如这里使用 Percona Xtrabackup 工具备份，就需要在 `backupCommands` 和 `restoreCommands` 中填写脚本。

## 步骤 4. 备份恢复集群

一切就绪，下面来试试如何备份和恢复一个集群。

### 4.1 创建集群

因为前面已经添加了 `BackupPolicyTemplate`，在创建完集群后，KubeBlocks 会监测备份策略，并为该集群创建一个 `BackupPolicy`，可通过以下命令查看：

1. 创建集群。

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster create mycluster --cluster-definition oracle-mysql 
   ```

   </TabItem>

   <TabItem value="Helm" label="Helm">

   ```bash
   helm install mysql ./tutorial-2-backup-restore/oracle-mysql
   ```

   </TabItem>

   </Tabs>

2. 查看集群备份策略。

   ```bash
   kbcli cluster list-backup-policy mycluster
   ```

### 4.2 快照备份

```bash
kbcli cluster backup mycluster --type snapshot
```

`type` 指定备份类型是 Snapshot 还是 datafile。

如果有多个备份策略，可以通过 `--policy` flag 指定。

### 4.3 文件备份

KubeBlocks 支持备份到本地和云上对象存储。这里展示如何备份到本地。

1. 修改 BackupPolicy，指定 PVC 名称。

   如 `spec.datafile.persistentVolumeClaim.name` 模块所示, 需要指定备份 pvc 的名称。

   ```yaml
     spec:
       datafile:
         backupToolName: oracle-mysql-xtrabackup
         backupsHistoryLimit: 7
         persistentVolumeClaim:
           name: mycluster-backup-pvc
           createPolicy: IfNotPresent
           initCapacity: 20Gi
   ```

2. 执行备份命令, 将 `--type` 设置为 `datafile`。

   ```bash
   kbcli cluster backup mycluster  --type datafile
   ```

### 4.4 从备份创建集群

1. 查看备份。

   ```bash
   kbcli cluster list-backups
   ```

2. 选择备份创建集群。

   ```bash
   kbcli cluster restore <clusterName> --backup <backup-name>
   ```

很快一个新的集群就创建出来了。

:::caution

需要注意的是，某些数据库只有在第一次初始化的时候才创建 root 账号和密码。

因此，前面通过备份恢复出来的数据库集群，虽然在流程上创建了新的 root 账号和密码，但是并没有生效，还需要通过原集群的 root 账号和密码登录。

:::

## 参考资料

- 关于 KubeBlocks 的备份恢复功能，可参考[备份恢复](./../../user-docs/backup-and-restore/overview.md)文档。

## 附录

### A.1 集群数据保护策略

KubeBlocks 对有状态的集群提供了数据保护策略。不同策略提供了不同的数据备份方式，大家可以尝试下列场景：

1. 如果通过 `kbcli cluster delete` 删除了集群，备份还在吗？
2. 如果把 cluster 的 `terminationPolicy` 改为 `WipeOut`，再删除，备份还在吗？
3. 如果把 cluster 的 `terminationPolicy` 改为 `DoNotTerminate`，再删除，会发生什么？

:::note

请参考 [KubeBlocks 的数据保护行为](./../../user-docs/kubeblocks-for-mysql/cluster-management/delete-mysql-cluster.md#终止策略)。

:::

### A.2 查看备份情况

[步骤 4](#步骤-4-备份恢复集群) 中通过 backup 子命令创建了备份。

```bash
kbcli cluster backup mycluster  --type snapshot
```

可以看到生成了一个新的备份对象，可以通过 `describe-backup` 子命令查看更多信息。

```bash
kbcli cluster describe-backup <your-back-up-name>
```
