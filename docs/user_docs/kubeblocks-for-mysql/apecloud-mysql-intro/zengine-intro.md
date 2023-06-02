---
title: ZEngine introduction
description: What is ZEngine?
keywords: [zengine, introduction]
sidebar_position: 2
---

# ZEngine introduction

ZEngine is a MySQL transaction storage engine developed based on the LSM-tree architecture.

The main features of ZEngine technology are as follows:

- It uses typical LSM-tree architecture, which is divided into the incremental modification part in memory and the full data part on disk.
- Writing uses asynchronous transaction pipeline technology, which avoids a large number of synchronous operations during transaction processing and has higher write throughput.
- Compaction is based on the extent/page unit, which makes it highly concurrent in small tasks and reduces the impact on the foreground. Compaction supports extent/page granularity data reuse to avoid data migration and copying.
- The data on disk has only two layers (L1 hot layer and L2 cold layer) in normal state, which is intended to improve read performance. The disk files are managed with granularity of FILE (1GB)/Extent (2MB)/Page (16KB), and data is stored with default compression (3-5 times compression ratio). The space can automatically expand and shrink.
- Fine-grained cold-hot data management supports proactive cache preheating after flush/compaction and supports compaction scheduling based on cold-hot characteristics.
- It is seamlessly integrated with the MySQL system. It supports the Chinese character set, Online/Instant/Parallel DDL, and xtrabackup physical backup.
