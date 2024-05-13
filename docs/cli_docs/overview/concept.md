---
title: Concepts
description: KubeBlocks, kbcli, multicloud, containerized database,
keywords: [kubeblocks, overview, introduction]
sidebar_position: 1
---
The management of containerized distributed database by KubeBlocks is mapped to objects at four levels: Cluster, Component, InstanceSet, and Instance, forming a layered architecture:

- Cluster layer: A Cluster object represents a complete distributed database cluster. Cluster is the top-level abstraction, includeing all components and services of the database.
- Component layer: A Component represents logical components that make up the Cluster object, such as metadata management, data storage, query engine, etc. Each Component object has its specific task and functions. A Cluster object contains one or more Component objects.
- InstanceSet layer: An InstanceSet object manages the workload required for multiple replicas inside a Component object, perceiving the roles of the replicas. A Component object contains an InstanceSet object.
- Instance layer: An Instance object represents an actual running instance within an InstanceSet object, corresponding to a Pod in Kubernetes. An InstanceSet object can manage zero to multiple Instance objects.
- ComponentDefinition is an API used to define components of a distributed database, describing the implementation details and behavior of the components. With ComponentDefinition, you can define key information about components such as container images, configuration templates, startup scripts, storage volumes, etc. They can also set the behavior and logic of components for different events (e.g., node joining, node leaving, addition of components, removal of components, role switching, etc.). Each component can have its own independent ComponentDefinition or share the same ComponentDefinition.
- ClusterDefinition is an API used to define the overall structure and topology of a distributed database cluster. Within ClusterDefinition, you can reference ComponentDefinitions of its included components, and define dependencies and references between components.