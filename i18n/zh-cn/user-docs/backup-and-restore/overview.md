---
title: 简介
description: KubeBlocks 备份恢复功能简介
keywords: [简介, 备份, 恢复]
sidebar_position: 1
sidebar_label: 简介
---

# 简介
KubeBlocks 提供备份恢复功能，以确保数据的安全性和可靠性。KubeBlocks 的备份恢复功能依赖于 BackupRepo，在使用完整的备份恢复功能之前，首先需要配置 [BackupRepo](../backup-and-restore/backup/backup-repo.md)。

KubeBlocks 采用物理备份的方式，将数据库中的物理文件作为备份对象。

- [按需备份](../backup-and-restore/backup/on-demand-backup.md)：按需备份仅包含完整数据，也称为全量备份。根据不同的备份选项，按需备份可以进一步分为备份工具备份和快照备份两种。
  - 备份工具备份：可使用数据库产品的备份工具，如 MySQL XtraBackup 和 PostgreSQL pg_basebackup。KubeBlocks 支持为不同的数据产品配置备份工具。
  - 快照备份：如果你的数据存储在支持快照的云盘中，你可以通过快照创建数据备份。快照备份通常比备份工具备份更快，因此推荐使用。
- [定时备份](../backup-and-restore/backup/scheduled-backup.md)：可指定保留时间、备份方法、时间等参数来[自定义备份](../backup-and-restore/backup/scheduled-backup.md)。或者你可以通过[日志自动备份](../backup-and-restore/backup/scheduled-backup.md)。日志备份使用备份数据库生成的增量日志文件，如 MySQL BinLog 和 PostgreSQL WAL，因此也被称为增量备份。基于时间点的恢复（PITR）依赖于日志备份。

至于恢复功能，KubeBlocks 支持从备份集中恢复数据。
- 恢复
  - [从备份集中恢复数据](../backup-and-restore/restore/restore-data-from-backup-set.md)

不同数据库引擎的备份恢复功能有所不同：

引擎         | 定时备份 | 实时备份  | 快照备份 | 数据文件备份 | 日志备份 | 全量备份 | PITR | 数据压缩 |
:-----         | :--------------- | :--------------- | :-------------- | :----------     | :--------- | :---------- | :--- | :--------------- |
PostgreSQL     | ✅               | ✅                | ✅              | ✅              | ✅         | ✅           | ✅   | ✅               |
ApeCloud MySQL | ✅               | ❌                | ✅              | ✅              | ✅         | ✅           | ✅   | ✅               |
MongoDB        | ✅               | ✅                | ✅              | ✅              | ✅         | ✅           | ✅   | ✅               |
Redis          | ✅               | ❌                | ✅              | ✅              | ❌         | ✅           | ❌   | ✅               |
Prometheus     | ✅               | ❌                | ✅              | ❌              | ❌         | ✅           | ❌   | ❌               |
Nebula         | ✅               | ❌                | ✅              | ❌              | ❌         | ✅           | ❌   | ❌               |
Milvus         | ✅               | ❌                | ✅              | ❌              | ❌         | ✅           | ❌   | ❌               |
Qdrant         | ✅               | ❌                | ✅              | ❌              | ❌         | ✅           | ❌   | ❌               |
Weaviate       | ✅               | ❌                | ✅              | ❌              | ❌         | ✅           | ❌   | ❌               |

