---
title: Upgrade to KubeBlocks v0.8
description: Upgrade to KubeBlocks v0.8, operation, tips and notes
keywords: [upgrade, 0.8]
sidebar_position: 3
sidebar_label: Upgrade to KubeBlocks v0.8
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';


# Upgrade to KubeBlocks v0.8

In this tutorial, you will learn how to upgrade to KubeBlocks v0.8.

:::note

Execute `kbcli version` to check the current KubeBlocks version you are running, and then upgrade.

:::

## Upgrade from KubeBlocks v0.6

If you are currently running KubeBlocks v0.6, please upgrade to v0.7.2 first.

1. Download kbcli v0.7.2.

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.7.2
    ```

2. Upgrade to KubeBlocks v0.7.2.

    ```shell
    kbcli kb upgrade --version 0.7.2
    ```

## Upgrade from KubeBlocks v0.7

1. Download kbcli v0.8.

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.8.1
    ```

2. Upgrade KubeBlocks.

    ```shell

    kbcli kb upgrade --version 0.8.1 --set dataProtection.image.datasafed.tag=0.1.0

    ```

    kbcli will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing addons are not deleted during the upgrade.

## FAQ

Refer to the [FAQ](./../faq.md) to address common questions and issues that may arise when upgrading KubeBlocks. If your question isn't covered, you can [submit an issue](https://github.com/apecloud/kubeblocks/issues/new/choose) or [start a discussion](https://github.com/apecloud/kubeblocks/discussions) on upgrading in GitHub.
