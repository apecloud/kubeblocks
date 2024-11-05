---
title: PITR
description: PITR
keywords: [备份恢复, 恢复, PITR, 按时间点恢复]
sidebar_position: 2
sidebar_label: PITR
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# PITR

### PITR 是什么？

PITR（Point-in-Time Recovery，时间点恢复）是一种数据库备份恢复技术，通常用于关系型数据库管理系统（RDBMS）。它可以恢复特定时间点开始的数据更改，将数据库回退到特定时间点之前的状态。在 PITR 中，数据库系统会定期创建全量备份，并记录备份点之后所有的事务日志，包括插入、更新和删除操作等。当恢复时，系统会首先还原最近的全量备份，然后应用备份之后的事务日志，将数据库恢复到所需的状态。

KubeBlocks 已支持对 MySQL 和 PostgreSQL 等数据库的 PITR 功能。

## 如何进行 PITR？

1. 查看可以将集群恢复到的时间戳。

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster describe pg-cluster
    ...
    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   RECOVERABLE-TIME                                                
    minio         Enabled       */5 * * * *       archive-wal     8d                 May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+080
    ```

    </TabItem>

    <TabItem value="kubectl" label="kubectl">

    ```bash
    # 1. 获取当前集群的全部备份对象（backup objects）
    kubectl get backup -l app.kubernetes.io/instance=pg-cluster
    
    # 2. 获取持续备份的备份实践范围
    kubectl get backup -l app.kubernetes.io/instance=pg-cluster -l dataprotection.kubeblocks.io/backup-type=Continuous -o yaml
    ...
    status:
        timeRange:
        end: "2024-05-07T10:47:14Z"
        start: "2024-05-07T10:07:45Z"
    ```

    </TabItem>

    </Tabs>

    `RECOVERABLE-TIME` 表示可以将恢复集群到的时间范围。

    可以看到当前持续日志备份的时间范围是 `May 07,2024 15:29:46 UTC+0800 ~ May 07,2024 15:48:47 UTC+0800`。但是还得需要一个基础全量备份才能恢复数据，并且这个全部备份完成时间需要落在日志备份的时间范围内才是有效的基础备份。

2. 将集群恢复到指定的时间点。

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster restore pg-cluster-pitr --restore-to-time 'May 07,2024 15:48:47 UTC+080' --backup <continuousBackupName>
    ```

    </TabItem>

    <TabItem value="kubectl" label="kubectl">

    ```bash
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

    </TabItem>

    </Tabs>

3. 查看新集群的状态。

    集群状态为 `Running` 时，表示恢复成功。

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster list pg-cluster-pitr
    >
    NAME                 NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
    pg-cluster-pitr      default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Jul 25,2023 19:42 UTC+0800
    ```

    </TabItem>

    <TabItem value="kubectl" label="kubectl">

    ```bash
    kubectl get cluster pg-cluster-pitr
    ```

    </TabItem>

    </Tabs>
