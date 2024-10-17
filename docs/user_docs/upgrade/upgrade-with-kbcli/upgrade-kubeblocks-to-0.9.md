---
title: Upgrade to KubeBlocks v0.9
description: Upgrade to KubeBlocks v0.9, operation, tips and notes
keywords: [upgrade, 0.9]
sidebar_position: 2
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

KubeBlocks 0.9 is compatible with KubeBlocks 0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using Addons from KubeBlocks 0.7 or earlier (0.6, etc), DO [upgrade KubeBlocks and all Addons to v0.8 first](upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

## Upgrade from KubeBlocks v0.8

1. Download kbcli v0.9.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.0
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.0 
    ```

    :::note

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.0 --set upgradeAddons=true
    ```

    :::

    kbcli will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing Addons are not deleted during the upgrade.

## Upgrade Addons

If you didn't specify `upgradeAddons` as `true` or your Addon is not included in the default installed Addons, you can upgrade Addons by the options provided below to use the v0.9.0 API.

:::note

If the Addon you want to upgrade is `mysql`, you need to upgrade this Addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.

If the Addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this Addon. Otherwise, these Addons cannot be used in KubeBlocks v0.9 normally.

:::

```bash
# View the Addon index list
kbcli addon index list

# Update one index and the default index is kubeblocks
kbcli addon index update kubeblocks

#Search available Addon versions
kbcli addon search <addonName>

# Install an Addon
kbcli addon install <addonName> --version x.y.z

# Upgrade this Addon to a specified version
kbcli addon upgrade <addonName> --version x.y.z

# Force to upgrade to a specified version
kbcli addon upgrade <addonName> --version x.y.z --force

# View the available Addon versions
kbcli addon list | grep <addonName>
```
