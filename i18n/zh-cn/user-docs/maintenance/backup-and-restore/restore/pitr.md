---
title: PITR
description: PITR
keywords: [备份恢复, 恢复, PITR, 按时间点恢复]
sidebar_position: 1
sidebar_label: PITR
---

# PITR

### PITR 是什么？

PITR（Point-in-Time Recovery，时间点恢复）是一种数据库备份恢复技术，通常用于关系型数据库管理系统（RDBMS）。它可以恢复特定时间点开始的数据更改，将数据库回退到特定时间点之前的状态。在 PITR 中，数据库系统会定期创建全量备份，并记录备份点之后所有的事务日志，包括插入、更新和删除操作等。当恢复时，系统会首先还原最近的全量备份，然后应用备份之后的事务日志，将数据库恢复到所需的状态。

KubeBlocks 已支持对 MySQL 和 PostgreSQL 等数据库的 PITR 功能。

## 如何进行 PITR？

1. 查看可以将集群恢复到的时间戳。

    ```bash
    kbcli cluster describe pg-cluster
    ...
    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   RECOVERABLE-TIME                                                
    minio         Enabled       */5 * * * *       archive-wal     8d                 May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+080
    ```

`RECOVERABLE-TIME` 表示可以将恢复集群到的时间范围。

可以看到当前持续日志备份的时间范围是 `May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+0800`。但是还得需要一个基础全量备份才能恢复数据，并且这个全部备份完成时间需要落在日志备份的时间范围内才是有效的基础备份。

2. 将集群恢复到指定的时间点。

    ```bash
    kbcli cluster restore pg-cluster-pitr --restore-to-time 'May 07,2024 15:48:47 UTC+080' --backup <continuousBackupName>
    ```

3. 查看新集群的状态。

    集群状态为 `Running` 时，表示恢复成功。

    ```bash
    kbcli cluster list pg-cluster-pitr
    >
    NAME                 NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
    pg-cluster-pitr      default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Jul 25,2023 19:42 UTC+0800
    ```
