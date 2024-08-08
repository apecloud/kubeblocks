---
title: Configure pod affinity for database clusters
description: How to configure pod affinity for database clusters
keywords: [pod affinity]
sidebar_position: 1
---

# Configure pod affinity for database clusters

Affinity controls the selection logic of pod allocation on nodes. By a reasonable allocation of Kubernetes pods on different nodes, the business availability, resource usage rate, and stability are improved.

Affinity and toleration can be set by `kbcli` or the CR YAML file of the cluster. `kbcli` only supports the cluster-level configuration and the CR YAML file supports both the cluster-level and component-level configurations.


Run `kbcli cluster create -h` to view the examples and the parameter options of affinity and toleration configurations.

```bash
kbcli cluster create -h
>
Create a cluster

Examples:
  ......
  
  # Create a cluster forced to scatter by node
  kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname --pod-anti-affinity
Required
  
  # Create a cluster in specific labels nodes
  kbcli cluster create --cluster-definition apecloud-mysql --node-labels
'"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'
  
  # Create a Cluster with two tolerations
  kbcli cluster create --cluster-definition apecloud-mysql --tolerations
'"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
  
  # Create a cluster, with each pod runs on their own dedicated node
  kbcli cluster create --tenancy=DedicatedNode

Options:
    ......
    --node-labels=[]:
        Node label selector

    --pod-anti-affinity='Preferred':
        Pod anti-affinity type, one of: (Preferred, Required)
        
    --tenancy='SharedNode':
        Tenancy options, one of: (SharedNode, DedicatedNode)

    --tolerations=[]:
        Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'

    --topology-keys=[]:
        Topology keys for affinity
    ......
.......
```

## Examples

The following examples use `kbcli` to create cluster instances and show the common situations of how to use pod affinity and toleration configuration.

### Default configuration

No affinity parameters are required.

### Spread evenly as much as possible

You can configure the pod affinity as "Preferred" if you want the pods of the cluster to be deployed in different topological domains, but do not want deployment failure due to failing to meet distribution requirements.
The example below creates and sets a cluster to be deployed evenly across nodes as much as possible.

```bash
kbcli cluster create --topology-keys kubernetes.io/hostname --pod-anti-affinity Preferred
```

### Forced spread evenly

You can configure the pod affinity as "Required" if the pods of the cluster must be deployed in different topological domains to ensure that the cluster can be disaster-tolerant across topological domains.
The example below creates and sets a cluster that must be deployed across nodes.

```bash
kbcli cluster create --topology-keys kubernetes.io/hostname --pod-anti-affinity Required
```

### Deploy pods in specified nodes

You can specify a node label to deploy a cluster on the specified node.
The example below creates and sets a cluster to be deployed on the node with an available zone label of `topology.kubernetes.io/zone=us-east-1a`.

```bash
kbcli cluster create --node-labels '"topology.kubernetes.io/zone=us-east-1a"'
```

### Deploy pods in dedicated nodes

If you want to manage node groups by the taint and node labels and need the clusters to be deployed on a dedicated host group, you can set tolerations and specify a node label.

For example, you have a host group for deploying database clusters and this host is added with a taint named `database=true:NoSchedule` and a label `database=true`, then you can follow the command below to create a cluster.

```bash
kbcli cluster create --node-labels '"databa=true"' --tolerations '"key=database,value=true,operator=Equal,effect=NoSchedule"
```

### One node only for one pod

If you need one cluster only for the online core business and need to ensure every pod of this cluster has its own node to avoid being affected by the cluster of the cluster, you can specify `tenancy` as "DedicatedNode".

```bash
kbcli cluster create --tenancy=DedicatedNode
```

:::note

This command will be performed successfully based on the prerequisite that you have added taints for these nodes. Otherwise, the business that is not managed by KubeBlocks can still be deployed on these nodes.

:::
