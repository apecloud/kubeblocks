---
title: Install Addons
description: Install KubeBlocks addons with Helm
keywords: [addon, helm, KubeBlocks]
sidebar_position: 3
sidebar_label: Install Addons
---

# Install Addons

With the release of KubeBlocks v0.8.0, Addons are decoupled from KubeBlocks and some Addons are not installed by default. If you want to use these Addons, install Addons first by index. Or if you uninstalled some Addons, you can follow the steps in this tutorial to install them again.

This tutorial takes etcd as an example. You can replace etcd with the Addon you need.

The official index repo is [KubeBlocks index](https://github.com/apecloud/block-index). The code of all Addons is maintained in the [KubeBlocks Addon repo](https://github.com/apecloud/kubeblocks-addons).

1. (Optional) Add the KubeBlocks repo. If you install KubeBlocks with Helm, just run `helm repo update`.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. View the Addon versions.

   ```bash
   helm search repo kubeblocks/etcd --devel --versions
   ```

3. Install the Addon (take etcd as example). Specify a version with `--version`.

   ```bash
   helm install etcd kubeblocks/etcd --namespace kb-system --create-namespace --version x.y.z
   ```

4. Verify whether this Addon is installed.

   The STATUS is `deployed` and this Addon is installed successfully.

   ```bash
   helm list -A
   >
   NAME                 NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART              APP VERSION
   ......
   etcd               	kb-system	1       	2024-10-25 07:18:35.294326176 +0000 UTC	deployed	etcd-0.9.0         v3.5.6
   ```

5. (Optional) You can run the command below to disable the Addon.

   ```bash
   helm uninstall etcd --namespace kb-system
   ```
