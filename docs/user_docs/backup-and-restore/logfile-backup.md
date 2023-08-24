---
title: Logfile backup
description: How to back up databases by logfiles
keywords: [backup and restore, logfile]
sidebar_position: 4
sidebar_label: Logfile
---

# Logfile backup

KubeBlocks only supports automatic logfile backup.

## Before you start

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

## Create backup

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Currently, KubeBlocks only supports automatic logfile backup.

1. Run the command below to enable automatic logfile backup.

   ```bash
   kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy --set schedule.logfile.enable=true
   ```

2. View the logfile backup to check whether the backup is successful.

   ```bash
   kbcli cluster list-backups
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Set `pitrEnabled` in the cluster YAML configuration file as `true` to enable automatic logfile backup.

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    ...
    # If the value is true, logfile backup is enabled automatically
    pitrEnabled: true
```

</TabItem>

</Tabs>