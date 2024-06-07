---
title: Upgrade to KubeBlocks v0.8
description: Upgrade to KubeBlocks v0.8, operation, tips and notes
keywords: [upgrade, 0.8]
sidebar_position: 1
sidebar_label: Upgrade to KubeBlocks v0.8
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Upgrade to KubeBlocks v0.8

In this tutorial, you will learn how to upgrade to KubeBlocks v0.8.

:::note

Execute `kbcli version` to check the current KubeBlocks version you are running, and then upgrade it.

:::

## Upgrade from KubeBlocks v0.6

If you are currently running KubeBlocks v0.6, please upgrade to v0.7.2 first.

1. Download kbcli v0.7.2.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.7.2
    ```

2. Upgrade to KubeBlocks v0.7.2.

    ```bash
    kbcli kb upgrade --version 0.7.2
    ```

## Upgrade from KubeBlocks v0.7

1. Set keepAddons.

    KubeBlocks v0.8 streamlines the default installed engines and separates the addons from KubeBlocks operators to KubeBlocks-Addons repo, such as greptime, influxdb, neon, oracle-mysql, orioledb, tdengine, mariadb, nebula, risingwave, starrocks, tidb, and zookeeper. To avoid deleting addon resources that are already in use during the upgrade, execute the following commands:

- Check the current KubeBlocks version.

    ```bash
    helm -n kb-system list | grep kubeblocks
    ```

- Set the value of keepAddons as true.

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts
    helm repo update kubeblocks
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version {VERSION} --set keepAddons=true
    ```

    Replace {VERSION} with your current KubeBlocks version, such as 0.7.2.

- Check addons.

    Execute the following command to ensure that the addon annotations contain `"helm.sh/resource-policy": "keep"`.

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
    ```

2. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removes CRD from the Helm chart. Before upgrading, you need to install CRD.

    ```bash
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.8.1/kubeblocks_crds.yaml
    ```

3. Upgrade KubeBlocks.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.8.1 --set dataProtection.image.datasafed.tag=0.1.0
    ```

:::note

To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.8, the versions of already-installed addons will not be upgraded by default. If you want to upgrade the addons to the versions built into KubeBlocks v0.8, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

```bash
helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.8.1 --set upgradeAddons=true
```

:::
