---
title: Supported add-ons 
description: add-ons suported by KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 2
sidebar_label: Supported add-ons 
---
# Supported add-ons

An add-on provides extension capabilities, i.e., manifests or application software, to the KubeBlocks control plane.
KubeBlocks, as a cloud-native data infrastructure based on Kubernetes, providing management and control for relational databases, NoSQL databases, vector databases, and stream computing systems; and these databases can be all added as addons.
Addons supported:

| Add-ons                      | Description                                                                                                                                                                                                       |
|------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| aws-load-balancer-controller | The AWS Load Balancer Controller manages AWS Elastic Load Balancers for a Kubernetes cluster.                                                                                                                     |
| csi-s3                       | A container storage interface for S3.                                                                                                                                                                             |
| external-dns                 | External-dns makes Kubernetes resources discoverable via public DNS servers.                                                                                                                                      |
| fault-chaos-mesh             | Service-direct faults that require Chaos Mesh to be installed on the AKS cluster.                                                                                                                                 |
| kubebench                    | A tool that checks whether Kubernetes is deployed securely by running the checks documented in the CIS Kubernetes Benchmark                                                                                       |
| kubeblocks-csi-driver        | Container storage interface driver                                                                                                                                                                                |
| loki                         | Loki is a horizontally-scalable, highly-available, multi-tenant log aggregation system inspired by Prometheus.                                                                                                    |
| migration                    | ？                                                                                                                                                                                                                |
| milvus                       | Milvus is a flexible, reliable, & blazing-fast cloud-native, open-source vector database.                                                                                                                         |
| nebula                       | NebulaGraph is an open source graph database that can store and process graphs with trillions of edges and vertices.                                                                                              |
| nyancat                      |                                                                                                                                                                                                                   |
| opensearch                   | OpenSearch is a scalable, flexible, and extensible open-source software suite for search, analytics, and observability applications                                                                               |
| pyroscope-server             | Pyroscope Server processes, aggregates, and stores data from agents for speedy queries of any time range.                                                                                                         |
| qdrant                       | Qdrant is a vector database & vector similarity search engine.                                                                                                                                                    |
| tdengine                     | TDengine™ is an industrial data platform purpose-built for the Industrial IoT, combining a time series database with essential features like stream processing, data subscription, and caching.                  |
| weaviate                     | Weaviate is an open-source vector database.                                                                                                                                                                       |
| agamotto                     |                                                                                                                                                                                                                   |
| alertmanager-webhook-adaptor | A general webhook server for receiving Prometheus Alertmanager's notifications and send them through different channel types.                                                                                     |
| apecloud-mysql               |                                                                                                                                                                                                                   |
| csi-hostpath-driver          | A sample (non-production) CSI Driver that creates a local directory as a volume on a single node                                                                                                                  |
| grafana                      | Grafana is the open source analytics & monitoring solution for every database.                                                                                                                                    |
| mongodb                      |                                                                                                                                                                                                                   |
| postgresql                   |                                                                                                                                                                                                                   |
| prometheus                   | An open-source monitoring system with a dimensional data model, flexible query language, efficient time series database and modern alerting approach.                                                             |
| redis                        |                                                                                                                                                                                                                   |
| snapshot-controller          | The snapshot controller will be watching the Kubernetes API server for VolumeSnapshot and VolumeSnapshotContent CRD objects.                                                                                      |
| kafka                        | Apache Kafka is an open-source distributed event streaming platform used by thousands of companies for high-performance data pipelines, streaming analytics, data integration, and mission-critical applications. |




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
