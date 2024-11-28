---
title: Install Addons
description: Install KubeBlocks addons with Helm
keywords: [addon, helm, KubeBlocks]
sidebar_position: 4
sidebar_label: Install Addons
---

# Install Addons

With the release of KubeBlocks v0.8.0, Addons are decoupled from KubeBlocks and some Addons are not installed by default. If you want to use these Addons, install Addons first by index. Or if you uninstalled some Addons, you can follow the steps in this tutorial to install them again.

This tutorial takes elasticsearch as an example. You can replace elasticsearch with the Addon you need.

The official index repo is [KubeBlocks index](https://github.com/apecloud/block-index). Addons are maintained in the [KubeBlocks Addon repo](https://github.com/apecloud/kubeblocks-addons).

:::note

Make sure the major version of Addons and KubeBlocks are the same.

For example, you can install an Addon v0.9.0 with KubeBlocks v0.9.2, but using mismatched major versions, such as an Addon v0.8.0 with KubeBlocks v0.9.2, may lead to errors.

:::

<Tabs>

<TabItem value="Helm" label="Install with Helm" default>

1. (Optional) Add the KubeBlocks repo. If you install KubeBlocks with Helm, just run `helm repo update`.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. View the Addon versions.

   ```bash
   helm search repo kubeblocks/elasticsearch --devel --versions
   ```

3. Install the Addon (take elasticsearch as example). Specify a version with `--version`.

   ```bash
   helm install kb-addon-es kubeblocks/elasticsearch --namespace kb-system --create-namespace --version x.y.z
   ```

4. Verify whether this Addon is installed.

   The STATUS is `deployed` and this Addon is installed successfully.

   ```bash
   helm list -A
   >
   NAME             NAMESPACE	REVISION	UPDATED                                STATUS  	 CHART                   APP VERSION
   ......
   kb-addon-es      kb-system	1       	2024-11-27 10:04:59.730127 +0800 CST   deployed	 elasticsearch-0.9.0     8.8.2 
   ```

5. (Optional) You can run the command below to uninstall the Addon.

   If you have created a related cluster, delete the cluster first.

   ```bash
   helm uninstall kb-addon-es --namespace kb-system
   ```

</TabItem>

<TabItem value="kbcli" label="Install with kbcli">

1. View the index.

   kbcli creates an index named `kubeblocks` by default and you can check whether this index is created by running `kbcli addon index list`.

   ```bash
   kbcli addon index list
   >
   INDEX        URL
   kubeblocks   https://github.com/apecloud/block-index.git 
   ```

   If the list is empty or you want to add your index, you can add the index manually by using `kbcli addon index add <index-name> <source>`. For example,

   ```bash
   kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git
   ```

   If you are not sure whether the kubeblocks index is the latest version, you can update it.

   ```bash
   kbcli addon index update kubeblocks
   ```

2. (Optional) Search whether the Addon exists in the index.

   ```bash
   kbcli addon search elasticsearch
   >
   ADDON           VERSION         INDEX
   elasticsearch   0.7.0           kubeblocks
   elasticsearch   0.7.1           kubeblocks
   elasticsearch   0.7.2           kubeblocks
   elasticsearch   0.8.0           kubeblocks
   elasticsearch   0.9.0           kubeblocks
   ```

3. Install the Addon.

   If there are multiple index sources and versions for an Addon, you can specify an index by `--index` and a version by `--version`. The system installs the latest version in the `kubeblocks` index by default.

   ```bash
   kbcli addon install elasticsearch --index kubeblocks --version x.y.z
   ```

4. Verify whether this Addon is installed.

   The STATUS is `Enabled` and this Addon is installed successfully.

   ```bash
   kbcli addon list
   >
   NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
   elasticsearch                  0.9.0          apecloud    Enabled    true
   ```

5. (Optional) You can run the command below to disable the Addon.

   If you have created a related cluster, delete the cluster first.

   ```bash
   kbcli addon disable elasticsearch
   ```

   Or you can run the command below to uninstall the Addon.

   ```bash
   kbcli addon uninstall elasticsearch
   ```

:::note

kbcli supports enable/disable an Addon. You can enable/disable Addons according to your need. Moreover, when installing KubeBlocks by kbcli, some Addons are installed but remain disabled by default, with their status shown as `Disabled`. You can enable them by kbcli. For example,

```bash
kbcli addon list
>
NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
alertmanager-webhook-adaptor   0.1.4          apecloud    Disabled   false
apecloud-otel-collector        0.1.2-beta.3   apecloud    Disabled   false
aws-load-balancer-controller   1.4.8          community   Disabled   false
csi-driver-nfs                 4.5.0          community   Disabled   false
csi-hostpath-driver            0.7.0          community   Disabled   false
grafana                        6.43.5         community   Disabled   false
llm                            0.9.0          community   Disabled   false
prometheus                     15.16.1        community   Disabled   false
qdrant                         0.9.1          community   Disabled   false
victoria-metrics-agent         0.8.41         community   Disabled   false
```

* Enable an Addon.

   ```bash
   kbcli addon enable qdrant
   ```

* Disable an Addon.

   ```bash
   kbcli addon disable qdrant
   ```

After enabling/disabling an Addon, check the Addon list to verify whether the Addon's status changes as required.

```bash
kbcli addon list
>
NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
qdrant                         0.9.1          community   Enabled    false
```

:::

</TabItem>

</Tabs>
