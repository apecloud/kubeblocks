---
title: 升级到 KubeBlocks v0.9
description: 如何升级到 KubeBlocks v0.9
keywords: [升级, 0.9]
sidebar_position: 2
sidebar_label: 升级到 KubeBlocks v0.9
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 升级到 KubeBlocks v0.9

本文档介绍如何将 KubeBlocks 升级至 KubeBlocks 0.9 版本。

:::note

在升级前，请先执行 `kbcli version` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。

:::

## 兼容性说明

KubeBlocks v0.9 可以兼容 KubeBlocks v0.8 的 API，但不保证兼容 v0.8 之前版本的 API，如果您正在使用 KubeBlocks v0.7 或者更老版本的引擎（版本号为 `0.7.x`, `0.6.x`），请务必参考 [v0.8 升级文档](./upgrade-kubeblocks-to-0.8.md)将 KubeBlocks 升级至 v0.8 并将所有引擎升级至 v0.8，以确保升级至 v0.9 后服务的可用性。

## 从 0.8 版本升级

1. 下载 kbcli v0.9.0。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.0
    ```

2. 升级 KubeBlocks。

    ```bash
    kbcli kb upgrade --version 0.9.0 
    ```

    :::note

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    kbcli kb upgrade --version 0.9.0 --set upgradeAddons=true
    ```

    :::

    kbcli 会默认为已有引擎添加 `"helm.sh/resource-policy": "keep"` 注解，确保升级过程中已有引擎不会被删除。

## 升级引擎

如果您在上述步骤中，没有将 `upgradeAddons` 指定为 `true`，或者您想要使用的引擎不在默认列表中，但您想要使用 v0.9.0 API，可使用如下方式升级引擎。

:::note

- 如果您想要升级的引擎是 `mysql`，您需要升级引擎并重启集群。否则使用 KubeBlocks v0.8 创建的集群将无法在 v0.9 中使用。

- 如果您想要升级 `clickhouse/milvus/elasticsearch/llm`，您需要先升级 KubeBlocks，再升级引擎，否在将无法在 v0.9 中正常使用。

:::

```bash
# 查看引擎索引列表
kbcli addon index list

# 更新某一个索引， 默认的是 kubeblocks
kbcli addon index update kubeblocks

# 检索可用的引擎版本
kbcli addon search {addon-name}

# 安装引擎
kbcli addon install {addon-name} --version x.y.z

# 更新引擎到指定版本
kbcli addon upgrade {addon-name} --version x.y.z

# 强制更新引擎到指定版本
kbcli addon upgrade {addon-name} --version x.y.z --force

# 查看指定引擎版本
kbcli addon list | grep {addon-name}
```

## FAQ

可查看 [FAQ](./../faq.md)，了解并解决升级 KubeBlocks 过程中的常见问题。如果您还遇到了其他问题，可以[提交 issue](https://github.com/apecloud/kubeblocks/issues/new/choose) 或者在 [GitHub 讨论区](https://github.com/apecloud/kubeblocks/discussions)提交您的问题。
