---
title: PITR
description: How to perform PITR
keywords: [backup and restore, restore, PITR]
sidebar_position: 2
sidebar_label: PITR
---

# PITR

## What is PITR?

PITR is a database backup and restore technique commonly used in relational database management systems (RDBMS). It allows to restore data changes starting from a specific point in time, rolling back the database to a state prior to that specific point. In PITR, the database system regularly creates full backups and maintains a record of all transaction logs occurring after each backup point, including insertions, updates, deletions, and other operations. During recovery, the system first restores the most recent full backup and then applies the transaction logs recorded after the backup to bring the database to the desired state.

KubeBlocks supports PITR for databases such as MySQL and PostgreSQL. This documentation takes PostgreSQL PITR as an example.

## How to perform PITR?

1. View the timestamps to which the cluster can be restored.
   
    ```powershell
    # 1. Get all backup objects for the current cluster
    kubectl get backup -l app.kubernetes.io/instance=pg-cluster
    
    # 2. Get the backup time range for Continuous Backup
    kubectl get backup -l app.kubernetes.io/instance=pg-cluster -l dataprotection.kubeblocks.io/backup-type=Continuous -oyaml
    ...
    status:
        timeRange:
        end: "2024-05-07T10:47:14Z"
        start: "2024-05-07T10:07:45Z"
    ```

    It can be seen that the current backup time range is `2024-05-07T10:07:45Z ~2024-05-07T10:47:14Z`. Still, a full backup is required for data restoration, and this full backup must be completed within the time range of the log backups.


2. Restore the cluster to a specific point in time.

    ```powershell
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
    name:  pg-cluster-pitr
    spec:
    clusterName:  pg-cluster-pitr
    restore:
        backupName: 818aa0e0-pg-kubeblocks-cloud-n-archive-wal
        restorePointInTime: "2024-05-07T10:07:45Z"
        volumeRestorePolicy: Parallel
    type: Restore
    ```

3. Check the status of the new cluster.

    ```powershell
    kubectl get cluster pg-cluster-pitr
    ```

    Once the status turns to `Running`, it indicates a successful operation.