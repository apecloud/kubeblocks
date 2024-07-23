---
title: PITR
description: 如何对集群进行时间点恢复
keywords: [备份恢复, 恢复, PITR, 时间点恢复]
sidebar_position: 2
sidebar_label: PITR
---

# PITR

## 什么是 PITR?

PITR（Point-in-Time Recovery，时间点恢复）是一种数据库备份恢复技术，通常用于关系型数据库管理系统（RDBMS）。它可以恢复特定时间点开始的数据更改，将数据库回退到特定时间点之前的状态。在 PITR 中，数据库系统会定期创建全量备份，并记录备份点之后所有的事务日志，包括插入、更新和删除操作等。当恢复时，系统会首先还原最近的全量备份，然后应用备份之后的事务日志，将数据库恢复到所需的状态。

KubeBlocks 已支持对 MySQL 和 PostgreSQL 等数据库的 PITR 功能。本章节以 PostgreSQL PITR 为例。

## 如何进行 PITR？

1. 查看可以将集群恢复到的时间戳。

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

    可以看到当前持续日志备份的时间范围是 `2024-05-07T10:07:45Z ~2024-05-07T10:47:14Z`。但是还得需要一个基础全量备份才能恢复数据，并且这个全部备份完成时间需要落在日志备份的时间范围内才是有效的基础备份。

2. 将集群恢复到指定的时间点。

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

3. 查看新集群的状态。

    ```powershell
    kubectl get cluster pg-cluster-pitr
    ```

    集群状态为 Running 时，表示恢复成功。
