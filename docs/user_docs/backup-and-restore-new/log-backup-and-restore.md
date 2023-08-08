---
title: Log file backup and restore
description: How to back up and restore databses by log files
keywords: [backup and restore, log file]
sidebar_position: 4
sidebar_label: Log file
---

# Log file backup

KubeBlocks only supports automatic log file backup.

## Before you start

Prepare a cluster for testing the backup and restore function. The following insstructions uses MySQL as an example.

1. Create a cluster.

   ```bash
   kbcli cluster create mysql mysql-cluster
   ```

2. View the backup policy.

   ```bash
   kbcli cluster list-backup-policy mysql-cluster
   ```

   By default, all the backups are stored in the default global repository. You can specify a new repository by editing the BackupPolicy resource.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster edit-backup-policy mysql-cluster --set="datafile.backupRepoName=my-repo"
   ```

   </TabItem>

   <TabItem value="kubectl" label="kubectl">

   ```bash
   kubectl edit backuppolicy mysql-cluster-mysql-backup-policy
   ...
   spec:
     datafile:
       ... 
       # Specify a backup repository name
       backupRepoName: my-repo
   ```

   </TabItem>

   </Tabs>

## Create backup

1. Run the command below to enable automatic log file backup.

   ```bash
   kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy --set schedule.logfile.enable=true
   ```

2. View the log file backup to check whether the backup is successful.

   ```bash
   kbcli cluster list-backups
   ```
