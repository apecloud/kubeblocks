---
title: PITR
description: How to implement point-in-time recovery of data
keywords: [backup and restore, pitr]
sidebar_position: 7
sidebar_label: PITR
---

# PITR

1. View the timestamp which the cluster can be restored to.

   ```bash
   kbcli cluster describe mysql-cluster
   ...
   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE       BACKUP-TTL   LAST-SCHEDULE                RECOVERABLE-TIME
   Enabled       0 18 * * *        datafile   7d           Jul 25,2023 19:36 UTC+0800   Jul 25,2023 14:53:00 UTC+0800 ~ Jul 25,2023 19:07:38 UTC+0800
   ```

   `RECOVERABLE-TIME` stands for the time range to which the cluster can be restored.

2. Run the command below to restore the cluster to the specified point.

   ```bash
   kbcli cluster restore mysql-cluster-pitr --restore-to-time 'Jul 25,2023 18:52:53 UTC+0800' --source-cluster mysql-cluster
   ```

3. View the status of the new cluster.

   The status shows `Running` and it means restore is successful.

   ```bash
   kbcli cluster list mysql-cluster-pitr
   >
   NAME                 NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mysql-cluster-pitr   default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Jul 25,2023 19:42 UTC+0800
   ```
