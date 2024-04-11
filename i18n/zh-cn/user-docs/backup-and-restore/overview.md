---
title: 简介
description: KubeBlocks 备份恢复功能简介
keywords: [简介, 备份, 恢复]
sidebar_position: 1
sidebar_label: 简介
---

# 简介

KubeBlocks 提供备份恢复功能，以确保数据的安全性和可靠性。KubeBlocks 的备份恢复功能依赖于 BackupRepo，在使用完整的备份恢复功能之前，首先需要配置 [BackupRepo](../backup-and-restore/backup/backup-repo.md)。

KubeBlocks 采用物理备份的方式，将数据库中的物理文件作为备份对象。你可以根据实际需求，选择对应的方式按需或定时备份集群数据。

- [按需备份](../backup-and-restore/backup/on-demand-backup.md)：根据不同的备份选项，按需备份可以进一步分为备份工具备份和快照备份两种。
  - 备份工具备份：可使用数据库产品的备份工具，如 MySQL XtraBackup 和 PostgreSQL pg_basebackup。KubeBlocks 支持为不同的数据产品配置备份工具。
  - 快照备份：如果你的数据存储在支持快照的云盘中，你可以通过快照创建数据备份。快照备份通常比备份工具备份更快，因此推荐使用。
- [定时备份](../backup-and-restore/backup/scheduled-backup.md)：可指定保留时间、备份方法、时间等参数来自定义备份设置。

KubeBlocks 支持从备份集中恢复数据。

- 恢复

  - [从备份集中恢复数据](../backup-and-restore/restore/restore-data-from-backup-set.md)

可按照以下顺序对数据库集群进行备份和恢复操作。

1. [配置 BackupRepo](./backup/backup-repo.md).
2. [配置 BackupPolicy](./backup/configure-backup-policy.md).
3. [定时备份](./backup/scheduled-backup.md)或者[按需备份](./backup/on-demand-backup.md)集群数据。
4. 从备份集中[恢复集群数据](./restore/restore-data-from-backup-set.md)。
