---
title: Scheduled backup
description: How to back up databases by schedule
keywords: [backup and restore, schedule, scheduled backup]
sidebar_position: 5
sidebar_label: Scheduled backup
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scheduled backup

## Backup by schedule

You can customize your backup schedule by modifying relevant parameters.

:::caution

The backup created by kbcli or kubectl is saved permanently. If you want to delete the backup, you can delete it manually.

:::

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy
>
spec:
  ...
  schedule:
    datafile:
      # UTC time zone, the example below stands for 2 A.M. every Monday
      cronExpression: "0 18 * * 0"
      # Enable this function
      enable: true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # Enable automatic backup
    enabled: true
    # UTC time zone, the example below stands for 2 A.M. every Monday
    cronExpression: 0 18 * * *
    # It specifies the backup method. Here is an example of backupTool. If your storage supports snapshot, you can change it to snapshot
    method: backupTool
    # Disable PITR. If enabled, automatic backup is enabled accordingly
    pitrEnabled: false
    # Retention period for a backup set
    retentionPeriod: 1d
```

</TabItem>

</Tabs>

## Backup automatically by log

KubeBlocks only supports automatic log backup.

### Before you start

Prepare a cluster for testing the backup and restore function. The following instructions use MySQL as an example.

1. Create a cluster.

   ```bash
   kbcli cluster create mysql mysql-cluster
   ```

2. View the backup policy.

   ```bash
   kbcli cluster list-backup-policy mysql-cluster
   ```

   By default, all the backups are stored in the default global repository but you can specify a new repository by [editing the BackupPolicy resource](./backup-repo.md#optional-change-the-backup-repository-for-a-cluster).

### Create backup

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Currently, KubeBlocks only supports automatic log backup.

1. Run the command below to enable automatic log backup.

   ```bash
   kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy --set schedule.logfile.enable=true
   ```

2. View the log backup to check whether the backup is successful.

   ```bash
   kbcli cluster list-backups
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Set `pitrEnabled` in the cluster YAML configuration file as `true` to enable automatic log backup.

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    ...
    # If the value is true, log backup is enabled automatically
    pitrEnabled: true
```

</TabItem>

</Tabs>
