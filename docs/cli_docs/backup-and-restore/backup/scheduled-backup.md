---
title: Scheduled backup
description: How to back up databases by schedule
keywords: [backup and restore, schedule, automatic backup, scheduled backup]
sidebar_position: 3
sidebar_label: Scheduled backup
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scheduled backup

KubeBlocks supports configuring scheduled backups for clusters.

```bash
kbcli cluster update mysql-cluster --backup-enabled=true \
--backup-method=xtrabackup --backup-repo-name=my-repo \
--backup-retention-period=7d --backup-cron-expression="0 18 * * *"
```

- `--backup-enabled` indicates whether to enable scheduled backups.
- `--backup-method` specifies the backup method. You can use the `kbcli cluster describe-backup-policy mysql-cluster` command to view the supported backup methods.
- `--backup-repo-name` specifies the name of the backupRepo.
- `--backup-retention-period` specifies the retention period for backups, which is 7 days in the example.
- `--backup-cron-expression` specifies the backup schedule using a cron expression in UTC timezone. Refer to [cron](https://en.wikipedia.org/wiki/Cron) for the expression format.


You can also execute the following command to view cluster information, where the `Data Protection:` section displays the configuration details of automatic backups.

```bash
kbcli cluster describe mysql-cluster
>
...
Data Protection:
BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION
my-repo       Enabled       0 18 * * *        xtrabackup      7d
```
