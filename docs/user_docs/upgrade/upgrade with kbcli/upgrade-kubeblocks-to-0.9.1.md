---
title: Upgrade to KubeBlocks v0.9.1
description: Upgrade to KubeBlocks v0.9.1, operation, tips and notes
keywords: [upgrade, 0.9.1]
sidebar_position: 1
sidebar_label: Upgrade to KubeBlocks v0.9.1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Upgrade to KubeBlocks v0.9.1

:::note

Execute `kbcli version` to check the current KubeBlocks version you are running before upgrading KubeBlocks.

:::

## Compatibility

KubeBlocks v0.9.1 is compatible with KubeBlocks v0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using Addons from KubeBlocks v0.7 or earlier (v0.7., v0.6., etc), DO [upgrade KubeBlocks and all addons to v0.8 first](./upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

If you are upgrading from v0.8 to v0.9, it's recommended to enable webhook to ensure the availability.

## Upgrade from KubeBlocks 0.9.0

1. Download kbcli v0.9.1.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.1
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.1 --set crd.enabled=false
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.1 --set crd.enabled=false --set upgradeAddons=true
    ```

    :::

   `kbcli` will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing addons are not deleted during the upgrade.

## Upgrade from KubeBlocks v0.8

1. Download kbcli v0.9.1.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.1
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.1 --set admissionWebhooks.enabled=true --set admissionWebhooks.ignoreReplicasCheck=true  --set crd.enabled=false  
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.1 --set upgradeAddons=true --set admissionWebhooks.enabled=true --set admissionWebhooks.ignoreReplicasCheck=true  --set crd.enabled=false 
    ```

    :::

    `kbcli` will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing Addons are not deleted during the upgrade.

## Upgrade addons

If you didn't specify `upgradeAddons` as `true` or your Addon is not included in the default installed Addons, you can upgrade Addons by the options provided below to use the v0.9.x API.

:::note

- If the Addon you want to upgrade is `mysql`, you need to upgrade this Addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.

- If the Addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this Addon. Otherwise, these Addons cannot be used in KubeBlocks v0.9 normally.

:::

```bash
# View the Addon index list
kbcli addon index list

# Update one index and the default index is kubeblocks
kbcli addon index update kubeblocks

# Search available Addon versions
kbcli addon search {addon-name}

# Install an Addon
kbcli addon install {addon-name} --version x.y.z

# Upgrade this Addon to a certain version
kbcli addon upgrade {addon-name} --version x.y.z

# Force to upgrade to a certain version
kbcli addon upgrade {addon-name} --version x.y.z --force

# View the available Addon versions
kbcli addon list | grep {addon-name}
```
