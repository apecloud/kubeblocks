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

也可以执行如下命令，查看集群信息，其中 `Data Protection:` 部分会显示自动备份的配置信息。

```bash
kbcli cluster describe mysql-cluster
>
...
Data Protection:
BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION
my-repo       Enabled       0 18 * * *        xtrabackup      7d
```
