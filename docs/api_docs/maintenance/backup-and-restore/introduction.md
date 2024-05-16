---
title: Introduction
description: Introduction of KubeBlocks backup and restore functions
keywords: [introduction, backup, restore]
sidebar_position: 1
sidebar_label: Introduction
---

# Introduction

KubeBlocks provides the backup and restore function to ensure the safety and reliability of your data. The backup and restore function of KubeBlocks relies on BackupRepo and before using the full backup and restore function, you need to [configure BackupRepo first](./backup/backup-repo.md).

KubeBlocks adopts physical backup which takes the physical files in a database as the backup object. You can choose one backup option based on your demands to back up the cluster data on demand or by schedule.

* [On-demand backup](./backup/on-demand-backup.md): Based on different backup options, on-demand backup can be further divided into backup tool and snapshot backup.
  * Backup tool: You can use the backup tool of the corresponding data product, such as MySQL XtraBackup and PostgreSQL pg_basebackup. KubeBlocks supports configuring backup tools for different data products.
  * Snapshot backup: If your data is stored in a cloud disk that supports snapshots, you can create a data backup by snapshots. Snapshot backup is usually faster than a backup tool, and thus is recommended.

* [Scheduled backup](./backup/scheduled-backup.md): You can specify retention time, backup method, time, and other parameters to customize your backup schedule.

As for the restore function, KubeBlocks supports restoring data from the backup set.

* Restore
  * [PITR](./restore/pitr.md).
  * [Restore data from the backup set](./restore/restore-data-from-backup-set.md).

Follow the steps below to back up and restore your cluster.

1. [Configure BackupRepo](./backup/backup-repo.md).
2. [Configure BackupPolicy](./backup/configure-backuppolicy.md).
3. Backup your cluster [on demand](./backup/on-demand-backup.md) or [by schedule](./backup/scheduled-backup.md).
4. Restore your data by [PITR](./restore/pitr.md) or from the [backup set](./restore/restore-data-from-backup-set.md).
