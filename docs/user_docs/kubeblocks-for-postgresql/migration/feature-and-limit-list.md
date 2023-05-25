---
title: Full feature and limit list
description: The full feature and limit list of KubeBlocks migration function for PostgreSQL
keywords: [postgresql, migration, migrate data in PostgreSQL to KubeBlocks, full feature, limit]
sidebar_position: 1
sidebar_label: Full feature and limit list
---

# Full feature and limit list

## Full feature list

* Precheck
  * Database connection
  * Database version
  * Whether the incremental migration is supported by a database
  * The existence of the table structure
  * Whether the table structure of the source database is supported
* Structure initialization
  * PostgreSQL
    * Table Struct
    * Table Constraint
    * Table Index
    * Table Comment
    * Table Sequence
* Data initialization
  * Supports all major data types
* Incremental data migration
  * Supports all major data types
  * Support the resumable upload capability of eventual consistency

## Limit list

* Overall limits
  * If the incremental data migration is used, the source database should enable CDC (Change Data Capture) related configurations (both are checked and blocked in precheck). For detailed configurations, see [Configure the source database](#configure-the-source-database).
  * A table without a primary key is not supported. And a table with a foreign key is not supported (both are checked and blocked in precheck).
  * Except for the incremental data migration module, other modules do not support resumable upload, i.e. if an exception occurs in this module, such as pod failure caused by downtime and network disconnection, a re-migration is required.
  * During the data transmission task, DDL on the migration objects in the source database is not supported.
  * The table name and field name cannot contain Chinese characters and special characters like a single quotation mark (') and a comma (,).
  * During the migration process, the PrimarySecondary switchover in the source library is not supported, which may cause the connection string specified in the task configuration to change. This further causes the migration link failure.
* Permission limits
  * The source account
    * LOGIN
    * The read permission of the source migration objects
    * REPLICATION
  * The sink account
    * LOGIN
    * The read/write permission of the sink database
* Precheck module: None
* Init-struct module
  * The Array data type is not supported, such as text[], text[3][3], integer[].
  * The user-defined type is not supported.
  * The database character set other than UTF-8 is not supported.
* Init-data module
  * Character sets of the source and sink databases should be the same.
* Data incremental migration module
  * Character sets of the source and sink databases should be the same.
