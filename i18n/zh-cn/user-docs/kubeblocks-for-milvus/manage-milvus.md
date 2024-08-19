---
title: 用 KubeBlocks 管理 Milvus
description: 如何用 KubeBlocks 管理 Milvus
keywords: [milvus, 向量数据库]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Milvus
---

# 用 KubeBlocks 管理 Milvus

生成式人工智能的爆火引发了人们对向量数据库的关注。目前，KubeBlocks 支持 Milvus 的管理和运维。本文档展示如何使用 KubeBlocks 管理 Milvus。

Milvus 是高度灵活、可靠且速度极快的云原生开源矢量数据库。它为 embedding 相似性搜索和 AI 应用程序提供支持，并努力使每个组织都可以访问矢量数据库。 Milvus 可以存储、索引和管理由深度神经网络和其他机器学习 (ML) 模型生成的十亿级别以上的 embedding 向量。

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 milvus 引擎](./../overview/database-engines-supported.md#使用引擎)。

## 创建集群

***步骤：***

1. 创建一个 Milvus 集群。

   如需管理其他向量数据库，可将 `cluster-definition` 的值更改为其他的数据库。

   ```bash
   kbcli cluster create milvus --cluster-definition=milvus-2.3.2
   ```

2. 检查集群是否已创建。

   ```bash
   kbcli cluster list
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION        VERSION               TERMINATION-POLICY   STATUS            CREATED-TIME
   milvus   default     milvus-2.3.2              milvus-2.3.2          Delete               Creating          Jul 05,2024 17:35 UTC+0800
   ```

3. 查看集群信息。

   ```bash
   kbcli cluster describe milvus
   >
   Name: milvus	 Created Time: Jul 05,2024 17:35 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION   STATUS    TERMINATION-POLICY   
   default     milvus-2.3.2                   Running   Delete               

   Endpoints:
   COMPONENT   MODE        INTERNAL                                        EXTERNAL   
   milvus      ReadWrite   milvus-milvus.default.svc.cluster.local:19530   <none>     
   minio       ReadWrite   milvus-minio.default.svc.cluster.local:9000     <none>     
   proxy       ReadWrite   milvus-proxy.default.svc.cluster.local:19530    <none>     
                           milvus-proxy.default.svc.cluster.local:9091                

   Topology:
   COMPONENT   INSTANCE             ROLE     STATUS    AZ       NODE     CREATED-TIME                 
   etcd        milvus-etcd-0        <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   minio       milvus-minio-0       <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   milvus      milvus-milvus-0      <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   indexnode   milvus-indexnode-0   <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   mixcoord    milvus-mixcoord-0    <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   querynode   milvus-querynode-0   <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   datanode    milvus-datanode-0    <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   proxy       milvus-proxy-0       <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS     
   milvus      false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   etcd        false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   minio       false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   proxy       false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   mixcoord    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   datanode    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   indexnode   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   querynode   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   

   Images:
   COMPONENT   TYPE        IMAGE                                                
   milvus      milvus      milvusdb/milvus:v2.3.2                               
   etcd        etcd        docker.io/milvusdb/etcd:3.5.5-r2                     
   minio       minio       docker.io/minio/minio:RELEASE.2022-03-17T06-34-49Z   
   proxy       proxy       milvusdb/milvus:v2.3.2                               
   mixcoord    mixcoord    milvusdb/milvus:v2.3.2                               
   datanode    datanode    milvusdb/milvus:v2.3.2                               
   indexnode   indexnode   milvusdb/milvus:v2.3.2                               
   querynode   querynode   milvusdb/milvus:v2.3.2                               

   Show cluster events: kbcli cluster list-events -n default milvus
   ```

## 扩缩容

当前，KubeBlocks 支持垂直扩缩用 Milvus 集群。

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale milvus --cpu=1 --memory=1Gi --components=milvus 
```

`kbcli cluster vscale` 命令会打印输出 `opsname`。执行以下命令检查扩缩容进度：

```bash
kbcli cluster describe-ops milvus-verticalscaling-rpw2l -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe milvus
```

## 磁盘扩容

***步骤：***

```bash
kbcli cluster volume-expand milvus --storage=40Gi --components=milvus
```

这里需要等待几分钟，直到磁盘扩容完成。

`kbcli cluster volume-expand` 命令会打印输出 `opsname`。执行以下命令检查磁盘扩容进度：

```bash
kbcli cluster describe-ops milvus-volumeexpansion-5pbd2 -n default
```

也可通过以下命令，查看磁盘扩容是否已经完成。

```bash
kbcli cluster describe milvus
```

## 重启

1. 重启集群。

   配置 `--components` 和 `--ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart milvus --components="milvus" \
   --ttlSecondsAfterSucceed=30
   ```

   - `--components` 表示需要重启的组件名称。
   - `--ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list milvus
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
   milvus   default     milvus-2.3.2           milvus-2.3.2    Delete               Running   Jul 05,2024 18:35 UTC+0800
   ```

   * STATUS=Updating 表示集群正在重启中。
   * STATUS=Running 表示集群已重启。

## 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

### 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   ```bash
   kbcli cluster stop milvus
   ```

2. 查看集群状态，确认集群是否已停止。

    ```bash
    kbcli cluster list
    ```

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   ```bash
   kbcli cluster start milvus
   ```

2. 查看集群状态，确认集群是否已再次运行。

    ```bash
    kbcli cluster list
    ```
