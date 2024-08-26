---
title: 安装引擎
description: 使用 Helm 安装 KubeBlocks 支持的引擎
keywords: [引擎, helm, KubeBlocks]
sidebar_position: 3
sidebar_label: 安装引擎
---

# 安装引擎

1. （可选）添加 KubeBlocks 仓库。如果您使用 Helm 安装 KubeBlocks，只需执行 `helm repo update`。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. 查看引擎版本（以 MariaDB 为例）。

   ```bash
   helm search repo kubeblocks/mariadb --devel --versions
   ```

3. 安装引擎。

   ```bash
   helm install mariadb kubeblocks/mariadb --namespace kb-system --create-namespace --version 0.9.0
   ```

4. 验证该引擎是否安装成功。

   如果状态显示为 `deployed`，则表明该引擎已成功安装。

   ```bash
   helm list -A
   >
   NAME                        	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART                       	APP VERSION
   ......
   mariadb                     	kb-system	1       	2024-05-08 17:41:29.112721 +0800 CST   	deployed	mariadb-0.9.0               	10.6.15
   ```

5. （可选）您可以执行以下命令禁用引擎。

   ```bash
   helm uninstall mariadb --namespace kb-system
   ```
