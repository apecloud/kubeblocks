---
title: 用 KubeBlocks 管理 Qdrant
description: 如何用 KubeBlocks 管理 Qdrant
keywords: [qdrant, 向量数据库]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Qdrant
---

# 用 KubeBlocks 管理 Qdrant

生成式人工智能的爆火引发了人们对向量数据库的关注。

Qdrant（读作：quadrant）是向量相似性搜索引擎和向量数据库。它提供了生产可用的服务和便捷的 API，用于存储、搜索和管理点（即带有额外负载的向量）。Qdrant 专门针对扩展过滤功能进行了优化，使其在各种神经网络或基于语义的匹配、分面搜索以及其他应用中充分发挥作用。

目前，KubeBlocks 支持 Qdrant 的管理和运维。本文档展示如何使用 KubeBlocks 管理 Qdrant。

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 qdrant 引擎](./../overview/database-engines-supported.md#使用引擎)

## 创建集群

***步骤：***

1. 创建一个 Qdrant 集群。

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant
   ```

   如果想创建一个有多副本的 Qdrant 集群，执行以下命令设置副本数量。

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant --set replicas=3
   ```

:::note

执行以下命令，查看更多集群创建的选项和默认值。
  
```bash
kbcli cluster create --help
```

:::

1. 检查集群是否已创建。

   ```bash
   kbcli cluster list
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   qdrant   default     qdrant               qdrant-1.1.0   Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

2. 查看集群信息。

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

## 监控集群

对于测试环境，您可以执行以下命令，打开 Grafana 监控大盘。

1. 查看内置监控引擎，确认监控引擎已启用。如果未启用，可参考[该文档](./../overview/database-engines-supported.md#使用引擎)启用引擎。

   ```bash
   # View all addons supported
   kbcli addon list
   ...
   grafana                        Helm   Enabled                   true                                                                                    
   alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
   prometheus                     Helm   Enabled    alertmanager   true 
   ...
   ```

2. 检查集群的监控功能是否启用。如果监控功能已启用，输出结果中将会显示 `disableExporter: false`。

   ```bash
   kubectl get cluster qdrant -o yaml
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
   ......
   spec:
     ......
     componentSpecs:
     ......
       disableExporter: false
   ```

   如果输出结果未显示 `disableExporter: false`，这表示集群监控功能未开启，需先启用该功能。

   ```bash
   kbcli cluster update qdrant --disable-exporter=false
   ```

3. 查看大盘列表。

   ```bash
   kbcli dashboard list
   >
   NAME                                 NAMESPACE   PORT    CREATED-TIME
   kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
   ```

4. 打开监控大盘，查看控制台。

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

对于生产环境，强烈建议您搭建您专用的监控系统或者购买第三方监控服务。可参考[监控文档](./../observability/monitor-database.md#生产环境)了解详情。

## 扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容 API 文档](./../../api-docs/maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list qdrant
>
NAME     CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY    STATUS    AGE
qdrant   qdrant               qdrant-1.8.1   Delete                Running   47m
```

#### 步骤

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

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list mycluster
>
NAME     CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY    STATUS    AGE
qdrant   qdrant               qdrant-1.8.1   Delete                Running   47m
```

#### 步骤

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
qdrant-volumeexpansion-5pbd2   VolumeExpansion   qdrant    Running  1/1        67s
```

查看磁盘扩容是否已经完成。

```bash
kbcli cluster describe qdrant
```

## 重启

1. 重启集群。

   配置 `--components` 和 `--ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart qdrant --components="qdrant" \
   --ttlSecondsAfterSucceed=30
   ```

   - `--components` 表示需要重启的组件名称。
   - `--ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list qdrant
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
   qdrant   default     qdrant                 qdrant-1.8.1    Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

   * STATUS=Updating 表示集群正在重启中。
   * STATUS=Running 表示集群已重启。

## 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

### 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   ```bash
   kbcli cluster stop qdrant
   ```

2. 查看集群状态，确认集群是否已停止。

    ```bash
    kbcli cluster list
    ```

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   ```bash
   kbcli cluster start qdrant
   ```

2. 查看集群状态，确认集群是否已再次运行。

    ```bash
    kbcli cluster list
    ```
