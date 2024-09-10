---
title: Upgrade to KubeBlocks v0.9
description: Upgrade to KubeBlocks v0.9, operation, tips and notes
keywords: [upgrade, 0.9]
sidebar_position: 1
sidebar_label: Upgrade to KubeBlocks v0.9
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Upgrade to KubeBlocks v0.9

In this tutorial, you will learn how to upgrade to KubeBlocks v0.9.

:::note

Execute `kbcli version` to check the current KubeBlocks version you are running, and then upgrade.

:::

## Compatibility

KubeBlocks 0.9 is compatible with KubeBlocks 0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using addons from KubeBlocks 0.7 or earlier (v0.7., 0.6., etc), DO [upgrade KubeBlocks and all addons to v0.8 first](upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

## Upgrade from KubeBlocks v0.8

1. Download kbcli v0.9.

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.0
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.0 
    ```

    :::note

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed addons will not be upgraded by default. If you want to upgrade the addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.0 --set upgradeAddons=true
    ```

    :::

    kbcli will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing addons are not deleted during the upgrade.


## Upgrade addons

If you didn't specify `upgradeAddons` as `true` or your addon is not included in the default installed addons, you can upgrade addons by the options provided below to use the v0.9.0 API.

:::note

If the addon you want to upgrade is `mysql`, you need to upgrade this addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.

If the addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this addon. Otherwise, these addons cannot be used in KubeBlocks v0.9 normally.

:::


```bash
kbcli addon index list

kbcli addon index update kubeblocks

kbcli addon upgrade xxx --force
```
