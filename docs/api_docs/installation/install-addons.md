---
title: Install Addons
description: Install KubeBlocks addons with Helm
keywords: [addon, helm, KubeBlocks]
sidebar_position: 3
sidebar_label: Install KubeBlocks
---

# Install addons

1. (Optional) Add the KubeBlocks repo. If you install KubeBlocks with Helm, just run `helm repo update`.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. View the addon versions.

   ```bash
   helm search repo kubeblocks/mariadb --devel --versions
   ```

3. Install the addon.

   ```bash
   helm install mariadb kubeblocks/mariadb --namespace kb-system --create-namespace --version 0.9.0
   ```

4. Verify whether this addon is installed.

   The STATUS is `deployed` and this addon is installed successfully.

   ```bash
   helm list -A
   >
   NAME                        	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART                       	APP VERSION
   ......
   mariadb                     	kb-system	1       	2024-05-08 17:41:29.112721 +0800 CST   	deployed	mariadb-0.9.0               	10.6.15
   ```

5. (Optional) You can run the command below to disable the addon.

   ```bash
   helm uninstall mariadb --namespace kb-system
   ```
