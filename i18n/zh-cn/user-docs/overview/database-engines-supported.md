---
title: 支持的数据库类型
description: KubeBlocks, kbcli, multicloud
keywords: [kubeblocks, 简介, prometheus, s3, alertmanager]
sidebar_position: 4
sidebar_label: 支持的数据库类型
---
# 支持的数据库类型

KubeBlocks 是基于 Kubernetes 的云原生数据基础设施，可以帮助用户轻松构建关系型、NoSQL、流计算和向量型数据库服务。而这些数据库类型通常以引擎（addon）的形式添加到 KubeBlocks 中。除了支持数据库引擎外，KubeBlocks 引擎还支持适配云环境的插件及其他应用。

| 数据库引擎       | 简介                                                                                                                                                                                                       |
|:----------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| apecloud-mysql  | ApeCloud MySQL 是一个与 MySQL 语法兼容的数据库，主要利用 RAFT 共识协议实现高可用性。                                                          |
| apecloud-postgresql | ApeCloud PostgreSQL 是一款兼容 PostgreSQL 语法，通过 RAFT 共识协议实现高可用的数据库。 |
| camellia-redis-proxy | Camellia Redis Proxy是使用 Netty4 开发的高性能 Redis 代理。 |
| clickhouse      | ClickHouse 是列式数据库，能够帮助用户使用 SQL 查询实时生成强大的分析功能。 |
| doris           | Apache Doris 是用于实时分析的现代数据仓库。它能够对大规模实时数据进行极速分析。 |
| elasticsearch   | Elasticsearch 是一个分布式、RESTful 风格的搜索引擎，专为生产规模的工作负载进行了速度和相关性能的优化。 |
| etcd            | etcd 是一个高度一致的分布式键值存储，它提供了一种可靠的方式来存储需要由分布式系统或机器集群访问的数据。 |
| flink           | Apache Flink 是一个框架和分布式处理引擎，用于在无边界和有边界数据流上进行有状态的计算。 |
| foxlake         | ApeCloud FoxLake 是一个开源的云原生数据仓库。|
| ggml            | GGML 是一个为机器学习设计的张量库，它的目标是使大型模型能够在高性能的消费级硬件上运行。 |
| greptimedb      | GreptimeDB 是一个云原生时间序列数据库，具有分布式、可扩展和高效的特性。 |
| influxdb        | InfluxDB 作为专用的时序数据库，可执行实时分析，优化大型时序数据工作负载的处理和扩展。 ｜
| kafka           | Apache Kafka 是一个开源的分布式事件流平台，广泛应用于高性能数据流水线、流式分析、数据集成和关键应用程序等场景，目前已经被数千家公司采用。 |
| mariadb         | MariaDB 是一个高性能的开源关系型数据库管理系统，广泛用于 Web 和应用服务器。 |
| milvus          | Milvus 是一个灵活、可靠且高性能的云原生开源向量数据库。                                                                                                                        |
| minio           | MinIO 是对象存储解决方案，它提供与 Amazon Web Services S3 兼容的 API 并支持 S3 所有核心功能。 |
| mogdb           | MogDB 是基于 openGauss 开源数据库、稳定、易用的企业级关系型数据库。 |
| mongodb         | MongoDB 是一个面向文档的 NoSQL 数据库，用于存储大量数据。                                                                                                                                 |
| mysql           | MySQL 是一个广泛使用的开源关系型数据库管理系统（RDBMS）。 |
| nebula          | NebulaGraph 是一个开源的分布式图数据库，擅长处理具有千亿个顶点和万亿条边的超大规模数据集。                                                                                             |
| neon            | Neon 是一家多云无服务器 Postgres 提供商。|
| oceanbase       | OceanBase 是一个无限可扩展的分布式数据库，适用于数据密集型事务和实时运营分析工作负载，具有超快的性能，在 TPC-C 基准测试中曾一度创造了世界纪录。OceanBase 已经为全球超过 400 家客户提供了服务，并且一直在支持支付宝的所有关键业务系统。 |
| vanilla-postgresql | Kubernetes 的官方 PostgreSQL 集群定义 Helm Chart。 |
| opengauss       | openGauss 是一款开源关系型数据库管理系统，采用木兰宽松许可证 v2 发行。 |
| openldap        | OpenLDAP 项目旨在协作开发一个强大、商业级、功能齐全、开源的 LDAP 应用套件和开发工具。其 Chart 为 KubeBlocks 提供了支持。 |
| opensearch      | opensearch 是一个开源、分布式、 RESTful 风格的搜索引擎。|
| oriolebd        | OrioleDB 是 PostgreSQL 的全新存储引擎，为该数据库平台带来了现代化的数据库容量、功能和性能。 |
| pika            | Pika 是一个可持久化的大容量 Redis 存储服务，兼容 string、hash、list、zset、set 的绝大部分接口。 |
| polardb-x       | PolarDB-X 是一个为高并发、大规模存储和复杂查询场景设计的云原生分布式 SQL 数据库。|
| postgresql      | PostgreSQL 是一个先进的企业级开源关系型数据库，支持 SQL（关系型）和 JSON（非关系型）查询。                                                           |
| pulsar          | Apache® Pulsar™ 是一个开源的、分布式消息流平台。 |
| qdrant          | Qdrant 是一个向量相似性搜索引擎和向量数据库。                                                                                                                                                   |
| redis           | Redis 是一个开源的、高性能的、键值对内存数据库。                                                                                                                                                   |
| risingwave      | RisingWave 是一个分布式 SQL 流处理数据库，旨在帮助用户降低实时应用的开发复杂性和成本。|
| solr            | Solr 是基于 Apache Lucene 构建的流行、高速的开源企业搜索平台。 |
| starrocks       | StarRocks 是一款高性能分析型数据仓库，支持多维、实时、高并发的数据分析。|
| tidb            | TiDB 是一个开源、云原生、分布式、兼容 MySQL 的数据库，用于弹性扩展和实时分析。 |
| tdengine        | TDengine™ 是一个专为工业物联网而搭建的工业大数据平台，结合了时序数据库和流处理、数据订阅和缓存等重要功能。                 |
| vllm            | vLLM 是一个快速且易于使用的 LLM 推理和服务库。 |
| weaviate        | Weaviate 是一个开源的向量数据库。                                                                                                                                                                      |
| xinference      | Xorbits Inference（Xinference）是一个性能强大且功能全面的分布式推理框架。 |
| zookeeper       | Apache ZooKeeper 是集中式服务，用于维护配置信息、命名、提供分布式同步以及提供组服务。 |
| zookeeper       | Apache ZooKeeper 是一个集中式服务,用于维护配置信息、命名、提供分布式同步和提供组服务。 |

## 数据库功能

| 数据库引擎 (v0.8.0)                     | 版本                              | 垂直扩缩容 | 水平扩缩容 | 存储扩容  | 停止/启动    | 重启    | 备份/恢复        | 日志 | 配置    | 升级（内核小版本）             | 账户     | 故障切换  | 切换        | 监控    |
|---------------------------------------|-----------------------------------|--------|--------|--------------|------------|---------|----------------|------|--------|-----------------------------|---------|----------|------------|---------|
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
| vanilla-postgresql | <p>12.15</p><p>14.7</p><p>14.7-zhparser</p>               | ✔️                | ✔️                | ✔️            | ✔️                | ✔️                | N/A    | N/A            | N/A  | N/A    | N/A                         | N/A     | N/A      | N/A        | N/A     |
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

升级功能是指 KubeBlocks 支持数据库内核小版本升级，例如，将 PostgreSQL 从 v12.14 升级至 v12.15。

:::

## 使用引擎

### 使用索引安装引擎

KubeBlocks v0.8.0 发布后，引擎（addon）与 KubeBlocks 解耦，KubeBlocks 仅默认安装了部分引擎，如需体验其它引擎，需通过索引安装相关引擎。

官网引擎索引仓库为 [KubeBlocks index](https://github.com/apecloud/block-index)。引擎代码维护在 [KubeBlocks addon repo](https://github.com/apecloud/kubeblocks-addons)。

1. 查看引擎仓库索引。

   kbcli 默认创建名为 `kubeblocks` 的索引，可使用 `kbcli addon index list` 查看该索引。

   ```bash
   kbcli addon index list
   >
   INDEX        URL
   kubeblocks   https://github.com/apecloud/block-index.git 
   ```

   如果命令执行结果未显示或者你想要添加自定义索引仓库，则表明索引未建立，可使用 `kbcli addon index add <index-name> <source>` 命令手动添加索引。例如，

   ```bash
   kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git
   ```

2. （可选）索引建立后，可以通过 `addon search` 命令检查想要安装的引擎是否在索引信息中存在

   ```bash
   kbcli addon search mariadb
   >
   ADDON     VERSION   INDEX
   mariadb   0.7.0     kubeblocks
   ```

3. 安装引擎。

   当引擎有多个版本和索引源时，可使用 `--index` 指定索引源，`--version` 指定安装版本。系统默认以 `kubeblocks` 索引仓库 为索引源，安装最新版本。

   ```bash
   kbcli addon install mariadb --index kubeblocks --version 0.7.0
   ```

   **后续操作**

   引擎安装完成后，可查看引擎列表、启用引擎。

### 查看引擎列表

执行 `kbcli addon list` 命令查看已经支持的引擎。

### 启用/禁用引擎

请按照以下步骤手动启用或禁用引擎。

***步骤：***

1. 执行 `kbcli addon enable` 启用引擎。

   ***示例***

   ```bash
   kbcli addon enable snapshot-controller
   ```

   执行 `kbcli addon disable` 禁用引擎。

2. 再次查看引擎列表，检查是否已启用引擎。

   ```bash
   kbcli addon list
   ```
