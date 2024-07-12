---
title: Overview
description: An overview of add an database add-on to KubeBlocks
keywords: [add-on, integration]
sidebar_position: 1
sidebar_label: Add-ons of KubeBlocks
---

# Add-ons of KubeBlocks

KubeBlocks is a control and management platform to manage a bunch of database engines and other add-ons.

This series provides basic knowledge of add-ons, so you can get a quick start and become a member of the KubeBlocks community.

KubeBlocks features a rich add-on ecosystem with major databases, streaming and vector databases, including:

- Relational Database: ApeCloud-MySQL (MySQL RaftGroup cluster), PostgreSQL (Replication cluster) 
- NoSQL Database: MongoDB, Redis
- Graph Database: Nebula (from community contributors)
- Time Series Database: TDengine, Greptime (from community contributors)
- Vector Database: Milvus, Qdrant, Weaviate, etc.
- Streaming: Kafka, Pulsar, ElasticSearch

Adding an add-on to KubeBlocks is easy, you can just follow this guide to add the add-on to KubeBlocks as long as you know the followings: 
1. How to write a YAML file (e.g., You should know how many spaces to add when indenting with YAML).
2. Knowledge about Helm (e.g. What is Helm and Helm chart).
3. Have tried K8s (e.g., You should know what a pod is, or have installed an operator on K8s with Helm).
4. Grasp basic concepts of KubeBlocks, such as ClusterDefinition, ClusterVersion and Cluster.

If you have any question, you can join our [slack channel](https://join.slack.com/t/kubeblocks/shared_invite/zt-22cx2f84x-BPZvnLRqBOGdZ_XSjELh4Q) to ask.