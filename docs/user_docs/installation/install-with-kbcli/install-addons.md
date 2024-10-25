---
title: Install Addons
description: Install KubeBlocks Addons
keywords: [kbcli, kubeblocks, addons, install]
sidebar_position: 3
sidebar_label: Install Addons
---

# Install Addons

## Use the index to install an addon

With the release of KubeBlocks v0.8.0, Addons are decoupled from KubeBlocks and some Addons are not installed by default. If you want to use these Addons, install Addons first by index. Or if you uninstalled some Addons, you can follow the steps in this tutorial to install them again.

This tutorial takes etcd as an example. You can replace etcd with the Addon you need.

The official index repo is [KubeBlocks index](https://github.com/apecloud/block-index). The code of all Addons is maintained in the [KubeBlocks Addon repo](https://github.com/apecloud/kubeblocks-addons).

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

2. (Optional) Search whether the addon exists in the index.

   ```bash
   kbcli addon search mariadb
   >
   ADDON     VERSION   INDEX
   mariadb   0.7.0     kubeblocks
   ```

3. Install the addon.

   If there are multiple index sources and versions for an addon, you can specify them by adding flags. The system installs the latest version in the `kubeblocks` index by default.

   ```bash
   kbcli addon install mariadb --index kubeblocks --version 0.7.0
   ```

   **What's next**

   After the addon is installed, you can list and enable it.

## List addons

To list supported addons, run `kbcli addon list` command.

## Enable/Disable addons

To manually enable or disable addons, follow the steps below.

***Steps:***

1. To enable an addon, use `kbcli addon enable`.

   ***Example***

   ```bash
   kbcli addon enable qdrant
   ```

   To disable an addon, use `kbcli addon disable`.

2. List the addons again to check whether it is enabled.

   ```bash
   kbcli addon list
   ```
