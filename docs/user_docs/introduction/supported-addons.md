---
title: Supported add-ons 
description: add-ons suported by KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 2
sidebar_label: Supported add-ons 
---
# Supported add-ons

KubeBlocks, as a cloud-native data infrastructure based on Kubernetes, providing management and control for relational databases, NoSQL databases, vector databases, and stream computing systems; and these databases can be all added as addons.

| Add-ons        | Description                                                                                                                                                                                                       |
|----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| milvus         | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| nebula         | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| qdrant         | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| tdengine       | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| weaviate       | Weaviate is an open-source vector database.                                                                                                                                                                       |
| apecloud-mysql | ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability through the utilization of the RAFT consensus protocol.                                                          |
| mongodb        | MongoDB is a document-oriented NoSQL database used for high volume data storage.                                                                                                                                  |
| postgresql     | PostgreSQL is an advanced, enterprise class open source relational database that supports both SQL (relational) and JSON (non-relational) querying.｜                                                             |
| redis          | Redis is a fast, open source, in-memory, key-value data store.                                                                                                                                                    |
| kafka          | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |


To list supported add-ons, run `kbcli addon list` command.
**To manually enable or disable add-ons**
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
