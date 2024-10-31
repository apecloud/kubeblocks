---
title: 用 kbcli 安装 KubeBlocks
description: 如何用 kbcli 安装 KubeBlocksInstall KubeBlocks
keywords: [taints, affinity, tolerance, 安装, kbcli, KubeBlocks]
sidebar_position: 2
sidebar_label: 用 kbcli 安装 KubeBlocks
---

# 用 kbcli 安装 KubeBlocks

使用 Playground 创建一个新的 Kubernetes 集群并安装 KubeBlocks，是快速上手的一种方法。然而，在实际生产环境中，情况会复杂得多，应用程序在不同的命名空间中运行，还存在资源或权限限制。本文档将介绍如何在现有的 Kubernetes 集群上部署 KubeBlocks。

如果您想要在本地环境试用 KubeBlocks，可通过 [Playground](./../../try-out-on-playground/try-kubeblocks-on-local-host.md) 试用，或者[先在本地创建 Kubernetes 测试集群](./../prerequisite/prepare-a-local-k8s-cluster.md)，然后按照本文操作步骤安装 KubeBlocks。

## 环境准备

准备一个可访问的 Kubernetes 集群，版本要求 1.22 及以上。该集群应满足如下要求。

<table>
    <tr>
        <th colspan="3">资源要求</th>
    </tr >
    <tr>
        <td >控制面</td>
        <td colspan="2">建议创建 1 个具有 4 核 CPU、4 GB 内存和 50 GB 存储空间的节点。 </td>
    </tr >
    <tr >
        <td rowspan="4">数据面</td>
        <td> MySQL </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4 GB 内存和 50 GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> PostgreSQL </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4 GB 内存和 50 GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> Redis </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4 GB 内存和 50 GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> MongoDB </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4 GB 内存和 50 GB 存储空间的节点。</td>
    </tr>
</table>

## 安装步骤

执行 `kbcli kubeblocks install` 将 KubeBlocks 安装在 `kb-system` 命名空间中，或者使用 `--namespace` 指定一个命名空间。

```bash
kbcli kubeblocks install
```

如果想安装 KubeBlocks 的指定版本，请按照以下步骤操作：

1. 查看可用的版本。

    ```bash
    kbcli kubeblocks list-versions
    ```

    如需查看包含 alpha 和 beta 在内的版本，可执行以下命令。

    ```bash
    kbcli kb list-versions --devel --limit=100
    ```

    或者，你可以在 [KubeBlocks Release 页面](https://github.com/apecloud/kubeblocks/releases/)中查看可用的版本。

2. 使用 `--version` 指定版本。

    ```bash
    kbcli kubeblocks install --version=x.x.x
    ```

    :::note

    kbcli 默认安装最新版本。如果您的环境中已有正在运行的 KubeBlocks 实例，则需要安装与之匹配的 kbcli 版本。

    例如，如果您当前使用的 KubeBlocks 版本是 v0.8.3，kbcli 应安装对应的 v0.8.3，而不是更高版本（如 v0.9.0），否则系统将因版本不匹配产生报错。

    :::

## 验证 KubeBlocks 安装

执行以下命令来检查 KubeBlocks 是否已成功安装。

```bash
kbcli kubeblocks status
```

***结果***

如果工作负载都显示已准备就绪，则表明已成功安装 KubeBlocks。

```bash
KubeBlocks is deployed in namespace: kb-system,version: x.x.x
>
KubeBlocks Workloads:
NAMESPACE   KIND         NAME                           READY PODS   CPU(CORES)   MEMORY(BYTES)   CREATED-AT
kb-system   Deployment   kb-addon-snapshot-controller   1/1          N/A          N/A             Oct 13,2023 14:27 UTC+0800
kb-system   Deployment   kubeblocks                     1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
kb-system   Deployment   kubeblocks-dataprotection      1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
```
