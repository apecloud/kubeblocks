---
title: 为集群配置 Pod 亲和性
description: 如何为集群配置 Pod 亲和性
keywords: [pod 亲和性]
sidebar_position: 1
---

# 为集群配置 Pod 亲和性

亲和性控制了 Pod 在节点上的分配逻辑。合理地将 Kubernetes 的 Pod 分配到不同的节点上，可以提高业务的可用性、资源使用率和稳定性。

可以通过 `kbcli` 件来设置亲和性和容忍度。`kbcli` 仅支持集群级别的配置，如需实现集群级别和组件级别的配置，可使用 CR YAML 文件，具体操作可参考[API 文档](./../../../api-docs/maintenance/resource-scheduling/resource-scheduling.md)。

执行 `kbcli cluster create -h` 命令查看示例，并配置亲和性及容忍度的参数。

```bash
kbcli cluster create -h
>
Create a cluster

Examples:
  ......
  
  # 创建一个强制分散在节点上的集群
  kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname --pod-anti-affinity
Required
  
  # 在特定标签的节点上创建一个集群
  kbcli cluster create --cluster-definition apecloud-mysql --node-labels
'"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'
  
  # 创建一个具有两个容忍度的集群
  kbcli cluster create --cluster-definition apecloud-mysql --tolerations
'"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
  
  # 创建一个集群，其中每个 Pod 在自己的专用节点上运行
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

## 示例

以下示例使用 `kbcli` 创建集群实例，并展示了如何进行 Pod 亲和性和容忍度配置。

### 默认配置

无需使用亲和性参数。

### 尽量打散

如果你希望集群的 Pod 部署在不同的拓扑域，但是不希望在节点资源充足的时候，因为不满足分布条件而部署失败，那么可以配置尽量打散，可以将 Pod 亲和性配置为“Preferred”。

下面的示例创建了一个尽可能跨节点的集群。

```bash
kbcli cluster create --topology-keys kubernetes.io/hostname --pod-anti-affinity Preferred
```

### 强制打散

如果集群的 Pod 必须部署在不同的拓扑域，以确保集群能够跨拓扑域具备容灾能力，你可以将 Pod 亲和性配置为“Required”。

下面的示例创建了一个必须跨节点部署的集群。

```bash
kbcli cluster create --topology-keys kubernetes.io/hostname --pod-anti-affinity Required
```

### 在指定节点上部署

可以通过节点标签在指定的节点上部署集群。

下面的示例创建了一个在带有拓扑标签 `topology.kubernetes.io/zone=us-east-1a` 的节点上部署的集群。

```bash
kbcli cluster create --node-labels '"topology.kubernetes.io/zone=us-east-1a"'
```

### 独享主机组

如果想通过污点和节点标签来管理节点分组，并且需要将集群部署在独享的主机分组中，可以设置容忍度并指定一个节点标签。

例如，如果有一个用于部署数据库集群的主机分组，并且该主机添加了一个名为 `database=true:NoSchedule` 的污点和一个名为 `database=true` 的标签，那么可以按照以下命令创建一个集群。

```bash
kbcli cluster create --node-labels '"databa=true"' --tolerations '"key=database,value=true,operator=Equal,effect=NoSchedule"
```

### 集群 Pod 独占一个节点

如果需要一个仅用于线上核心业务的集群，并且需要确保该集群的每个 Pod 都有自己的节点以避免受到集群中其他 Pod 的影响，你可以将 `tenancy` 设置为“DedicatedNode”。

```bash
kbcli cluster create --tenancy=DedicatedNode
```

:::note

只有为这些节点添加污点之后，命令才能成功执行。否则，未由 KubeBlocks 托管的业务仍然可以部署在这些节点上。

:::
