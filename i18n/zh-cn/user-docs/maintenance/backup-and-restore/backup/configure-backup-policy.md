---
title: BackupPolicy
description: 如何配置 BackupPolicy
keywords: [备份, 备份策略]
sidebar_position: 2
sidebar_label: BackupPolicy
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# BackupPolicy

## 配置加密密钥

为了确保恢复的集群能够正常访问，KubeBlocks 在备份集群时会将集群的连接密码加密，并将其安全地存储在 Backup 对象的 Annotation 中。因此，为了保障您的数据安全，强烈建议谨慎分配备份对象的 Get/List 权限，并在安装或升级 KubeBlocks 时，务必指定加密密钥。这些措施将有助于确保你的数据得到妥善保护。

```shell
kbcli kubeblocks install --set dataProtection.encryptionKey="your key"
or
kbcli kubeblocks upgrade --set dataProtection.encryptionKey="your key"
```

## 创建集群

创建测试集群，用于后续备份教程使用。

```shell
# 创建 MySQL 集群
kbcli cluster create mysql mysql-cluster

# 查看 backupPolicy
kbcli cluster list-backup-policy mysql-cluster
>
NAME                                       DEFAULT   CLUSTER         CREATE-TIME                  STATUS      
mysql-cluster-mysql-backup-policy          true      mysql-cluster   May 23,2023 19:53 UTC+0800   Available   
mysql-cluster-mysql-backup-policy-hscale   false     mysql-cluster   May 23,2023 19:53 UTC+0800   Available
```

如无特殊设置，所有备份都会保存在全局默认仓库中，可以执行如下命令查看所有 BackupRepo，其中 `DEFAULT` 字段为 `true`，表示该 BackupRepo 为默认 BackupRepo。

```bash
# 查看 BackupRepo
kbcli backuprepo list
```

## 查看 BackupPolicy

使用 KubeBlocks 创建数据库集群后，对于支持备份的数据库，会自动为其创建一个备份策略（BackupPolicy），可以执行如下命令查看集群的备份策略：

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list-backup-policy mysql-cluster
>
NAME                                       NAMESPACE   DEFAULT   CLUSTER         CREATE-TIME                  STATUS
mysql-cluster-mysql-backup-policy          default     true      mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
mysql-cluster-mysql-backup-policy-hscale   default     false     mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get backuppolicy | grep mysql-cluster
>
mysql-cluster-mysql-backup-policy                            Available   35m
mysql-cluster-mysql-backup-policy-hscale                     Available   35m
```

</TabItem>

</Tabs>

备份策略中包含了该集群支持的备份方法，执行以下命令进行查看备份方法：

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster describe-backup-policy mysql-cluster
> 
kbcli cluster describe-backup-policy mysql-cluster
Summary:
  Name:               mysql-cluster-mysql-backup-policy
  Cluster:            mysql-cluster
  Namespace:          default
  Default:            true

Backup Methods:
NAME              ACTIONSET                           SNAPSHOT-VOLUMES
xtrabackup        xtrabackup-for-apecloud-mysql       false
volume-snapshot   volumesnapshot-for-apecloud-mysql   true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get backuppolicy mysql-cluster-mysql-backup-policy -o yaml
```

</TabItem>

</Tabs>

对于 MySQL 集群而言，默认支持两种备份方法：`xtrabackup` 和 `volume-snapshot`，前者使用备份工具 `xtrabackup` 将 MySQL 数据备份至对象存储中；后者则使用云存储的卷快照能力，通过快照方式对数据进行备份。创建备份时，可以指定要使用哪种备份方法进行备份。
