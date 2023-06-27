---
title: Configure pod affinity for database clusters
description: How to configure pod affinity for database clusters
keywords: [pod affinity]
sidebar_position: 1
---

# Configure pod affinity for database clusters

Affinity controls the selection logic of pod allocation on nodes. By a reasonable allocation of Kubernetes pods on different nodes, the business availability, resource usage rate, and stability are improved.

Affinity and toleration can be set by `kbcli` or the CR YAML file of the cluster. `kbcli` only supports the cluster-level configuration and the CR YAML file supports both the cluster-level and component-level configurations.

## Option 1. Use kbcli

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

## Option 2. Use a YAML file

You can configure pod affinity and toleration in either the spec of a cluster or the spec of a component.

The cluster-level configuration is used as the default configuration of all components; if the pod affinity configuration exists in a component, the component-level configuration will take effect and cover the default cluster-level configuration.

```yaml
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    nodeLabels:
      topology.kubernetes.io/zone: us-east-1a
    tenancy: sharedNode
  tolerations:
  - key: EngineType
    operator: Equal
    value: mysql
    effect: NoSchedule
  componentSpecs:
  - name: mysql
    componentDefRef: mysql
    affnity:
      podAntiAffinity: Required
      topologyKeys:
        - kubernetes.io/hostname
    ......
```

**Description of parameters in the YAML file**

* Affinity
  Parameters related to pod affinity are under the object of `spec.affinity` in the Cluster CR YAML file.
  The pod affinity configuration can be applied to the cluster or component and the component-level configuration covers the cluster-level configuration.

* Toleration
  Parameters related to toleration are under the object of `spec.tolerations` in the Cluster CR YAML file and Kubernetes native semantics are used. For the toleration parameter configuration, refer to the Kubernetes official document [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).
  Like affinity configuration, toleration also supports component-level and cluster-level configurations. The cluster-level configuration is set as default and the component-level configuration covers the cluster-level configuration.

| **Parameter**   | **Value**                                    | **Description**  |
| :--             | :--                                          | :--              |
| podAntiAffinity | - Required <br/> - Prefferred (default)      | It stands for the anti-affinity level of the pod under the current component.<br/> - Required means pods must be spread evenly in the fault domain specified by `topologyKeys`. <br/> - Preferred means pods can be spread as evenly as possible in the fault domain specified by `topologyKeys`. |
| topologyKeys    |                                              | TopologyKey is the key of the node label. The node with the same value as this key is considered to be in the same topology, i.e. the same fault domain.<br/>For example, if the TopologyKey is `kubernetes.io/hostname`, every node is a domain of this topology. If the TopologyKey is `topology.kubernetes.io/zone`, every available zone is a domain of this topology. |
| nodeLabels      |                                              | NodeLabels specifies a pod can only be scheduled to the node with the specified node label. |
| tenancy         | - SharedNode (default) <br/> - DedicatedNode | It refers to the pod tenancy type:<br/> - SharedNode means that multiple pods share a node.<br/> - DedicatedNode means that a node is dedicated to a pod. |

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
