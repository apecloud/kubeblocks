---
title: 安装 KubeBlocks
description: 在现有的 Kubernetes 集群上使用 Helm 安装 KubeBlocks
keywords: [污点, 亲和性, 容忍, 安装, kbcli, KubeBlocks]
sidebar_position: 3
sidebar_label: 安装 KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 安装 KubeBlocks

使用 Playground 创建一个新的 Kubernetes 集群并安装 KubeBlocks，是快速上手的一种方法。然而，在实际生产环境中，情况会复杂得多，应用程序在不同的命名空间中运行，还存在资源或权限限制。本文档将介绍如何在现有的 Kubernetes 集群上部署 KubeBlocks。

KubeBlocks 是 Kubernetes 原生 operator，可通过 Helm、 kubectl 应用 YAML 文件或者 kbcli 安装 KubeBlocks。

如果您想要在本地环境试用 KubeBlocks，可通过 [Playground](./../try-out-on-playground/try-kubeblocks-on-local-host.md) 试用，或者[先在本地创建 Kubernetes 测试集群](./prepare-a-local-k8s-cluster/prepare-a-local-k8s-cluster.md)，然后按照本文操作步骤安装 KubeBlocks。

:::note

- 请确保您安装和卸载 KubeBlocks 使用的方式保持一致，例如，如果您使用 Helm 安装 KubeBlocks，卸载时也需使用 Helm。
- 请确保您已安装 [kubectl](https://kubernetes.io/docs/tasks/tools/)，[Helm](https://helm.sh/docs/intro/install/) 或 [kbcli](./install-kbcli.md)。
- 请确保您已安装 Snapshot Controller。如果尚未安装，请按照 [安装 Snapshot Controller](#安装-kubeblocks) 部分中的步骤安装。

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

## 安装 Snapshot Controller

SnapshotController 是一个 Kubernetes 组件，用于管理 CSI 卷快照，可让用户创建、恢复和删除持久卷 (PV) 的快照。KubeBlocks DataProtection 控制器会使用 Snapshot Controller 来为数据库创建快照备份。

如果您的 Kubernetes 集群没有以下 CRD，说明您可能没有部署 Snapshot Controller。

```bash
kubectl get crd volumesnapshotclasses.snapshot.storage.k8s.io
kubectl get crd volumesnapshots.snapshot.storage.k8s.io
kubectl get crd volumesnapshotcontents.snapshot.storage.k8s.io
```

:::note

如果您确定不需要使用快照备份功能, 可以只安装SnapshotController CRD, 跳过后续步骤。

```bash
# v8.2.0 is the latest version of the external-snapshotter, you can replace it with the version you need.
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
```

:::

### 第 1 步：部署 Snapshot Controller

可以通过 Helm 或者 kubectl 安装 Snapshot Controller。以下演示如何使用 Helm 安装：

```bash
helm repo add piraeus-charts https://piraeus.io/helm-charts/
helm repo update
# Update the namespace to an appropriate value for your environment (e.g. kb-system)
helm install snapshot-controller piraeus-charts/snapshot-controller -n kb-system --create-namespace
```

如需更多信息，请参阅 [Snapshot Controller Configuration](https://artifacthub.io/packages/helm/piraeus-charts/snapshot-controller#configuration)。

### 第 2 步：验证 Snapshot Controller 是否安装成功

检查 snapshot-controller Pod 是否正在运行：

```bash
kubectl get pods -n kb-system | grep snapshot-controller
```

<details>

<summary>Output</summary>

```bash
snapshot-controller-xxxx-yyyy   1/1   Running   0   30s
```

</details>

如果该 Pod 处于 CrashLoopBackOff 状态，请查看日志：

```bash
kubectl logs -n kb-system deployment/snapshot-controller
```

## 安装KubeBlocks

<Tabs>

<TabItem value="Helm" label="Helm" default>

按照以下步骤使用 Helm 安装 KubeBlocks：

1. 获取 KubeBlocks 版本:

   * 选项 A - 获取最新稳定版本(例如 v0.9.2):
   ```bash
   curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r | head -n 1
   ```

   * 选项 B - 查看所有可用版本(包括 alpha 和 beta 版本):
     * 访问 [KubeBlocks 发布列表](https://github.com/apecloud/kubeblocks/releases)。
     * 或使用命令:
     ```bash
     curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[].tag_name' | sort -V -r
     ```

2. 使用选择的版本创建所需的 CRDs:
   ```bash
   # 将 <VERSION> 替换为您选择的版本
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks_crds.yaml

   # 示例:如果版本是 v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks_crds.yaml
   ```

3. 添加 KubeBlocks 的 Helm 仓库：

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

4. 安装 KubeBlocks：

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   如果您想要在安装 KubeBlocks 添加自定义容忍度，可使用以下命令：

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
     --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
     --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   如果您想要安装指定版本的 KubeBlocks，可执行如下步骤：

   1. 在 [KubeBlocks 发布列表](https://github.com/apecloud/kubeblocks/releases/) 中查看可用版本。
   2. 使用 `--version` 指定版本，并执行以下命令：

      ```bash
      helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version=<VERSION>
      ```

     :::note

     如果不指定版本，默认安装最新版本。

     :::

</TabItem>

<TabItem value="kubectl" label="kubectl">

与 Kubernetes 中的其他资源相同，KubeBlocks 也可以通过 YAML 文件和 kubectl 命令进行安装。

1. 获取 KubeBlocks 版本:

   * 选项 A - 获取最新稳定版本(例如 v0.9.2):
   ```bash
   curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r | head -n 1
   ```

   * 选项 B - 查看所有可用版本(包括 alpha 和 beta 版本):
      * 访问 [KubeBlocks 发布列表](https://github.com/apecloud/kubeblocks/releases)。
      * 或使用命令:
     ```bash
     curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[].tag_name' | sort -V -r
     ```

2. 使用选择的版本创建所需的 CRDs:
   ```bash
   # 将 <VERSION> 替换为您选择的版本
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks_crds.yaml

   # 示例:如果版本是 v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks_crds.yaml
   ```

3. 安装 KubeBlocks:

   ```bash
   # 将 <VERSION> 替换为在步骤 2 中使用的相同版本
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks.yaml

   # 示例:如果版本是 v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks.yaml
   ```

   :::note

   请确保创建 CRDs 和安装 KubeBlocks 时使用相同版本以避免兼容性问题。

   :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

执行 `kbcli kubeblocks install` 将 KubeBlocks 安装在 `kb-system` 命名空间中，或者使用 `--namespace` 指定一个命名空间。

```bash
kbcli kubeblocks install
```

如果想安装 KubeBlocks 的指定版本，请按照以下步骤操作：

1. 查看可用的版本：

    ```bash
    kbcli kubeblocks list-versions
    ```

    如需查看包含 alpha 和 beta 在内的版本，可执行以下命令：

    ```bash
    kbcli kb list-versions --devel --limit=100
    ```

    或者，你可以在 [KubeBlocks 发布列表](https://github.com/apecloud/kubeblocks/releases/)中查看可用的版本。

2. 使用 `--version` 指定版本：

    ```bash
    kbcli kubeblocks install --version=<VERSION>
    ```

    :::note

    kbcli 默认安装最新版本。如果您的环境中已有正在运行的 KubeBlocks 实例，则需要安装与之匹配的 kbcli 版本。

    例如，如果您当前使用的 KubeBlocks 版本是 v0.9.2，kbcli 应安装对应的 v0.9.2，而不是更高版本（如 v1.0.0），否则系统将因版本不匹配产生报错。

    :::

</TabItem>

</Tabs>

## 验证 KubeBlocks 安装

执行以下命令，检查 KubeBlocks 是否已成功安装：

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

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

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli kubeblocks status
```

***结果***

如果工作负载都显示已准备就绪，则表明已成功安装 KubeBlocks。

```bash
KubeBlocks is deployed in namespace: kb-system, version: <VERSION>
>
KubeBlocks Workloads:
NAMESPACE   KIND         NAME                           READY PODS   CPU(CORES)   MEMORY(BYTES)   CREATED-AT
kb-system   Deployment   kb-addon-snapshot-controller   1/1          N/A          N/A             Oct 13,2023 14:27 UTC+0800
kb-system   Deployment   kubeblocks                     1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
kb-system   Deployment   kubeblocks-dataprotection      1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
```

</TabItem>

</Tabs>
