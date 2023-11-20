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
| clickhouse      |
| elasticsearch   | Elasticsearch is a distributed, RESTful search engine optimized for speed and relevance on production-scale workloads. |
| etcd            | etcd is a strongly consistent, distributed key-value store that provides a reliable way to store data that needs to be accessed by a distributed system or cluster of machines. |
| foxlake         | ApeCloud FoxLake is an open-source cloud-native data warehouse. |
| ggml            | GGML is a tensor library for machine learning to enable large models and high performance on commodity hardware.  |
| greptimedb      | GreptimeDB is an open-source time-series database with a special focus on scalability, analytical capabilities and efficiency. |
| kafka           | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |
| mariadb         | MariaDB is a high performance open source relational database management system that is widely used for web and application servers. |
| milvus          | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| mongodb         | MongoDB is a document-oriented NoSQL database used for high volume data storage.                                                                                                                                  |
| mysql  (Primary-Secondary replication) |
| nebula          | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| neon            | Neon is Serverless Postgres built for the cloud. |
| oceanbase       | Unlimited scalable distributed database for data-intensive transactional and real-time operational analytics workloads, with ultra-fast performance that has once achieved world records in the TPC-C benchmark test. OceanBase has served over 400 customers across the globe and has been supporting all mission critical systems in Alipay. |
| official-postgresql | An official PostgreSQL cluster definition Helm chart for Kubernetes. |
| openldap        | The OpenLDAP Project is a collaborative effort to develop a robust, commercial-grade, fully featured, and open source LDAP suite of applications and development tools. This chart provides KubeBlocks. |
| opensearch      | Open source distributed and RESTful search engine. |
| oracle-mysql    | MySQL is a widely used, open-source relational database management system (RDBMS). |
| oriolebd        | OrioleDB is a new storage engine for PostgreSQL, bringing a modern approach to database capacity, capabilities and performance to the world's most-loved database platform. |
| pika            | Pika is a persistent huge storage service, compatible with the vast majority of redis interfaces, including string, hash, list, zset, set and management interfaces. |
| polardb-x       | PolarDB-X is a cloud native distributed SQL Database designed for high concurrency, massive storage, complex querying scenarios. |
| postgresql      | PostgreSQL is an advanced, enterprise class open source relational database that supports both SQL (relational) and JSON (non-relational) querying.                                                            |
| pulsar          | Apache® Pulsar™ is an open-source, distributed messaging and streaming platform built for the cloud. |
| qdrant          | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| redis           | Redis is a fast, open source, in-memory, key-value data store.                                                                                                                                                    |
| risingwave      | RisingWave is a distributed SQL database for stream processing. It is designed to reduce the complexity and cost of building real-time applications. |
| starrocks       | StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis. |
| tdengine        | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| vllm            | vLLM is a fast and easy-to-use library for LLM inference and serving. |
| weaviate        | Weaviate is an open-source vector database.                                                                                                                                                                       |
| zookeeper       | Apache ZooKeeper is a centralized service for maintaining configuration information, naming, providing distributed synchronization, and providing group services. |

## Supported functions of add-ons

| Add-on (v0.7.0)                       | version                           | Vscale | Hscale | Volumeexpand | Stop/Start | Restart | Backup/Restore | Logs | Config | Upgrade (DB engine version) | Account | Failover | Switchover | Monitor |
|---------------------------------------|-----------------------------------|--------|--------|--------------|------------|---------|----------------|------|--------|-----------------------------|---------|----------|------------|---------|
| apecloud-mysql                        | 8.0.30                            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | ✔️          | ✔️       |
| clickhouse                            | 22.9.4                            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| elasticsearch                         | 8.8.2                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| etcd                                  | 3.5.6                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| foxlake                               | 0.2.0                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| ggml                                  | N/A                               |        | N/A    | N/A          | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| greptimedb                            | 0.3.2                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| kafka                                 | 3.3.2                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| mariadb                               | 10.6.15                           | ✔️      | N/A    | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| milvus                                | 2.2.4                             | ✔️      | N/A    | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| mongodb                               | 4.0<br />4.2<br />4.4<br />5.0<br />5.0.20<br />6.0<br />sharding-5.0 | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | ✔️              | ✔️    | ✔️      | N/A                         | N/A     | ✔️        | ✔️          | ✔️       |
| mysql (Primary-Secondary replication) | 5.7.42<br />8.0.33                      | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| nebula                                | 3.5.0                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| neon                                  | latest                            | ✔️      | N/A    | N/A          | N/A        | N/A     | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oceanbase                             | 4.2.0.0-100010032023083021        |        | ✔️      | ✔️            | N/A        | N/A     | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| official-postgresql                   | 12.15<br />14.7<br />14.7-zhparser            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| openldap                              | 2.4.57                            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opensearch                            | 2.7.0                             | ✔️      | N/A    | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oracle-mysql                          | 8.0.32<br />8.0.32-perf                 | ✔️      | N/A    | ✔️            | ✔️          | ✔️       | ✔️              | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |
| orioledb                              | beta1                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| polardb-x                             | 2.3                               | ✔️      | ✔️      | N/A          | ✔️          | N/A     | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| postgresql                            | 12.14.0<br />12.14.1<br />12.15.0<br />14.7.2<br />14.8.0 | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | ✔️              | ✔️    | ✔️      | ✔️                           | ✔️       | ✔️        | ✔️          | ✔️       |
| pulsar                                | 2.11.2                            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| qdrant                                | 1.5.0                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | ✔️              | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| redis                                 | 7.0.6                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | N/A        | ✔️       |
| risingwave                            | 1.0.0                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| starrocks                             | 3.1.1                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| tdengine                              | 3.0.5.0                           | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| vllm                                  | N/A                               | N/A    | N/A    | N/A          | ✔️          | ✔️       | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| weaviate                              | 1.18.0                            | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| zookeeper                             | 3.7.1                             | ✔️      | ✔️      | ✔️            | ✔️          | ✔️       | N/A            | ✔️    | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |

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
