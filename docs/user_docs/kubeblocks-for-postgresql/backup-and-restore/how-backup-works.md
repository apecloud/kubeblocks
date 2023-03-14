---
title: How backup works
description: How PostgreSQL backup works
sidebar_position: 1
---

# How backup works
Kubeblocks integrates cloud-native backup and restore solutions, and currently supports PostgreSQL backup and recovery by snapshots.
- Backup 
  - Install `CSI driver` that supports the snapshots function and the Kubeblocks native plug-in `snapshot-controller` which is integrated in kbcli in your Kubernetes environment.
  - Initialize all the plug-ins and choose a PostgreSQL cluster. And then create a backup with kbcli. Kbcli creates a `BackupPolicy` and a `Backup` object and associates them with the PostgreSQL cluster by `Labels`.
  - Once a PostgreSQL cluster has backup executed, `BackupPolicy` configures a `CronJob` automatically to  backup the cluster periodically. 
  - Check the Backup object with kbcli, when the status is Completed, the backup is done.

- Restore
  - Check the backup list with `kbcli cluster list-backups` to choose a backup name that is completed.
  - Use `kbcli cluster restore` to restore a new PostgreSQL cluster. 

     > ***Note:***
     > 
     > Only a newly created backup is supported. 
  - A new PostgreSQL cluster is created, and the `dataSource` of  PVC is assigned to backup set ID.
  - Wait until the cluster creation and restoration to complete, connect to the cluster with `kbcli cluster connect`, and verify the restored data.