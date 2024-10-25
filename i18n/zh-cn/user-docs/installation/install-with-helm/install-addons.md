---
title: 安装引擎
description: 使用 Helm 安装 KubeBlocks 支持的引擎
keywords: [引擎, helm, KubeBlocks]
sidebar_position: 3
sidebar_label: 安装引擎
---

# 安装引擎

KubeBlocks v0.8.0 发布后，引擎（Addon）与 KubeBlocks 解耦，KubeBlocks 默认安装了部分引擎，如需体验其它引擎，需通过索引安装相关引擎。如果您卸载了部分引擎，也可通过本文步骤，重新安装。

本文以 etcd 为例，可根据实际情况替换引擎名称。

官网引擎索引仓库为 [KubeBlocks index](https://github.com/apecloud/block-index)。引擎代码维护在 [KubeBlocks addon repo](https://github.com/apecloud/kubeblocks-addons)。

1. （可选）添加 KubeBlocks 仓库。如果您使用 Helm 安装了 KubeBlocks，只需执行 `helm repo update`。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. 查看引擎版本。

   ```bash
   helm search repo kubeblocks/etcd --devel --versions
   ```

3. 安装引擎（以 etcd 为例）。

   ```bash
   helm install mariadb kubeblocks/etcd --namespace kb-system --create-namespace --version 0.9.0
   ```

4. 验证该引擎是否安装成功。

   如果状态显示为 `deployed`，则表明该引擎已成功安装。

   ```bash
   helm list -A
   >
   NAME                 NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART              APP VERSION
   ......
   etcd               	kb-system	1       	2024-10-25 07:18:35.294326176 +0000 UTC	deployed	etcd-0.9.0         v3.5.6
   ```

5. （可选）您可以执行以下命令禁用引擎。

   ```bash
   helm uninstall etcd --namespace kb-system
   ```
