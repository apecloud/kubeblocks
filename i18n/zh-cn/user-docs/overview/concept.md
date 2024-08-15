---
title: 概念
description: KubeBlocks, kbcli, 多云
keywords: [kubeblocks, 概念, 简介]
sidebar_position: 3
---

# 概念

- 节点（Node）：在分布式数据库中，每台计算机被称为一个节点，每个节点都有其独立的存储和处理能力。通过添加新节点，可以轻松扩展分布式数据库的存储和处理能力，以应对日益增长的数据量和并发访问需求。分布式数据库可以将读写请求分配到不同的节点进行处理，从而实现负载均衡并提高系统的并发处理能力。

- 数据分片（Data Sharding）：为了实现数据的分布式存储，需要将数据分成多个部分，每个部分称为一个数据分片。常见的数据分片策略包括：

   - 范围分片（Range Sharding）：根据键值范围将数据分成多个分片，每个分片负责一个连续的键值范围。
   - 哈希分片（Hash Sharding）：使用哈希函数将数据的键值映射到不同的分片，每个分片负责一个哈希值范围。
   - 复合分片（Composite Sharding）：结合多种分片策略，例如先根据范围进行分片，然后再根据哈希进行分片，以优化数据的分布和访问效率。

- Pod：Pod 是 K8s 中最小的可部署和可管理单元。它由一个或多个紧密关联的容器组成，这些容器共享网络和存储资源，并作为一个整体进行调度和管理。在 K8s 中，可以通过配置资源请求和限制来管理和控制 Pod 对节点资源（CPU、内存）的使用。

   - 资源请求定义了 Pod 在运行时所需的最低资源量。K8s 调度器会选择能够满足 Pod 资源请求的节点，确保这些节点有足够的可用资源来满足 Pod 的需求。
   - 资源限制定义了 Pod 在运行时可以使用的最大资源量。它们用于防止 Pod 消耗过多的资源，从而保护节点和其他 Pod 不受影响。

- 数据复制（Replication）

   为了提高数据的可用性和容错性，分布式数据库通常会将数据复制到多个节点上，每个节点拥有完整或部分数据的副本。通过数据复制和故障转移机制，分布式数据库即使在节点故障时也能继续提供服务，从而提高系统的可用性。常见的复制策略包括：

   - 主从复制（Primary-Replica Replication）：
     - 每个分区有一个主节点和多个从节点。
     - 写操作在主节点上执行，然后同步到从节点。
     - 常见的主从复制协议包括强同步、半同步、异步，以及基于 Raft/Paxos 的复制协议。
   - 多主复制（Multi-Primary Replication）：
     - 每个分区有多个主节点。
     - 写操作可以在任意一个主节点上执行，然后同步到其他主节点和从节点。
     - 数据一致性通过复制协议以及全局锁、乐观锁等机制来维护。

总的来说，数据复制是分布式数据库提高可用性和容错性的一项关键技术。不同的复制策略在一致性、可用性和性能之间涉及不同的权衡，应基于具体的应用需求来做出决策。

KubeBlocks 对容器化分布式数据库的管理映射到四个层级的对象：Cluster（集群）、Component（组件）、InstanceSet（实例集）和 Instance（实例），形成了分层架构：

- **集群层（Cluster layer）**：Cluster 对象表示一个完整的分布式数据库集群。Cluster 是最高级别的抽象，包括数据库的所有组件和服务。
- **组件层（Component layer）**：Component 表示构成 Cluster 对象的逻辑组件，如元数据管理、数据存储、查询引擎等。每个 Component 对象都有其特定的任务和功能。一个 Cluster 对象包含一个或多个 Component 对象。
- **实例集层（InstanceSet layer）**：InstanceSet 对象管理 Component 对象内多个副本所需的工作负载，感知这些副本的角色。一个 Component 对象包含一个 InstanceSet 对象。
- **实例层（Instance layer）**：Instance 对象表示 InstanceSet 对象中的实际运行实例，对应于 Kubernetes 中的 Pod。一个 InstanceSet 对象可以管理零个到多个 Instance 对象。
- **ComponentDefinition** 是用于定义分布式数据库组件的 API，描述了组件的实现细节和行为。通过 ComponentDefinition，可以定义组件的关键信息，如容器镜像、配置模板、启动脚本、存储卷等。它们还可以为组件在不同事件（如节点加入、节点离开、组件增加、组件移除、角色切换等）下设置行为和逻辑。每个组件可以拥有独立的 ComponentDefinition，或共享相同的 ComponentDefinition。
- **ClusterDefinition** 是用于定义分布式数据库集群整体结构和拓扑的 API。在 ClusterDefinition 中，可以引用其包含组件的 ComponentDefinition，并定义组件之间的依赖关系和引用关系。
