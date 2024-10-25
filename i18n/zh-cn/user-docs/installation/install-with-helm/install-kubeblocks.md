---
title: 安装 KubeBlocks
description: 在现有的 Kubernetes 集群上使用 Helm 安装 KubeBlocks
keywords: [污点, 亲和性, 容忍, 安装, kbcli, KubeBlocks]
sidebar_position: 1
sidebar_label: 安装 KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 安装 KubeBlocks 

KubeBlocks 是 Kubernetes 原生 operator，可通过 Helm 或者 kubectl 应用 YAML 文件安装 KubeBlocks。本文将介绍如何在现有的 Kubernetes 集群上部署 KubeBlocks。

如果您想要在本地环境试用 KubeBlocks，可通过 [Playground](./../../try-out-on-playground/try-kubeblocks-on-local-host.md) 试用，或者[先在本地创建 Kubernetes 测试集群](./../prerequisite/prepare-a-local-k8s-cluster.md)，然后按照本文操作步骤安装 KubeBlocks。

:::note

如果您使用 Helm 安装 KubeBlocks，卸载时也需使用 Helm。

请确保您已安装 [kubectl](https://kubernetes.io/docs/tasks/tools/) 和 [Helm](https://helm.sh/docs/intro/install/)。

:::

## 环境准备

准备一个可访问的 Kubernetes 集群，版本要求 1.22 及以上。该集群应满足如下要求。

<table>
	<tr>
	    <th colspan="3">资源要求</th>
	</tr >
	<tr>
	    <td >控制面</td>
	    <td colspan="2">建议创建 1 个具有 4 核 CPU、4 GB 内存和 50 GB 存储空间的节点。</td>
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

<Tabs>

<TabItem value="Helm" label="Helm" default>

按照以下步骤使用 Helm 安装 KubeBlocks。

1. 创建安装所依赖的 CRDs，并指定您想要安装的版本。

   ```bash
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.z/kubeblocks_crds.yaml
   ```

   您可以通过 [KubeBlocks 发布列表](https://github.com/apecloud/kubeblocks/releases) 查看 KubeBlocks 的所有版本，包括 alpha 及 beta 版本。

   也可通过执行以下命令，获取稳定版本：

   ```bash
   curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r
   ```

2. 添加 KubeBlocks 的 Helm 仓库。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

3. 安装 KubeBlocks。

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   如果您想要在安装 KubeBlocks 添加自定义容忍，可使用以下命令：

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
     --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
     --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   如果您想要安装指定版本的 KubeBlocks，可执行如下步骤：

   1. 在 [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/) 中查看可用版本。
   2. 使用 `--version` 指定版本，并执行以下命令。

      ```bash
      helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="x.y.z"
      ```

     :::note

     如果不指定版本，默认安装最新版本。

     :::

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks 可以像 Kubernetes 中的其他资源一样，通过 YAML 文件和 kubectl 命令进行安装。

执行以下命令，安装当前小版本发布的最新 operator。

 ```bash
 kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.x/kubeblocks.yaml
 ```

您可通过 [KubeBlocks 发布页资产](https://github.com/apecloud/kubeblocks/releases) 中获取 YAML 文件地址。

</TabItem>

</Tabs>

## 验证 KubeBlocks 安装

执行以下命令，检查 KubeBlocks 是否已成功安装。

```bash
kubectl -n kb-system get pods
```

***结果***

如果工作负载都显示已处于 Running 状态，则表明已成功安装 KubeBlocks。

```bash
NAME                                             READY   STATUS    RESTARTS       AGE
alertmanager-webhook-adaptor-8dfc4878d-svzrc     2/2     Running   0              3m56s
grafana-77dfd6959-4gnhp                          1/1     Running   0              3m56s
kb-addon-snapshot-controller-546f84b78d-8rjs4    1/1     Running   0              3m56s
kubeblocks-7cf7745685-ddlwk                      1/1     Running   0              4m39s
kubeblocks-dataprotection-95fbc79cc-b544l        1/1     Running   0              4m39s
prometheus-alertmanager-5c9fc88996-qltrk         2/2     Running   0              3m56s
prometheus-kube-state-metrics-5dbbf757f5-db9v6   1/1     Running   0              3m56s
prometheus-node-exporter-r6kvl                   1/1     Running   0              3m56s
prometheus-pushgateway-8555888c7d-xkgfc          1/1     Running   0              3m56s
prometheus-server-5759b94fc8-686xp               2/2     Running   0              3m56s
```
