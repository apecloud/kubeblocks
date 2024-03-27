---
title: Redis Cluster 模式
description: KubeBlocks 中的 Redis Cluster 模式
keywords: [redis cluster, 概述, 功能]
sidebar_position: 4
---

# Redis Cluster 模式

虽然 Redis Sentinel 集群提供了出色的故障转移支持，但其本身不提供数据分片，所有数据仍然驻留在单个 Redis 实例上，并受到该实例的内存和性能限制，因此可能会影响系统在处理大型数据集和高读/写操作时的水平扩展能力。

KubeBlocks 现已支持 Redis Cluster 模式。该模式不仅允许更大的内存分布，还支持并行处理，从而显著地提高了数据密集型操作的性能。本文档将对 Redis Cluster 模式及其基本操作进行简要介绍。