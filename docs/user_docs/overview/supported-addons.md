---
title: Supported addons 
description: Addons supported by KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 4
sidebar_label: Supported addons 
---

# Supported addons

KubeBlocks, as a cloud-native data infrastructure based on Kubernetes, provides management and control for relational databases, NoSQL databases, vector databases, and stream computing systems; and these databases can be all added as addons. Besides databases, the KubeBlocks addon now also supports plugins for cloud environments and applications.

For installing and enabling Addons, refer to install Addons [by kbcli](./../installation/install-with-kbcli/install-addons.md) or [by Helm](./../installation/install-with-helm/install-addons.md).

| Addons          | Description                                                                                                                                                                                                       |
|:----------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apecloud-mysql  | ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability through the utilization of the RAFT consensus protocol.                                                          |
| apecloud-postgresql | ApeCloud PostgreSQL is a database that is compatible with PostgreSQL syntax and achieves high availability through the utilization of the RAFT consensus protocol. |
| camellia-redis-proxy | Camellia Redis Proxy is a high-performance redis proxy developed using Netty4. |
| clickhouse      | ClickHouse is a column-oriented database that enables its users to generate powerful analytics, using SQL queries, in real-time. |
| elasticsearch   | Elasticsearch is a distributed, RESTful search engine optimized for speed and relevance on production-scale workloads. |
| etcd            | etcd is a strongly consistent, distributed key-value store that provides a reliable way to store data that needs to be accessed by a distributed system or cluster of machines. |
| flink           | Apache Flink is a framework and distributed processing engine for stateful computations over unbounded and bounded data streams. |
| greatsql        | GreatSQL is a high performance open source relational database management system that can be used on common hardware for financial-grade application scenarios.  |
| greptimedb      | GreptimeDB is an open-source time-series database with a special focus on scalability, analytical capabilities and efficiency. |
| influxdb        | InfluxDB enables real-time analytics by serving as a purpose-built database that optimizes processing and scaling for large time series data workloads. |
| kafka           | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |
| mariadb         | MariaDB is a high performance open source relational database management system that is widely used for web and application servers. |
| milvus          | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| minio           | MinIO is an object storage solution that provides an Amazon Web Services S3-compatible API and supports all core S3 features. |
| mogdb           | MogDB is a stable and easy-to-use enterprise-ready relational database based on the openGauss open source database. |
| mongodb         | MongoDB is a document-oriented NoSQL database used for high volume data storage.                                                                                                                                  |
| mysql           | MySQL is a widely used, open-source relational database management system (RDBMS). |
| nebula          | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| neon            | Neon is Serverless Postgres built for the cloud. |
| oceanbase-ce    | Unlimited scalable distributed database for data-intensive transactional and real-time operational analytics workloads, with ultra-fast performance that has once achieved world records in the TPC-C benchmark test. OceanBase has served over 400 customers across the globe and has been supporting all mission critical systems in Alipay. |
| official-postgresql | An official PostgreSQL cluster definition Helm chart for Kubernetes. |
| opengauss       | openGauss is an open source relational database management system that is released with the Mulan PSL v2.  |
| openldap        | The OpenLDAP Project is a collaborative effort to develop a robust, commercial-grade, fully featured, and open source LDAP suite of applications and development tools. This chart provides KubeBlocks. |
| opensearch      | Open source distributed and RESTful search engine. |
| opentenbase     | OpenTenBase is an enterprise-level distributed HTAP open source database. |
| orchestrator	   | Orchestrator is a MySQL high availability and replication management tool, runs as a service and provides command line access, HTTP API and Web interface. |
| oriolebd        | OrioleDB is a new storage engine for PostgreSQL, bringing a modern approach to database capacity, capabilities and performance to the world's most-loved database platform. |
| polardb-x       | PolarDB-X is a cloud native distributed SQL Database designed for high concurrency, massive storage, complex querying scenarios. |
| postgresql      | PostgreSQL is an advanced, enterprise class open source relational database that supports both SQL (relational) and JSON (non-relational) querying.                                                            |
| pulsar          | Apache® Pulsar™ is an open-source, distributed messaging and streaming platform built for the cloud. |
| qdrant          | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| rabbitmq        | RabbitMQ is a reliable and mature messaging and streaming broker.  |
| redis           | Redis is a fast, open source, in-memory, key-value data store.                                                                                                                                                    |
| risingwave      | RisingWave is a distributed SQL database for stream processing. It is designed to reduce the complexity and cost of building real-time applications. |
| solr            | Solr is the popular, blazing-fast, open source enterprise search platform built on Apache Lucene. |
| starrocks-ce    | StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis. |
| tdengine        | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| tidb            | TiDB is an open-source, cloud-native, distributed, MySQL-Compatible database for elastic scale and real-time analytics. |
| victoria-metrics  | VictoriaMetrics is a fast, cost-effective and scalable monitoring solution and time series database. |
| weaviate        | Weaviate is an open-source vector database.                                                                                                                                                                       |
| xinference      | Xorbits Inference(Xinference) is a powerful and versatile library designed to serve language, speech recognition, and multimodal models. |
| yanshan         | YashanDB is a database management system developed by Shenzhen Institute of Computing Science. |
| zookeeper       | Apache ZooKeeper is a centralized service for maintaining configuration information, naming, providing distributed synchronization, and providing group services. |

## Supported functions of addons

:::note

- The versions listed below may not be up-to-date, and some supported versions might be missing. For the latest addon versions, please refer to the [KubeBlocks addon GitHub repo](https://github.com/apecloud/kubeblocks-addons).
- The upgrade feature means that KubeBlocks supports minor version upgrades for a database engine. For example, you can upgrade PostgreSQL from v12.14 to v12.15.

:::

| Addon (v0.9.0)     | version                                                    | Vscale           | Hscale           | Volumeexpand | Stop/Start       | Restart          | Expose | Backup/Restore | Logs | Config | Upgrade (DB engine version) | Account | Failover | Switchover | Monitor |
|:-------------------:|:---------------------------------------------------------:|:----------------:|:----------------:|:------------:|:----------------:|:----------------:|:------:|:--------------:|:----:|:------:|:---------------------------:|:-------:|:--------:|:----------:|:-------:|
| apecloud-mysql     | <p>apecloud-mysql-8.0.30</p><p>wescale-0.2.7</p>           | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      |✔️               | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | ✔️          | ✔️       |
| apecloud-postgresql | 14.11.0                                                   | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| camellia-redis-proxy |  1.2.26                                                  | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | N/A            | ✔️    | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| clickhouse          | 22.9.4                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| elasticsearch       | <p>7.10.1</p><p>7.7.1</p><p>7.8.1</p><p>8.1.3</p><p>8.8.2 </p>8.8.2   | ✔️    | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| etcd                | <p>3.5.15</p><p>3.5.6</p>                                | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| flink               | 1.16                                                      | ✔️                | ✔️ (task manager) | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| greptimedb          | 0.3.2                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| influxdb            | 2.7.4                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| kafka               | <p>kafka-broker-3.3.2</p><p>kafka-combine-3.3.2</p><p>kafka-controller-3.3.2</p><p>kafka-exporter-1.6.0</p> | ✔️ | ✔️ | ✔️ | ✔️       | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| mariadb             | 10.6.15                                                   | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| milvus              | 2.3.2                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| minio               | RELEASE.2024-06-29T01-20-47Z                              | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| mogdb               | 5.0.5                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | N/A     | N/A      | ✔️          | N/A     |
| mongodb      | <p>4.0.28</p><p>4.2.24</p><p>4.4.29</p><p>5.0.28</p><p>6.0.16</p><p>7.0.12</p>  | ✔️ | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | N/A     | ✔️        | ✔️          | ✔️       |
| mysql               | <p>5.7.44</p><p>8.0.33</p><p>8.4.2</p><p>2.4.4</p>        | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | ✔️          | ✔️       |
| nebula              | 3.5.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| neon | <p>neon-broker-1.0.0</p><p>neon-compute-1.0.0</p><p>neon-pageserver-1.0.0</p><p>neon-safekeeper-1.0.0</p>  | ✔️ | N/A | N/A    | N/A              | N/A              | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oceanbase           | 4.3.0                                                     | N/A              | ✔️                | ✔️            | N/A              | N/A              |        | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| official-postgresql | 14.7                                                      | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opengauss           | 5.0.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| openldap            | 2.4.57                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                |        | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opensearch          | 2.7.0                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opentenbase         | 2.5.0                                                     |                  |                  |              |                  |                  |        |                |      |        |                             |         |          |            |         |
| oracle              | 19.3.0-ee                                                 | ✔️                | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| orchestrator	       | 3.2.6                                                     |                  |                  |              |                  |                  |        |                |      |        |                             |         |          |            |         |
| orioledb            | 14.7.2-beta1                                              | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| polardb-x           | 2.3                                                       | ✔️                | ✔️                | N/A          | ✔️                | N/A              | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| postgresql | <p>12.14.0</p><p>12.14.1</p><p>12.15.0</p><p>14.7.2</p><p>14.8.0</p><p>15.7.0</p> | ✔️ | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | ✔️                           | ✔️       | ✔️        | ✔️          | ✔️       |
| pulsar | <p>pulsar-bkrecovery-2.11.2</p><p>pulsar-bkrecovery-3.0.2</p><p>pulsar-bookkeeper-2.11.2</p><p>pulsar-bookkeeper-3.0.2</p><p>pulsar-broker-2.11.2</p><p>pulsar-broker-3.0.2</p><p>pulsar-proxy-2.11.2</p><p>pulsar-proxy-3.0.2</p><p>pulsar-zookeeper-2.11.2</p><p>pulsar-zookeeper-3.0.2</p>  | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| qdrant  | <p>1.10.0</p><p>1.5.0</p><p>1.7.3</p><p>1.8.1</p><p>1.8.4</p>         | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | ✔️              | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| rabbitmq            | <p>3.13.2</p><p>3.12.14</p><p>3.11.28</p><p>3.10.25</p><p>3.9.29</p><p>3.8.14</p>    | ✔️   | ✔️  | ✔️            | ✔️                | ✔️                | ✔️      | N/A            | N/A  | N/A    | N/A                         | Managed by the RabitMQ Management system.     | ✔️      | ✔️        |✔️     |
| redis  | <p>redis-7.0.6</p><p>redis-7.2.4</p><p>redis-cluster-7.0.6</p><p>redis-cluster-7.2.4</p><p>redis-sentinel-7.0.6</p><p>redis-sentinel-7.2.4</p><p>redis-twemproxy-0.5.0</p>    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | N/A        | ✔️       |
| risingwave          | 1.0.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| solr                | 8.11.2                                                    | ✔️                | ✔️                | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| starrocks           | <p>starrocks-ce-be-3.2.2</p><p>starrocks-ce-be-3.3.0</p><p>starrocks-ce-fe-3.2.2</p><p>starrocks-ce-fe-3.3.0</p>    | ✔️      | ✔️   | ✔️  | ✔️    | ✔️        | N/A    | N/A       | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| tdengine            | 3.0.5                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| tidb   | <p>6.5.10</p><p>7.1.5</p><p>7.5.2</p><p>tidb-pd-6.5.10</p><p>tidb-pd-7.1.5</p><p>tidb-pd-7.5.2</p><p>tikv-6.5.10</p><p>tikv-7.1.5</p><p>tikv-7.5.2</p>  | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| victoria-metrics    | 1.0.0                                                     |                  |                  |              |                  |                  |        |                |      |        |                             |         |          |            |         |
| weaviate            | 1.23.1                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| xinference          | <p>xinference-0.11.0</p><p>xinference-0.11.0-cpu</p>      | ✔️                | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| yashan              | personal-23.1.1.100                                       | ✔️                | ✔️ (Standalone)   | ✔️            | ✔️                | ✔️                | N/A    | N/A            | ✔️    | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |
| zookeeper | <p>3.4.14</p><p>3.6.4</p><p>3.7.2</p><p>3.8.4</p><p>3.9.2</p>       | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | ✔️    | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |
