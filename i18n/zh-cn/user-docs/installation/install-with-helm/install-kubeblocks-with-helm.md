---
title: 用 Helm 安装 KubeBlocks
description: 如何用 Helm 安装 KubeBlocks
keywords: [taints, affinity, tolerance, 安装, helm, KubeBlocks]
sidebar_position: 1
sidebar_label: 用 Helm 安装 KubeBlocks
---

# 用 Helm 安装 KubeBlocks

KubeBlocks 是基于 Kubernetes 的原生应用，你可以使用 Helm 来进行安装。

:::note

如果使用 Helm 安装 KubeBlocks，那么卸载也需要使用 Helm。

请确保已安装 [kubectl](https://kubernetes.io/zh-cn/docs/tasks/tools/) 和 [Helm](https://helm.sh/zh/docs/intro/install/)。

:::

## 环境准备

<table>
    <tr>
        <th colspan="3">资源要求</th>
    </tr >
    <tr>
        <td >控制面</td>
        <td colspan="2">建议创建 1 个具有 4 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr >
    <tr >
        <td rowspan="4">数据面</td>
        <td> MySQL </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。 </td>
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

**使用 Helm 安装 KubeBlocks**

1. 创建 CRD 依赖。

   ```bash
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.8.1/kubeblocks_crds.yaml
   ```

2. 添加 Helm 仓库。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   
   helm repo update
   ```

3. 安装 KubeBlocks。

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   如果想要使用自定义的 tolerations 安装 KubeBlocks，可以使用以下命令：

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
    --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
    --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   如果想安装 KubeBlocks 的指定版本，请按照以下步骤操作：

     1. 在 [KubeBlocks Release 页面](https://github.com/apecloud/kubeblocks/releases/)查看可用的版本。
     2. 使用 `--version` 指定版本，并执行以下命令。


         ```bash
         helm install kubeblocks kubeblocks/kubeblocks \
         --namespace kb-system --create-namespace --version="x.x.x"
         ```

:::note

默认安装最新版本。

:::

## 验证 KubeBlocks 安装

执行以下命令来检查 KubeBlocks 是否已成功安装。

```bash
kbcli kubeblocks status
```

***结果***

如果工作负载都已准备就绪，则表明 KubeBlocks 已成功安装。

```bash
NAME                                            READY   STATUS    RESTARTS      AGE
kb-addon-snapshot-controller-649f8b9949-2wzzk   1/1     Running   2 (24m ago)   147d
kubeblocks-dataprotection-f6dbdbf7f-5fdr9       1/1     Running   2 (24m ago)   147d
kubeblocks-6497f7947-mc7vc                      1/1     Running   2 (24m ago)   147d
```