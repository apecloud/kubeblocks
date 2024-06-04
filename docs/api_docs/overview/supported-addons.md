---
title: Supported addons 
description: Addons supported by KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 4
sidebar_label: Supported addons 
---

# Supported add-ons

KubeBlocks, as a cloud-native data infrastructure based on Kubernetes, provides management and control for relational databases, NoSQL databases, vector databases, and stream computing systems; and these databases can be all added as addons. Besides databases, KubeBlocks addon now also supports plugins for cloud environments and applications.

| Addons          | Description                                                                                                                                                                                                       |
|:----------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apecloud-mysql  | ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability through the utilization of the RAFT consensus protocol.                                                          |
| apecloud-postgresql | ApeCloud PostgreSQL is a database that is compatible with PostgreSQL syntax and achieves high availability through the utilization of the RAFT consensus protocol. |
| camellia-redis-proxy | Camellia Redis Proxy is a high-performance redis proxy developed using Netty4. |
| clickhouse      | ClickHouse is a column-oriented database that enables its users to generate powerful analytics, using SQL queries, in real-time. |
| doris           | Apache Doris is a modern data warehouse for real-time analytics. It delivers lightning-fast analytics on real-time data at scale. |
| elasticsearch   | Elasticsearch is a distributed, RESTful search engine optimized for speed and relevance on production-scale workloads. |
| etcd            | etcd is a strongly consistent, distributed key-value store that provides a reliable way to store data that needs to be accessed by a distributed system or cluster of machines. |
| flink           | Apache Flink is a framework and distributed processing engine for stateful computations over unbounded and bounded data streams. |
| foxlake         | ApeCloud FoxLake is an open-source cloud-native data warehouse. |
| ggml            | GGML is a tensor library for machine learning to enable large models and high performance on commodity hardware.  |
| greptimedb      | GreptimeDB is an open-source time-series database with a special focus on scalability, analytical capabilities and efficiency. |
| halo            | HaloDB is a fast and simple embedded key-value store written in Java. |
| influxdb        | InfluxDB enables real-time analytics by serving as a purpose-built database that optimizes processing and scaling for large time series data workloads. |
| kafka           | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |
| mariadb         | MariaDB is a high performance open source relational database management system that is widely used for web and application servers. |
| milvus          | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| minio           | MinIO is an object storage solution that provides an Amazon Web Services S3-compatible API and supports all core S3 features. |
| mogdb           | MOgDB is a stable and easy-to-use enterprise-ready relational database based on the openGauss open source database. |
| mongodb         | MongoDB is a document-oriented NoSQL database used for high volume data storage.                                                                                                                                  |
| mysql           | MySQL is a widely used, open-source relational database management system (RDBMS). |
| nebula          | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| neon            | Neon is Serverless Postgres built for the cloud. |
| oceanbase       | Unlimited scalable distributed database for data-intensive transactional and real-time operational analytics workloads, with ultra-fast performance that has once achieved world records in the TPC-C benchmark test. OceanBase has served over 400 customers across the globe and has been supporting all mission critical systems in Alipay. |
| official-postgresql | An official PostgreSQL cluster definition Helm chart for Kubernetes. |
| opengauss       | openGauss is an open source relational database management system that is released with the Mulan PSL v2.  |
| openldap        | The OpenLDAP Project is a collaborative effort to develop a robust, commercial-grade, fully featured, and open source LDAP suite of applications and development tools. This chart provides KubeBlocks. |
| opensearch      | Open source distributed and RESTful search engine. |
| oriolebd        | OrioleDB is a new storage engine for PostgreSQL, bringing a modern approach to database capacity, capabilities and performance to the world's most-loved database platform. |
| pika            | Pika (PikiwiDB) is a persistent huge storage service, compatible with the vast majority of redis interfaces, including string, hash, list, zset, set and management interfaces. |
| polardb-x       | PolarDB-X is a cloud native distributed SQL Database designed for high concurrency, massive storage, complex querying scenarios. |
| postgresql      | PostgreSQL is an advanced, enterprise class open source relational database that supports both SQL (relational) and JSON (non-relational) querying.                                                            |
| pulsar          | Apache® Pulsar™ is an open-source, distributed messaging and streaming platform built for the cloud. |
| qdrant          | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| redis           | Redis is a fast, open source, in-memory, key-value data store.                                                                                                                                                    |
| risingwave      | RisingWave is a distributed SQL database for stream processing. It is designed to reduce the complexity and cost of building real-time applications. |
| solr            | Solr is the popular, blazing-fast, open source enterprise search platform built on Apache Lucene. |
| starrocks       | StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis. |
| tidb            | TiDB is an open-source, cloud-native, distributed, MySQL-Compatible database for elastic scale and real-time analytics. |
| tdengine        | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| vllm            | vLLM is a fast and easy-to-use library for LLM inference and serving. |
| weaviate        | Weaviate is an open-source vector database.                                                                                                                                                                       |
| xinference      | Xorbits Inference(Xinference) is a powerful and versatile library designed to serve language, speech recognition, and multimodal models. |
| yanshan         | YashanDB is a database management system developed by Shenzhen Institute of Computing Science. |
| zookeeper       | Apache ZooKeeper is a centralized service for maintaining configuration information, naming, providing distributed synchronization, and providing group services. |

## Supported functions of add-ons

| Add-on (v0.8.0)     | version                                                   | Vscale           | Hscale           | Volumeexpand | Stop/Start       | Restart          | Expose | Backup/Restore | Logs | Config | Upgrade (DB engine version) | Account | Failover | Switchover | Monitor |
|:-------------------:|:---------------------------------------------------------:|:----------------:|:----------------:|:------------:|:----------------:|:----------------:|:------:|:--------------:|:----:|:------:|:---------------------------:|:-------:|:--------:|:----------:|:-------:|
| apecloud-mysql      | 8.0.30                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      |✔️               | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | ✔️          | ✔️       |
| apecloud-postgresql | 14.11                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| camellia-redis-proxy |  1.2.26                                                  | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | N/A            | ✔️    | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| clickhouse          | 22.9.4                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| doris               | 2.0.3                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| elasticsearch       | 8.8.2                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| etcd                | 3.5.6                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| foxlake             | 0.8.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| flink               | 1.16                                                      | ✔️                | ✔️ (task manager) | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| ggml                | N/A                                                       | N/A              | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| greptimedb          | 0.3.2                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| halo                | 0.2.0                                                     | ✔️                | ✔️                | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| influxdb            | 2.7.4                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| kafka               | 3.3.2                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| mariadb             | 10.6.15                                                   | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| milvus              | 2.2.4                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| minio               | 8.0.17                                                    | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| mogdb               | 5.0.5                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | N/A     | N/A      | ✔️          | N/A     |
| mongodb      | <p>4.0</p><p>4.2</p><p>4.4</p><p>5.0</p><p>5.0.20</p><p>6.0</p>  | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | N/A     | ✔️        | ✔️          | ✔️       |
| mysql               | <p>5.7.42</p><p>8.0.33 </p>                               | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | ✔️          | ✔️       |
| nebula              | 3.5.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| neon                | latest                                                    | ✔️                | N/A              | N/A          | N/A              | N/A              | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oceanbase           | 4.2.0.0-100010032023083021                                | N/A              | ✔️                | ✔️            | N/A              | N/A              |        | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oceanbase-cluster   | 4.2.0.0-100010032023083021                                | ✔️ (host network) | ✔️                | ✔️            | ✔️ (host network) | ✔️ (host network) | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| official-postgresql | <p>12.15</p><p>14.7</p><p>14.7-zhparser</p>               | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opengauss           | 5.0.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| openldap            | 2.4.57                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                |        | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| opensearch          | 2.7.0                                                     | ✔️                | N/A              | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| oracle              | 19.3.0-ee                                                 | ✔️                | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| orioledb            | beta1                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| pika                | 3.5.1                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| polardb-x           | 2.3                                                       | ✔️                | ✔️                | N/A          | ✔️                | N/A              | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| postgresql | <p>12.14.0</p><p>12.14.1</p><p>12.15.0</p><p>14.7.2</p><p>14.8.0</p> | ✔️              | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | ✔️                           | ✔️       | ✔️        | ✔️          | ✔️       |
| pulsar              | 2.11.2                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| qdrant              | 1.5.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | ✔️              | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| redis               | 7.0.6                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | ✔️      | ✔️              | ✔️    | ✔️      | N/A                         | ✔️       | ✔️        | N/A        | ✔️       |
| risingwave          | 1.0.0                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| starrocks           | 3.1.1                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| solr                | 8.11.2                                                    | ✔️                | ✔️                | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| tdengine            | 3.0.5.0                                                   | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| tidb                | 7.1.2                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| vllm                |                                                           | N/A              | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| weaviate            | 1.18.0                                                    | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | ✔️      | N/A                         | N/A     | N/A      | N/A        | ✔️       |
| xinference          | 1.16.0                                                    | ✔️                | N/A              | N/A          | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
| yashan              | personal-23.1.1.100                                       | ✔️                | ✔️ (Standalone)   | ✔️            | ✔️                | ✔️                | N/A    | N/A            | ✔️    | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |
| zookeeper           | 3.7.1                                                     | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | ✔️    | ✔️      | N/A                         | N/A     | N/A      | N/A        | N/A     |

:::note

The upgrade feature means that KubeBlocks supports minor version upgrades for a database engine. For example, you can upgrade PostgreSQL from v12.14 to v12.15.

:::

## Install addons

1. (Optional) Add the KubeBlocks repo. If you install KubeBlocks with Helm, just run `helm repo update`. 

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. View the addon versions.

   ```bash
   helm search repo kubeblocks/mariadb --devel --versions
   ```

3. Install the addon.

   ```bash
   helm install mariadb kubeblocks/mariadb --namespace kb-system --create-namespace --version 0.9.0
   ```

4. Verify whether this addon is installed.

   The STATUS is `deployed` and this addon is installed successfully.

   ```bash
   helm list -A
   >
   NAME                        	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART                       	APP VERSION
   ......
   mariadb                     	kb-system	1       	2024-05-08 17:41:29.112721 +0800 CST   	deployed	mariadb-0.9.0               	10.6.15
   ```
