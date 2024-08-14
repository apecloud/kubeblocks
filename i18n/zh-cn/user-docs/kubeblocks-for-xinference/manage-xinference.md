---
title: 用 KubeBlocks 管理 Xinference
description: 如何用 KubeBlocks 管理 Xinference
keywords: [xinference, LLM, AI]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Xinference
---

# 用 KubeBlocks 管理 Xinference

Xorbits Inference (Xinference) 是一个开源平台，用于简化各种 AI 模型的运行和集成。借助 Xinference，您可以使用任何开源 LLM、嵌入模型和多模态模型在云端或本地环境中运行推理，并创建强大的 AI 应用。

## 开始之前

- [安装 kbcli](./../installation/install-with-kbcli/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
- [安装并启用 xinference 引擎](./../overview/supported-addons.md#use-addons)。

## 创建集群

***步骤：***

1. 创建集群。

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference
   ```

   如需创建多副本 Weaviate 集群，可使用以下命令，设置副本数量。

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference --set replicas=3
   ```

   您也可使用 `--set` 指定 CPU、memory、存储的值。

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference --set cpu=1,memory=2Gi,storage=10Gi
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
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     xinference           xinference-0.11.0   Delete               Running   Jul 17,2024 17:24 UTC+0800   
   ```

3. 查看集群信息。

   ```bash
    kbcli cluster describe mycluster
    >
    Name: mycluster	 Created Time: Jul 17,2024 17:29 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION             STATUS    TERMINATION-POLICY
    default     xinference           xinference-0.11.0   Running   Delete

    Endpoints:
    COMPONENT    MODE        INTERNAL                                              EXTERNAL
    xinference   ReadWrite   mycluster-xinference.default.svc.cluster.local:9997   <none>

    Topology:
    COMPONENT    INSTANCE                 ROLE     STATUS    AZ       NODE                    CREATED-TIME
    xinference   mycluster-xinference-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 17:29 UTC+0800

    Resources Allocation:
    COMPONENT    DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    xinference   false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT    TYPE         IMAGE
    xinference   xinference   docker.io/xprobe/xinference:v0.11.0

    Show cluster events: kbcli cluster list-events -n default mycluster
   ```

## 垂直扩缩容

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kbcli cluster list
>
   NAME         NAMESPACE   CLUSTER-DEFINITION     VERSION              TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster    default     xinference             xinference-0.11.0    Delete               Running   Jul 05,2024 17:29 UTC+0800
```

#### 步骤

执行以下命令进行垂直扩缩容。

```bash
kbcli cluster vscale mycluster --cpu=0.5 --memory=512Mi --components=xinference 
```

执行 `kbcli cluster vscale` 后会输出一条 ops 相关命令，可使用该命令查看扩缩容任务进度。

```bash
kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n default
```

也可通过以下命令，查看扩缩容任务是否完成。

```bash
kbcli cluster describe mycluster
```

## 重启集群

1. 执行以下命令，重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart mycluster --components="xinference" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启是否成功。

   检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list mycluster
   >
   NAME         NAMESPACE   CLUSTER-DEFINITION     VERSION              TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster    default     xinference             xinference-0.11.0    Delete               Running   Jul 05,2024 18:42 UTC+0800
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
