---
title: Supported add-ons 
description: add-ons supported by KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 2
sidebar_label: Supported add-ons 
---

# Supported add-ons

KubeBlocks, as a cloud-native data infrastructure based on Kubernetes, providing management and control for relational databases, NoSQL databases, vector databases, and stream computing systems; and these databases can be all added as addons.

| Add-ons         | Description                                                                                                                                                                                                       |
|:----------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apecloud-mysql  | ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability through the utilization of the RAFT consensus protocol.                                                          |
| elasticsearch   | Elasticsearch is a distributed, RESTful search engine optimized for speed and relevance on production-scale workloads. |
| greptimedb      | GreptimeDB is an open-source time-series database with a special focus on scalability, analytical capabilities and efficiency. |
| kafka           | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |
| mongodb         | MongoDB is a document-oriented NoSQL database used for high volume data storage.                                                                                                                                  |
| milvus          | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| nebula          | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| neon            | Neon is Serverless Postgres built for the cloud. |
| pika            | Pika is a persistent huge storage service, compatible with the vast majority of redis interfaces, including string, hash, list, zset, set and management interfaces. |
| postgresql      | PostgreSQL is an advanced, enterprise class open source relational database that supports both SQL (relational) and JSON (non-relational) querying.                                                            |
| pulsar          | Apache® Pulsar™ is an open-source, distributed messaging and streaming platform built for the cloud. |
| qdrant          | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| redis           | Redis is a fast, open source, in-memory, key-value data store.                                                                                                                                                    |
| risingwave      | RisingWave is a distributed SQL database for stream processing. It is designed to reduce the complexity and cost of building real-time applications. |
| starrocks       | StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis. |
| tdengine        | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| weaviate        | Weaviate is an open-source vector database.                                                                                                                                                                       |

## Supported functions of add-ons

| Add-ons         | Vertical Scaling  | Horizontal Scaling  | Volume Expansion  | Stop/Start  | Restart  | Backup/Restore  | Logs  | Config   | Upgrade  | Account  | Failover  | Switchover  | Monitor  |
|:----------------|:------------------|:--------------------|:------------------|:------------|:---------|:----------------|:------|:---------|:---------|:---------|:----------|:------------|:---------|
| apecloud-mysql  | ✔                 | ✔                   | ✔                 | ✔           | ✔        | ✔               | ✔     | ✔        |          | ✔        | ✔         | ✔           | ✔        |
| elasticsearch   | ✔                 | ✔                   | ✔                 | ✔           | ✔        |                 | ✔     |          |          |          |           |             |          |
| greptimedb      | ✔                 | ✔ hscale <br /> frontend, datanode  | ✔                | ✔          | ✔       |                | ✔    |          |          |         |          |            |         |
| kafka           | ✔                 | ✔ hscale out <br /> broker   | ✔                | ✔          | ✔       |                | ✔    | ✔       |         |         |          |             | ✔        |
| milvus          | ✔                 | ✔                   | ✔                 | ✔           | ✔        |                 | ✔     |          |          |          |           |             |          |
| mongodb         | ✔                 | ✔                   | ✔                 | ✔           | ✔        | ✔               | ✔     | ✔        |          |          | ✔         | ✔           | ✔        |
| nebula          | ✔                 | ✔ hscale <br /> nebula-console, nebula-graphd, nebula-metad, nebula-storaged | ✔                | ✔          | ✔       |                | ✔    |         |         |         |          |            |         |
| neon            | ✔                 |                     |                   |             |          |                 |       |          |          |          |           |             |          |
| pika            |
| postgresql      | ✔                 | ✔                   | ✔                 | ✔           | ✔        | ✔               | ✔     | ✔        | ✔        | ✔        | ✔         | ✔           | ✔        |
| pulsar          | ✔                 | ✔                   | ✔                 | ✔           | ✔        |                 | ✔     | ✔        |          |          |           |             | ✔        |
| qdrant          | ✔                 | ✔ hscale out        | ✔                 | ✔           | ✔        | ✔               | ✔     |          |          |          |           |             | ✔        |
| redis           | ✔                 | ✔                   | ✔                 | ✔           | ✔        | ✔               | ✔     | ✔        |          | ✔        | ✔         |             | ✔        |
| risingwave      | ✔                 | ✔ hscale <br /> frontend, compute | ✔                 | ✔          | ✔       |                | ✔    |         |         |         |          |            |         |
| starrocks       | ✔                 | ✔ hscale be         | ✔                 | ✔           | ✔        |                 | ✔     |          |          |          |           |             |          |
| tdengine        | ✔                 | ✔                   | ✔                 | ✔           | ✔        |                 | ✔     |          |          |          |           |             |          |
| weaviate        | ✔                 | ✔ hscale out        | ✔                 | ✔           | ✔        |                 | ✔     | ✔        |          |          |           |             | ✔        |

## Use add-ons

### List add-ons

To list supported add-ons, run `kbcli addon list` command.

### Enable/Disable add-ons

To manually enable or disable add-ons, follow the steps below.

***Steps:***

1. To enable the add-on, use `kbcli addon enable`.

   ***Example***

   ```bash
   kbcli addon enable snapshot-controller
   ```

   To disable the add-on, use `kbcli addon disable`.

2. List the add-ons again to check whether it is enabled.

   ```bash
   kbcli addon list
   ```
