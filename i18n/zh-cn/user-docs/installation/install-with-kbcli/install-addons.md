---
title: Install Addons
description: Install KubeBlocks Addons
keywords: [kbcli, kubeblocks, addons, install]
sidebar_position: 3
sidebar_label: Install Addons
---

# Install Addons

## Use the index to install an addon

With the release of KubeBlocks v0.8.0, addons are decoupled from KubeBlocks and some addons are not installed by default. If you want to use these addons, add addons first by index.

The official index repo is [KubeBlocks index](https://github.com/apecloud/block-index). The code of all addons is maintained in the [KubeBlocks addon repo](https://github.com/apecloud/kubeblocks-addons).

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
   kbcli addon enable snapshot-controller
   ```

   To disable an addon, use `kbcli addon disable`.

2. List the addons again to check whether it is enabled.

   ```bash
   kbcli addon list
   ```

## 使用引擎

### 使用索引安装引擎

KubeBlocks v0.8.0 发布后，引擎（addon）与 KubeBlocks 解耦，KubeBlocks 仅默认安装了部分引擎，如需体验其它引擎，需通过索引安装相关引擎。

官网引擎索引仓库为 [KubeBlocks index](https://github.com/apecloud/block-index)。引擎代码维护在 [KubeBlocks addon repo](https://github.com/apecloud/kubeblocks-addons)。

1. 查看引擎仓库索引。

   kbcli 默认创建名为 `kubeblocks` 的索引，可使用 `kbcli addon index list` 查看该索引。

   ```bash
   kbcli addon index list
   >
   INDEX        URL
   kubeblocks   https://github.com/apecloud/block-index.git 
   ```

   如果命令执行结果未显示或者你想要添加自定义索引仓库，则表明索引未建立，可使用 `kbcli addon index add <index-name> <source>` 命令手动添加索引。例如，

   ```bash
   kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git
   ```

2. （可选）索引建立后，可以通过 `addon search` 命令检查想要安装的引擎是否在索引信息中存在

   ```bash
   kbcli addon search mariadb
   >
   ADDON     VERSION   INDEX
   mariadb   0.7.0     kubeblocks
   ```

3. 安装引擎。

   当引擎有多个版本和索引源时，可使用 `--index` 指定索引源，`--version` 指定安装版本。系统默认以 `kubeblocks` 索引仓库 为索引源，安装最新版本。

   ```bash
   kbcli addon install mariadb --index kubeblocks --version 0.7.0
   ```

   **后续操作**

   引擎安装完成后，可查看引擎列表、启用引擎。

### 查看引擎列表

执行 `kbcli addon list` 命令查看已经支持的引擎。

### 启用/禁用引擎

请按照以下步骤手动启用或禁用引擎。

***步骤：***

1. 执行 `kbcli addon enable` 启用引擎。

   ***示例***

   ```bash
   kbcli addon enable snapshot-controller
   ```

   执行 `kbcli addon disable` 禁用引擎。

2. 再次查看引擎列表，检查是否已启用引擎。

   ```bash
   kbcli addon list
   ```