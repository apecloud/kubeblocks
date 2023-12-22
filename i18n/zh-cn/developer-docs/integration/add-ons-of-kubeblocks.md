---
title: 简介
description: 简要介绍 KubeBlocks 支持的数据库引擎
keywords: [数据库引擎, 集成]
sidebar_position: 1
sidebar_label: KubeBlocks 中的数据库引擎
---

# KubeBlocks 中的数据库引擎

KubeBlocks 是基于 K8s 的云原生数据基础设施，用于管理各类数据库引擎。

本系列文档将介绍数据库引擎集成的基础知识，帮助你快速入门，成为 KubeBlocks 社区的一员。

KubeBlocks 集成生态丰富，已经接入多种主流数据库，包括：

- 关系型数据库：ApeCloud-MySQL（MySQL 集群版）、PostgreSQL（PostgreSQL 主备版）；
- NoSQL 数据库：MongoDB、Redis；
- 图数据库：Nebula（来自社区贡献者）；
- 时序数据库：TDengine、Greptime（来自社区贡献者）；
- 向量数据库：Milvus、Qdrant、Weaviate 等；
- 流数据库：Kafka、Pulsar。

在开始之前，你需要有以下知识储备：
1. 会写点 YAML（例如知道 YAML 的缩进需要多少个空格）。
2. 了解 Helm（例如知道什么是 Helm 和 Helm chart）。
3. 玩过 K8s（例如知道什么是 Pod，在 K8s 上用 Helm 装过 Operator）。
4. 了解 KubeBlocks  的基本概念，如 ClusterDefinition、ClusterVersion 和 Cluster。

如有任何问题，可以加入[官方 Slack 频道](https://join.slack.com/t/kubeblocks/shared_invite/zt-22cx2f84x-BPZvnLRqBOGdZ_XSjELh4Q)进行咨询。