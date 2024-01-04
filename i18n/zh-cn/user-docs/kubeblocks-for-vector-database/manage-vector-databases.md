---
title: 用 KubeBlocks 管理向量数据库
description: 如何用 KubeBlocks 管理向量数据库
keywords: [qdrant, milvus, weaviate]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理向量数据库
---

# 用 KubeBlocks 管理向量数据库

生成式人工智能的爆火引发了人们对向量数据库的关注。目前，KubeBlocks 支持 Qdrant、Milvus、Weaviate 等向量数据库的管理和运维。本文档以 Qdrant 为例，展示如何使用 KubeBlocks 管理向量数据库。

在开始之前，请[安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md) 和 [KubeBlocks](./../installation/install-with-helm/install-kubeblocks-with-helm.md)。

## 创建集群

***步骤：***

1. 创建一个 Qdrant 集群。

   如需管理其他向量数据库，可将 `cluster-definition` 的值更改为其他的数据库。

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant
   ```

   如果想创建一个有多副本的 Qdrant 集群，执行以下命令设置副本数量。

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant --set replicas=3
   ```

2. 检查集群是否已创建。

   ```bash
   kbcli cluster list
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   qdrant   default     qdrant               qdrant-1.1.0   Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

3. 查看集群信息。

   ```bash
   kbcli cluster describe qdrant
   >
   Name: qdrant         Created Time: Aug 15,2023 23:03 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION        STATUS    TERMINATION-POLICY
   default     qdrant               qdrant-1.1.0   Running   Delete

   Endpoints:
   COMPONENT   MODE        INTERNAL                                       EXTERNAL
   qdrant      ReadWrite   qdrant-qdrant.default.svc.cluster.local:6333   <none>
                           qdrant-qdrant.default.svc.cluster.local:6334

   Topology:
   COMPONENT   INSTANCE          ROLE     STATUS    AZ       NODE                   CREATED-TIME
   qdrant      qdrant-qdrant-0   <none>   Running   <none>   x-worker3/172.20.0.3   Aug 15,2023 23:03 UTC+0800
   qdrant      qdrant-qdrant-1   <none>   Running   <none>   x-worker2/172.20.0.5   Aug 15,2023 23:03 UTC+0800
   qdrant      qdrant-qdrant-2   <none>   Running   <none>   x-worker/172.20.0.2    Aug 15,2023 23:04 UTC+0800

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
   qdrant      false       1 / 1                1Gi / 1Gi               data:20Gi      standard

   Images:
   COMPONENT   TYPE     IMAGE
   qdrant      qdrant   docker.io/qdrant/qdrant:latest

   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE     BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME
   Disabled      <none>            <none>   7d           <none>          <none>

   Show cluster events: kbcli cluster list-events -n default qdrant
   ```

## 连接到向量数据库集群

Qdrant 提供两种客户端访问协议：HTTP 和 gRPC，它们分别使用端口 6333 和 6334 进行通信。根据客户端所在的位置，你可以使用不同的方法连接到 Qdrant 集群。

:::note

如果你的集群在 AWS 上，请先安装 AWS 负载均衡控制器。

:::

- 如果客户端在 K8s 集群内部，执行 `kbcli cluster describe qdrant` 命令获取集群的 ClusterIP 地址或相应的 K8s 集群域名。
- 如果客户端在 K8s 集群外部但在同一 VPC 内，执行 `kbcli cluster expose qdrant --enable=true --type=vpc` 命令获取数据库集群的 VPC 负载均衡器地址。
- 如果客户端在 VPC 外部，执行 `kbcli cluster expose qdrant --enable=true --type=internet` 命令打开数据库集群的公共网络可达地址。

## 监控向量数据库

打开 Grafana 监控页。

```bash
kbcli dashboard open kubeblocks-grafana
```

此命令会打开浏览器，然后就可以看到仪表盘。

## 扩缩容

KubeBlocks 支持对向量数据库的水平/垂直扩缩容。

### 水平扩缩容

执行以下命令进行水平扩缩容。

```bash
kbcli cluster hscale qdrant --replicas=5 --components=qdrant
```

这里需要等待几秒钟，直到扩缩容完成。

`kbcli cluster hscale` 命令会打印输出 `opsname`。执行以下命令检查扩缩容进度：

```bash
kubectl get ops qdrant-horizontalscaling-xpdwz
>
NAME                             TYPE                CLUSTER   STATUS    PROGRESS   AGE
qdrant-horizontalscaling-xpdwz   HorizontalScaling   qdrant    Running   0/2        16s
```

查看扩缩容是否已经完成。

```bash
kbcli cluster describe qdrant
```

### 垂直扩缩容

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale qdrant --cpu=0.5 --memory=512Mi --components=qdrant 
```

这里需要等待几秒钟，直到扩缩容完成。

`kbcli cluster vscale` 命令会打印输出 `opsname`。执行以下命令检查扩缩容进度：

```bash
kubectl get ops qdrant-verticalscaling-rpw2l
>
NAME                           TYPE              CLUSTER   STATUS    PROGRESS   AGE
qdrant-verticalscaling-rpw2l   VerticalScaling   qdrant    Running   1/5        44s
```

查看扩缩容是否已经完成。

```bash
kbcli cluster describe qdrant
```

## 磁盘扩容

***步骤：***

```bash
kbcli cluster volume-expand qdrant --storage=40Gi --components=qdrant -t data
```

这里需要等待几分钟，直到磁盘扩容完成。

`kbcli cluster volume-expand` 命令会打印输出 `opsname`。执行以下命令检查磁盘扩容进度：

```bash
kubectl get ops qdrant-volumeexpansion-5pbd2
>
NAME                           TYPE              CLUSTER   STATUS   PROGRESS   AGE
qdrant-volumeexpansion-5pbd2   VolumeExpansion   qdrant    Running  -/-        67s
```

查看磁盘扩容是否已经完成。

```bash
kbcli cluster describe qdrant
```