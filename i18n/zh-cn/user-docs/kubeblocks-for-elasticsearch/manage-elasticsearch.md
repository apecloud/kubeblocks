---
title: 用 KubeBlocks 管理 Elasticsearch
description: 如何用 KubeBlocks 管理 Elasticsearch
keywords: [elasticsearch]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Elasticsearch
---

# 用 KubeBlocks 管理 Elasticsearch

Elasticsearch 是一个分布式、RESTful 风格的搜索和数据分析引擎，能够解决不断涌现出的各种用例。作为 Elastic Stack 的核心，Elasticsearch 会集中存储您的数据，让您飞快完成搜索，微调相关性，进行强大的分析，并轻松缩放规模。

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 elasticsearch 引擎](./../overview/database-engines-supported.md#使用引擎)。

## 创建集群

***步骤***

1. 创建集群。

   ```bash
   kbcli cluster create elasticsearch elasticsearch
   ```

:::note

执行以下命令，查看更多集群创建的选项和默认值。
  
```bash
kbcli cluster create --help
```

:::

2. 查看集群是否已创建。

   ```bash
   kbcli cluster list
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS            CREATED-TIME
   elasticsearch   default     elasticsearch        elasticsearch-8.8.2   Delete               Running          Jul 05,2024 16:51 UTC+0800   
   ```

3. 查看集群信息。

   ```bash
   kbcli cluster describe elasticsearch
   >
   Name: elasticsearch	 Created Time: Jul 05,2024 16:51 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION               STATUS    TERMINATION-POLICY   
   default     elasticsearch        elasticsearch-8.8.2   Running   Delete               

   Endpoints:
   COMPONENT       MODE        INTERNAL                                                     EXTERNAL   
   elasticsearch   ReadWrite   elasticsearch-elasticsearch.default.svc.cluster.local:9200   <none>     
                            elasticsearch-elasticsearch.default.svc.cluster.local:9300              
                            elasticsearch-elasticsearch.default.svc.cluster.local:9114              
    coordinating    ReadWrite   elasticsearch-coordinating.default.svc.cluster.local:9200    <none>     
                            elasticsearch-coordinating.default.svc.cluster.local:9300               
    ingest          ReadWrite   elasticsearch-ingest.default.svc.cluster.local:9200          <none>     
                            elasticsearch-ingest.default.svc.cluster.local:9300                     
    data            ReadWrite   elasticsearch-data.default.svc.cluster.local:9200            <none>     
                            elasticsearch-data.default.svc.cluster.local:9300                       
    master          ReadWrite   elasticsearch-master.default.svc.cluster.local:9200          <none>     
                            elasticsearch-master.default.svc.cluster.local:9300                     

    Topology:
    COMPONENT       INSTANCE                        ROLE     STATUS    AZ       NODE     CREATED-TIME                 
    master          elasticsearch-master-0          <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    data            elasticsearch-data-0            <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    ingest          elasticsearch-ingest-0          <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    elasticsearch   elasticsearch-elasticsearch-0   <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    coordinating    elasticsearch-coordinating-0    <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   

    Resources Allocation:
    COMPONENT       DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS     
    elasticsearch   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    coordinating    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    ingest          false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    data            false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    master          false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   

    Images:
    COMPONENT       TYPE            IMAGE                                   
    elasticsearch   elasticsearch   docker.io/bitnami/elasticsearch:8.8.2   
    coordinating    coordinating    docker.io/bitnami/elasticsearch:8.8.2   
    ingest          ingest          docker.io/bitnami/elasticsearch:8.8.2   
    data            data            docker.io/bitnami/elasticsearch:8.8.2   
    master          master          docker.io/bitnami/elasticsearch:8.8.2   

    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   

    Show cluster events: kbcli cluster list-events -n default elasticsearch
   ```

## 连接集群

Elasticsearch 提供 HTTP 访问协议，使用端口 9200 进行通信。您可通过本地主机访问集群。

```bash
curl http://127.0.0.1:9200/_cat/nodes?v
```

## 监控集群

测试环境中，可执行以下命令，打开 Grafana 监控大盘。

1. 查看 KubeBlocks 内置引擎，确保监控相关引擎已开启。如果监控引擎未启用，可参考[该文档](./../overview/database-engines-supported.md#使用引擎)，启用引擎。

   ```bash
   # View all addons supported
   kbcli addon list
   ...
   grafana                        Helm   Enabled                   true                                                                                    
   alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
   prometheus                     Helm   Enabled    alertmanager   true 
   ...
   ```

2. 查看集群监控功能是否开启。可通过查看集群 YAML 文件中是否显示 `disableExporter: false`，如果有该字段，则说明集群监控功能已开启。

   ```bash
   kubectl get cluster elasticsearch -o yaml
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

   如果输出结果未显示 `disableExporter: false`，则说明集群未开启监控功能，可执行以下命令，开启该功能。

   ```bash
   kbcli cluster update elasticssearch --disable-exporter=false
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

4. 打开监控大盘网页控制台。

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

对于生产环境，强烈建议您搭建专属监控系统或者购买第三方监控服务，详情可参考[监控文档](./../observability/monitor-database.md#for-production-environment)。

## 扩缩容

### 水平扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容 API 文档](./../../api-docs/maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list mycluster
>
NAME        CLUSTER-DEFINITION          VERSION               TERMINATION-POLICY    STATUS    AGE
mycluster   elasticsearch               elasticsearch-8.8.2   Delete                Running   47m
```

#### 步骤

执行以下命令进行水平扩缩容。

```bash
kbcli cluster hscale elasticsearch --replicas=2 --components=elasticsearch
```

- `--components` 表示准备进行水平扩容的组件名称。
- `--replicas` 表示指定组件的副本数。 根据需要设定数值，进行扩缩容。

执行 `kbcli cluster hscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops elasticsearch-horizontalscaling-xpdwz -n demo
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe elasticsearch
```

### 垂直扩缩容

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale elasticsearch --cpu=2 --memory=3Gi --components=elasticsearch 
```

执行 `kbcli cluster vscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops elasticsearch-verticalscaling-rpw2l
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe elasticsearch
```

## 磁盘扩容

***步骤：***

```bash
kbcli cluster volume-expand elasticsearch --storage=40Gi --components=elasticsearch -t data
```

执行磁盘扩容任务可能需要几分钟。

执行 `kbcli cluster volume-expand` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops elasticsearch-volumeexpansion-5pbd2 -n default
```

也可通过以下命令，查看磁盘扩容任务是否完成。

```bash
kbcli cluster describe elasticsearch
```

## 重启集群

1. 执行以下命令，重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart elasticsearch --components="elasticsearch" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启是否成功。

   检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list elasticsearch
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION          VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   elasticsearch   default     elasticsearch               elasticsearch-8.8.2   Delete               Running   Jul 05,2024 17:51 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。
