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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Configure scheduled backups with kbcli:

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

Modify the backup field with kubectl as follows:

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # Enable automatic backups
    enabled: true
    # UTC timezone, the example below stands for 2 A.M. every Monday
    cronExpression: 0 18 * * *
    # Use xtrabackup for backups. If your storage supports snapshot, you can change it to volume-snapshot
    method: xtrabackup
    # Retention period for a backup set
    retentionPeriod: 7d
    # BackupRepo
    repoName: my-repo
```

</TabItem>

</Tabs>
  
After scheduled backup is enabled, execute the following command to check if a CronJob object has been created:

```bash
kubectl get cronjob
>
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mysql-cluster-default-xtrabackup   0 18 * * *   False     0        <none>          57m
```

You can also execute the following command to view cluster information, where the `Data Protection:` section displays the configuration details of automatic backups.

```bash
kbcli cluster describe mysql-cluster
>
...
Data Protection:
BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION
my-repo       Enabled       0 18 * * *        xtrabackup      7d
```
