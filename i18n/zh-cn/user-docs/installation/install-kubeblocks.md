# 用 kbcli 安装 KubeBlocks
创建一个新的 Kubernetes 集群，并使用 Playground 安装 KubeBlocks，这是快速入门 KubeBlocks 的一种方式。然而，在实际生产环境中，情况会复杂得多，应用程序在不同的命名空间中运行，还存在资源或权限限制。本文档将介绍如何在现有的 Kubernetes 集群上部署 KubeBlocks。

## 环境准备
准备一个可访问的 Kubernetes 集群，版本要求 1.22 及以上。该集群应满足如下要求。

<table>
    <tr>
        <th colspan="3">资源要求</th>
    </tr >
    <tr>
        <td >控制面</td>
        <td colspan="2">建议创建 1 个具有 4 核 CPU、4GB 内存和 50GB 存储空间的节点。 </td>
    </tr >
    <tr >
        <td rowspan="4">数据面</td>
        <td> MySQL </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> PostgreSQL </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> Redis </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> MongoDB </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
</table>

## 安装步骤

执行 `kbcli kubeblocks install` 将 KubeBlocks 安装在 `kb-system` 命名空间中，或者使用 `--namespace` 标记来指定一个命名空间。

```kbcli kubeblocks install
```

如果想安装 KubeBlocks 的指定版本，请按照以下步骤操作：

1. 查看可用的版本。
```
kbcli kubeblocks list-versions
```
或者，你可以在 KubeBlocks Release 页面中查看可用的版本。

2. 使用--version指定版本，并执行以下命令。

```
kbcli kubeblocks install --version=x.x.x
```

  :::note

kbcli 默认安装最新版本。在安装 KubeBlocks 时，kbcli 会安装与之匹配的版本。请确保 kbcli 和 KubeBlocks 的主版本号相匹配。
例如，你可以安装 kbcli v0.6.1 和 KubeBlocks v0.6.3。但是，如果安装的是 kbcli v0.5.0 和 KubeBlocks v0.6.0，就可能会报错，因为它们不匹配。
  
  :::

## 检查 KubeBlocks 是否安装成功

执行以下命令来检查 KubeBlocks 是否已成功安装。

kbcli kubeblocks status

*结果*
如果工作负载都已准备就绪，则表明 KubeBlocks 已成功安装。
```
KubeBlocks is deployed in namespace: kb-system,version: x.x.x
>
KubeBlocks Workloads:
NAMESPACE   KIND         NAME                           READY PODS   CPU(CORES)   MEMORY(BYTES)   CREATED-AT
kb-system   Deployment   kb-addon-snapshot-controller   1/1          N/A          N/A             Oct 13,2023 14:27 UTC+0800
kb-system   Deployment   kubeblocks                     1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
kb-system   Deployment   kubeblocks-dataprotection      1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
```