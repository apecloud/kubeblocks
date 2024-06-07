---
title: Concepts
description: KubeBlocks, kbcli, multicloud, containerized database,
keywords: [kubeblocks, overview, introduction]
sidebar_position: 2
---

- Node: In a distributed database, each computer is referred to as a node, and each node has its own storage and processing capabilities. By adding new nodes, the storage and processing capacity of the distributed database can be easily expanded to accommodate the growing volume of data and concurrent access demands. Distributed databases can distribute read and write requests to different nodes for processing, achieving load balancing and improving the system's concurrent processing capabilities.
- Data Sharding: To achieve distributed storage of data, it is necessary to divide the data into multiple parts, with each part being called a data shard. Common data sharding strategies include:
  - Range Sharding: The data is divided into multiple shards based on the key value range, with each shard responsible for a continuous key value range.
  - Hash Sharding: A hash function is used to map the data's key values to different shards, with each shard being responsible for a hash value range.
  - Composite Sharding: Multiple sharding strategies are combined, such as first sharding based on range and then sharding based on hash, to optimize the distribution and access efficiency of data.

- Pod: A Pod is the smallest deployable and manageable unit in K8s. It consists of one or more closely related containers that share network and storage resources and are scheduled and managed as a single entity. In K8s, Pod's utilization of node resources (CPU, memory) can be managed and controlled by configuring resource requests and limits. 
  - Resource requests define the minimum amount of resources that a Pod requires at runtime. The K8s scheduler selects nodes that can satisfy the Pod's resource requests, ensuring that the nodes have sufficient available resources to meet the Pod's needs.
  - Resource limits define the maximum amount of resources that a Pod can use at runtime. They are used to prevent the Pod from consuming excessive resources and protect nodes and other Pods from being affected.

- Replication

  To improve the availability and fault tolerance of data, distributed databases typically replicate data across multiple nodes, with each node having a complete or partial copy of the data. Through data replication and failover mechanisms, distributed databases can continue to provide service even when nodes fail, thereby increasing the system's availability. Common replication strategies include:
  - Primary-Replica Replication:
    - Each partition has a single primary node and multiple replica nodes.
    - Write operations are executed on the primary node and then synchronized to the replica nodes.
    - Common primary-replica replication protocols include strong synchronous, semi-synchronous, asynchronous, and Raft/Paxos-based replication protocols.
  - Multi-Primary Replication:
    - Each partition has multiple primary nodes.
    - Write operations can be executed on any of the primary nodes, and then synchronized to the other primary nodes and replica nodes.
    - Data consistency is maintained through the replication protocol, combined with global locks, optimistic locking, and other mechanisms.

Overall, data replication is a key technology used by distributed databases to improve availability and fault tolerance. Different replication strategies involve different trade-offs between consistency, availability, and performance, and the choice should be made based on the specific application requirements.

The management of a containerized distributed database by KubeBlocks is mapped to objects at four levels: Cluster, Component, InstanceSet, and Instance, forming a layered architecture:

- Cluster layer: A Cluster object represents a complete distributed database cluster. Cluster is the top-level abstraction, including all components and services of the database.
- Component layer: A Component represents logical components that make up the Cluster object, such as metadata management, data storage, query engine, etc. Each Component object has its specific task and functions. A Cluster object contains one or more Component objects.
- InstanceSet layer: An InstanceSet object manages the workload required for multiple replicas inside a Component object, perceiving the roles of the replicas. A Component object contains an InstanceSet object.
- Instance layer: An Instance object represents an actual running instance within an InstanceSet object, corresponding to a Pod in Kubernetes. An InstanceSet object can manage zero to multiple Instance objects.
- ComponentDefinition is an API used to define components of a distributed database, describing the implementation details and behavior of the components. With ComponentDefinition, you can define key information about components such as container images, configuration templates, startup scripts, storage volumes, etc. They can also set the behavior and logic of components for different events (e.g., node joining, node leaving, addition of components, removal of components, role switching, etc.). Each component can have its own independent ComponentDefinition or share the same ComponentDefinition.
- ClusterDefinition is an API used to define the overall structure and topology of a distributed database cluster. Within ClusterDefinition, you can reference ComponentDefinitions of its included components, and define dependencies and references between components.
