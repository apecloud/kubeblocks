---
title: Full feature and limit list
description: The full feature and limit list of KubeBlocks migration function for MongoDB
keywords: [mongodb, migration, migrate data in MongoDB to KubeBlocks, full feature, limit]
sidebar_position: 1
sidebar_label: Full feature and limit list
---

# Full feature and limit list

## Full feature list

* Precheck
  * Database connection
  * Database version
  * Whether the incremental migration is supported by a database
  * Whether the table structure of the source database is supported
* Data initialization
  * Supports all major data types
* Incremental data migration
  * Supports all major data types
  * Support the resumable upload capability of eventual consistency

## Limit list

* Overall limits
  * If the incremental data migration is used, the source database should be the master node under the Replica Set structure.
  * Except for the incremental data migration module, other modules do not support resumable upload, i.e. if an exception occurs in this module, such as pod failure caused by downtime and network disconnection, a re-migration is required.
  * During the data transmission task, operations such as Drop, Rename, and DropDatabase on the migration objects in the source database is not supported.
  * The database name and collection name cannot contain Chinese characters and special characters like a single quotation mark (') and a comma (,).
  * During the migration process, the switchover of primary and secondary nodes in the source library is not supported, which may cause the connection string specified in the task configuration to change. This further leads to migration link failure.
* Precheck module: None
* Data initialization module
  * The database character set other than UTF-8 is not supported.
* Data incremental migration module
  * The database character set other than UTF-8 is not supported.
