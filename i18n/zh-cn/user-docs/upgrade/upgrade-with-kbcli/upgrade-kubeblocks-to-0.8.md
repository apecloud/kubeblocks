---
title: 升级到 KubeBlocks v0.8
description: 如何升级到 KubeBlocks v0.8
keywords: [升级, 0.8]
sidebar_position: 3
sidebar_label: 升级到 KubeBlocks v0.8
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 升级到 KubeBlocks v0.8

本文档介绍如何从低版本 KubeBlocks 升级至 KubeBlocks 0.8 版本。

:::note

在升级前，请先执行 `kbcli version` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。

:::

## 从 v0.6 版本升级

如果当前使用的是 KubeBlocks v0.6 版本，请先将 KubeBlocks 升级至 v0.7.2 版本，操作如下：

1. 下载 kbcli v0.7.2 版本。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.7.2
    ```

2. 升级至 KubeBlocks v0.7.2。

    ```shell
    kbcli kb upgrade --version 0.7.2
    ```

## 从 v0.7 版本升级

1. 下载 kbcli v0.8 版本。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.8.1
    ```

2. 升级 KubeBlocks。

    ```shell

    kbcli kb upgrade --version 0.8.1 --set dataProtection.image.datasafed.tag=0.1.0

    ```

    kbcli 会默认为已有 addon 添加 annotation `"helm.sh/resource-policy": "keep"`，确保升级过程中已有的 addon 不会被删除。

## FAQ

可查看 [FAQ](./../faq.md)，了解并解决升级 KubeBlocks 过程中的常见问题。如果您还遇到了其他问题，可以[提交 issue](https://github.com/apecloud/kubeblocks/issues/new/choose) 或者在 [GitHub 讨论区](https://github.com/apecloud/kubeblocks/discussions)提交您的问题。
