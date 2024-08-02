---
title: PITR
description: How to perform PITR
keywords: [backup and restore, restore, PITR, postgresql]
sidebar_position: 2
sidebar_label: PITR
---

# PITR

## What is PITR (Point-in-Time Recovery)?

PITR (Point-in-Time Recovery) is a database backup and recovery technique commonly used in Relational Database Management Systems (RDBMS). It allows for the recovery of data changes to a specific point in time, restoring the database to a state prior to that point. In PITR, the database system regularly creates full backups and logs all transactions thereafter, including insert, update, and delete operations. During recovery, the system first restores the most recent full backup, and then applies the transaction logs recorded after the backup, bringing the database back to the desired state.

KubeBlocks supports PITR for databases such as MySQL and PostgreSQL. This documentation takes PostgreSQL PITR as an example.

## How to perform PITR?

1. View the timestamps to which the cluster can be restored.

    ```bash
    kbcli cluster describe pg-cluster
    ...
    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   RECOVERABLE-TIME                                                
    minio         Enabled       */5 * * * *       archive-wal     8d                 May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+0800
    ```

    `RECOVERABLE-TIME` represents the time range within which the cluster can be restored.

    It can be seen that the current backup time range is `May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+0800`. Still, a full backup is required for data restoration, and this full backup must be completed within the time range of the log backups.

2. Restore the cluster to a specific point in time.

    ```bash
    kbcli cluster restore pg-cluster-pitr --restore-to-time 'May 07,2024 15:48:47 UTC+0800' --backup <continuousBackupName>
    ```

3. Check the status of the new cluster.

    ```bash
    kbcli cluster list pg-cluster-pitr
    ```

    Once the status turns to `Running`, it indicates a successful operation.
