---
title: 用 KubeBlocks 管理 StarRocks
description: 如何使用 KubeBlocks 管理 StarRocks
keywords: [starrocks, 分析型数据库, data warehouse]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 StarRocks
---

# 用 KubeBlocks 管理 StarRocks

StarRocks 是一款高性能分析型数据仓库，使用向量化、MPP 架构、CBO、智能物化视图、可实时更新的列式存储引擎等技术实现多维、实时、高并发的数据分析。

KubeBlocks supports the management of StarRocks.

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 starrocks 引擎](./../overview/supported-addons.md#use-addons)。

## 创建集群

***步骤：***

1. 执行以下命令，创建 StarRocks 集群。

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks
   ```

   您可使用 `--set` 指定 CPU、memory、存储的值。

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks --set cpu=1,memory=2Gi,storage=10Gi
   ```

:::note

执行以下命令，查看更多集群创建的选项和默认值。
  
```bash
kbcli cluster create --help
```

:::

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   default     starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800   
   ```

3. 查看集群信息。

   ```bash
    kbcli cluster describe mycluster
    >
    Name: mycluster	 Created Time: Jul 17,2024 19:06 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
    default     starrocks            starrocks-3.1.1   Running   Delete

    Endpoints:
    COMPONENT   MODE        INTERNAL                                      EXTERNAL
    fe          ReadWrite   mycluster-fe.default.svc.cluster.local:9030   <none>

    Topology:
    COMPONENT   INSTANCE         ROLE     STATUS    AZ       NODE                    CREATED-TIME
    be          mycluster-be-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800
    fe          mycluster-fe-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800

    Resources Allocation:
    COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    fe          false       1 / 1                1Gi / 1Gi               data:20Gi      standard
    be          false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT   TYPE   IMAGE
    fe          fe     docker.io/starrocks/fe-ubuntu:2.5.4
    be          be     docker.io/starrocks/be-ubuntu:2.5.4

    Show cluster events: kbcli cluster list-events -n default mycluster
   ```

## 扩缩容

### 垂直扩缩容

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   default     starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800  
```

#### 步骤

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale mycluster --cpu=2 --memory=20Gi --components=be
```

这里需要等待几秒钟，直到扩缩容完成。

`kbcli cluster vscale` 命令会打印输出 `opsname`。执行以下命令检查扩缩容进度：

```bash
kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe mycluster
```

### 水平扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容 API 文档](./../../api-docs/maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   default     starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800   
```

#### 步骤

执行以下命令进行水平扩缩容。

```bash
kbcli cluster hscale mycluster --replicas=3 --components=be
```

- `--components` 表示准备进行水平扩容的组件名称。
- `--replicas` 表示指定组件的副本数。根据需要设定数值，进行扩缩容。

执行 `kbcli cluster hscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops mycluster-horizontalscaling-smx8b -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe mycluster
```

## 磁盘扩容

执行以下命令进行磁盘扩容。

```bash
kbcli cluster volume-expand mycluster --storage=40Gi --components=be
```

执行磁盘扩容任务可能需要几分钟。

执行 `kbcli cluster volume-expand` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops mycluster-volumeexpansion-smx8b -n default
```

也可通过以下命令，查看磁盘扩容任务是否完成。

```bash
kbcli cluster describe mycluster
```

## 重启集群

1. 执行以下命令，重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart mycluster --components="starrocks" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启是否成功。

   检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     starrocks              starrocks-3.1.1    Delete               Running   Jul 17,2024 19:06 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

## 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

## 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   ```bash
   kbcli cluster stop mycluster
   ```

2. 查看集群状态，确认集群是否已停止。

    ```bash
    kbcli cluster list
    ```

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   ```bash
   kbcli cluster start mycluster
   ```

2. 查看集群状态，确认集群是否已再次运行。

    ```bash
    kbcli cluster list
    ```
