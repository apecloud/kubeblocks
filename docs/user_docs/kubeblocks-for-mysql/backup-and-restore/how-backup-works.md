---
title: How backup works
description: How ApeCloud MySQL backup works
sidebar_position: 1
---

# How backup works
KubeBlocks integrates cloud-native backup and restore solutions, and currently supports MySQL backup and recovery by snapshots.
- Backup 
  - Install `CSI driver` that supports the snapshots function and the Kubeblocks native plug-in `snapshot-controller` which is integrated in kbcli in your Kubernetes environment.
  - Initialize all the plug-ins and choose a MySQL cluster. And then create a backup with kbcli. Kbcli creates a `BackupPolicy` and a `Backup` object and associates them with the MySQL cluster by `Labels`.
  - Once a MySQL cluster has backup executed, `BackupPolicy` configures a `CronJob` automatically to  backup the cluster periodically. 
  - Check the Backup object with kbcli, when the status is Completed, the backup is done.

- Restore
  - Check the backup list with `kbcli cluster list-backups` to choose a backup name that is completed.
  - Use `kbcli cluster restore` to restore a new MySQL cluster. 

     > ***Note:***
     > 
     > Only a newly created backup is supported. 
  - A new MySQL cluster is created, and the `dataSource` of  PVC is assigned to backup set ID.
  - Wait until the cluster creation and restoration to complete, connect to the cluster with `kbcli cluster connect`, and verify the restored data.