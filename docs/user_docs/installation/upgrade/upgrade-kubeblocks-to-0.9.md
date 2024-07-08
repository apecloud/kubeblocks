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

## Compatability

KubeBlocks 0.9 is compatible with KubeBlocks 0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using addons from KubeBlocks 0.7 or earlier (v0.7., 0.6., etc), DO [upgrade KubeBlocks and all addons to v0.8 first](./upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

## Upgrade from KubeBlocks v0.8

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. Set keepAddons.

    KubeBlocks v0.8 streamlines the default installed engines. To avoid deleting addon resources that are already in use during the upgrade, execute the following commands first.

    - Check the current KubeBlocks version.

         ```shell
         helm -n kb-system list | grep kubeblocks
         ```

    - Set the value of keepAddons as true.

         ```shell
         helm repo add kubeblocks https://apecloud.github.io/helm-charts
         helm repo update kubeblocks
         helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version {VERSION} --set keepAddons=true
         ```

         Replace `{VERSION}` with your current KubeBlocks version, such as 0.8.3.

    - Check addons.

         Execute the following command to ensure that the addon annotations contain `"helm.sh/resource-policy": "keep"`.

         ```shell
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. Delete the incompatible OpsDefinition.

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removes CRD from the Helm chart. Before upgrading, you need to install CRD.

    ```shell
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/kubeblocks_crds.yaml
    ```

4. Upgrade KubeBlocks.

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=false
    ```

    :::note

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed addons will not be upgraded by default. If you want to upgrade the addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=true
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

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

</TabItem>

</Tabs>

## Upgrade addons

If you didn't specify `upgradeAddons` as `true` or your addon is not included in the default installed addons, you can upgrade addons by the options provided below to use the v0.9.0 API.

:::note

If the addon you want to upgrade is `mysql`, you need to upgrade this addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.

If the addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this addon. Otherwise, these addons cannot be used in KubeBlocks v0.9 normally.

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# Update helm repo
helm repo update

# Update addon version
helm upgrade -i xxx kubeblocks-addons/xxx --version x.x.x -n kb-system  
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli addon index list

kbcli addon index update kubeblocks

kbcli addon upgrade xxx --force
```

</TabItem>

</Tabs>
