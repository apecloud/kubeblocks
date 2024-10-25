---
title: 安装引擎
description: 安装 KubeBlocks 引擎
keywords: [kbcli, kubeblocks, addons, 安装，引擎]
sidebar_position: 3
sidebar_label: 安装引擎
---

# 安装引擎

KubeBlocks v0.8.0 发布后，引擎（Addon）与 KubeBlocks 解耦，KubeBlocks 默认安装了部分引擎，如需体验其它引擎，需通过索引安装相关引擎。如果您卸载了部分引擎，也可通过本文步骤，重新安装。

本文以 etcd 为例，可根据实际情况替换引擎名称。

官网引擎索引仓库为 [KubeBlocks index](https://github.com/apecloud/block-index)。引擎代码维护在 [KubeBlocks addon repo](https://github.com/apecloud/kubeblocks-addons)。

## 使用索引安装引擎

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

   如果不确定索引是否为最新版本，可使用如下命令更新索引。

   ```bash
   kbcli addon index update kubeblocks
   ```

2. （可选）索引建立后，可以通过 `addon search` 命令检查想要安装的引擎是否在索引信息中存在。

   ```bash
   kbcli addon search etcd
   >
   ADDON   VERSION         INDEX
   etcd    0.7.0           kubeblocks
   etcd    0.8.0           kubeblocks
   etcd    0.9.0           kubeblocks
   ```

3. 安装引擎。

   当引擎有多个版本和索引源时，可使用 `--index` 指定索引源，`--version` 指定安装版本。系统默认以 `kubeblocks` 索引仓库 为索引源，安装最新版本。

   ```bash
   kbcli addon install etcd --index kubeblocks --version x.y.z
   ```

   **后续操作**

   引擎安装完成后，可查看引擎列表、启用引擎。

## 查看引擎列表

执行 `kbcli addon list` 命令查看已经支持的引擎。

## 启用/禁用引擎

请按照以下步骤手动启用或禁用引擎。

***步骤：***

1. 执行 `kbcli addon enable` 启用引擎。

   ***示例***

   ```bash
   kbcli addon enable etcd
   ```

   执行 `kbcli addon disable` 禁用引擎。

2. 再次查看引擎列表，检查是否已启用引擎。

   ```bash
   kbcli addon list
   ```

## 卸载引擎

您也可卸载已安装的引擎。如果已经创建了相关集群，请先删除集群。

```bash
kbcli cluster delete <name>
```

卸载已安装的引擎。

```bash
kbcli addon uninstall etcd
```
