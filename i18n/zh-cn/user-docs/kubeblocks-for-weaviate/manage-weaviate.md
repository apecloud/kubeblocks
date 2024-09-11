---
title: 用 KubeBlocks 管理 Weaviate
description: 如何用 KubeBlocks 管理 Weaviate
keywords: [weaviate, 向量数据库]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Weaviate
---

# 用 KubeBlocks 管理 Weaviate

生成式人工智能的爆火引发了人们对向量数据库的关注。Weaviate 是开源向量数据库，可简化人工智能应用程序的开发。内置向量和混合搜索、易于连接的机器学习模型以及对数据隐私的关注使各级开发人员能够更快地构建、迭代和扩展 AI 功能。

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 weaviate 引擎](./../overview/database-engines-supported.md#使用引擎)。

## 创建集群

***步骤：***

1. 创建集群

   ```bash
   kbcli cluster create weaviate --cluster-definition=weaviate
   ```

   如需创建多副本 Weaviate 集群，可使用以下命令，设置副本数量。

   ```bash
   kbcli cluster create weaviate --cluster-definition=weaviate --set replicas=3
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
   NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS           CREATED-TIME
   weaviate        default     weaviate             weaviate-1.18.0       Delete               Running          Jul 05,2024 17:42 UTC+0800   
   ```

3. 查看集群信息。

   ```bash
    kbcli cluster describe weaviate
    >
    Name: weaviate	 Created Time: Jul 05,2024 17:42 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
    default     weaviate             weaviate-1.18.0   Running   Delete

    Endpoints:
    COMPONENT   MODE        INTERNAL                                           EXTERNAL
    weaviate    ReadWrite   weaviate-weaviate.default.svc.cluster.local:8080   <none>

    Topology:
    COMPONENT   INSTANCE              ROLE     STATUS    AZ       NODE                    CREATED-TIME
    weaviate    weaviate-weaviate-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 05,2024 17:42 UTC+0800

    Resources Allocation:
    COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    weaviate    false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT   TYPE       IMAGE
    weaviate    weaviate   docker.io/semitechnologies/weaviate:1.19.6

    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   RECOVERABLE-TIME 
   ```

## 连接集群

Weaviate 提供 HTTP 访问协议，使用端口 8080 进行通信。您可通过本地主机访问集群。

```bash
curl http://localhost:8080/v1/meta | jq
```

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

2. 查看集群监控功能是否开启。可通过查看集群 YAML 文件中是否显示 `disableExporter: false`，如果有该字段，则说明集群监控功能已开启。

   ```bash
   kubectl get cluster weaviate -o yaml
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
   kbcli cluster update weaviate --disable-exporter=false
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

对于生产环境，强烈建议您搭建专属监控系统或者购买第三方监控服务，详情可参考[监控文档](./../observability/monitor-database.md#生产环境)。

## 扩缩容

### 水平扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容 API 文档](./../../api-docs/maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list weaviate
>
NAME       NAMESPACE   CLUSTER-DEFINITION     VERSION            TERMINATION-POLICY   STATUS    CREATED-TIME
weaviate   default     weaviate               weaviate-1.18.0    Delete               Running   Jul 24,2023 11:38 UTC+0800
```

#### 步骤

执行以下命令进行水平扩缩容。

```bash
kbcli cluster hscale weaviate --replicas=3 --components=weaviate
```

- `--components` 表示准备进行水平扩容的组件名称。
- `--replicas` 表示指定组件的副本数。 根据需要设定数值，进行扩缩容。

执行 `kbcli cluster hscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops weaviate-horizontalscaling-xpdwz -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe weaviate
```

### 垂直扩缩容

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list weaviate
>
NAME       NAMESPACE   CLUSTER-DEFINITION     VERSION            TERMINATION-POLICY   STATUS    CREATED-TIME
weaviate   default     weaviate               weaviate-1.18.0    Delete               Running   Jul 24,2023 11:38 UTC+0800
```

#### 步骤

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale weaviate --cpu=0.5 --memory=512Mi --components=weaviate 
```

执行 `kbcli cluster vscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops weaviate-verticalscaling-rpw2l -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe weaviate
```

## 磁盘扩容

***步骤：***

```bash
kbcli cluster volume-expand weaviate --storage=40Gi --components=weaviate -t data
```

执行磁盘扩容任务可能需要几分钟。

执行 `kbcli cluster volume-expand` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops weaviate-volumeexpansion-5pbd2 -n default
```

也可通过以下命令，查看磁盘扩容任务是否完成。

```bash
kbcli cluster describe weaviate
```

## 重启集群

1. 执行以下命令，重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart weaviate --components="weaviate" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启是否成功。

   检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list weaviate
   >
   NAME       NAMESPACE   CLUSTER-DEFINITION     VERSION            TERMINATION-POLICY   STATUS    CREATED-TIME
   weaviate   default     weaviate               weaviate-1.18.0    Delete               Running   Jul 05,2024 18:42 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

## 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

## 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   ```bash
   kbcli cluster stop weaviate
   ```

2. 查看集群状态，确认集群是否已停止。

    ```bash
    kbcli cluster list
    ```

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   ```bash
   kbcli cluster start weaviate
   ```

2. 查看集群状态，确认集群是否已再次运行。

    ```bash
    kbcli cluster list
    ```
