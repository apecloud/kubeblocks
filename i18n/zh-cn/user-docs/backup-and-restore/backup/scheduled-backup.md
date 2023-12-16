---
title: 定时备份
description: 如何进行定时备份
keywords: [备份恢复, 定时, 自动备份, 定时备份]
sidebar_position: 3
sidebar_label: 定时备份
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 定时备份

KubeBlocks 支持为集群配置自动备份。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

使用 kbcli 命令配置集群自动备份的命令如下：

```bash
kbcli cluster update mysql-cluster --backup-enabled=true \
--backup-method=xtrabackup --backup-repo-name=my-repo \
--backup-retention-period=7d --backup-cron-expression="0 18 * * *"
```

- `--backup-enabled` 表示是否开启自动备份。
- `--backup-method` 指定备份方法。支持的备份方法可以执行 `kbcli cluster describe-backup-policy mysql-cluster` 命令查看。
- `--backup-repo-name` 指定备份仓库的名称。
- `--backup-retention-period` 指定备份保留时长。以上示例中为 7 天。
- `--backup-cron-expression` 指定自动备份的备份周期。表达式格式与 linux 系统中的定时任务保持一致，时区为 UTC，参考 [Cron](https://en.wikipedia.org/wiki/Cron)。 

</TabItem>

<TabItem value="kubectl" label="kubectl">

用 kubectl 命令修改 Cluster 中的 backup 字段，命令如下：

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # 开启自动备份
    enabled: true
    # UTC 时区, 下面示例是每周一凌晨 2 点
    cronExpression: 0 18 * * *
    # 使用 xtrabackup 进行备份，如果存储支持快照，可以指定为 volume-snapshot
    method: xtrabackup
    # 备份集保留时长
    retentionPeriod: 7d
    # 备份仓库
    repoName: my-repo
```

</TabItem>

</Tabs>
  
开启自动备份后，可以执行如下命令查看是否有 CronJob 对象被创建：

```bash
kubectl get cronjob
>
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mysql-cluster-default-xtrabackup   0 18 * * *   False     0        <none>          57m
```

也可以执行如下命令，查看集群信息，其中 `Data Protection:` 部分会显示自动备份的配置信息。

```bash
kbcli cluster describe mysql-cluster
>
...
Data Protection:
BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION
my-repo       Enabled       0 18 * * *        xtrabackup      7d
```
