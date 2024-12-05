---
title: Upgrade to v0.9.0
description: Upgrade to KubeBlocks v0.9.0, operation, tips and notes
keywords: [upgrade, 0.9.0]
draft: true
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Upgrade to KubeBlocks v0.9.0

In this tutorial, you will learn how to upgrade to KubeBlocks v0.9.0.

:::note

Execute `helm -n kb-system list | grep kubeblocks` or `kbcli version` to check the current KubeBlocks version you are running, and then upgrade KubeBlocks.

:::

## Compatibility

KubeBlocks 0.9.0 is compatible with KubeBlocks 0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using Addons from KubeBlocks 0.7 or earlier (0.6, etc), DO [upgrade KubeBlocks and all Addons to v0.8 first](https://github.com/apecloud/kubeblocks/blob/main/docs/user_docs/upgrade-kubeblocks/upgrade-to-0.8.md) to ensure service availability before upgrading to v0.9.0.

## Upgrade from KubeBlocks v0.8

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. Add the `"helm.sh/resource-policy": "keep"` for Addons.

    KubeBlocks v0.8 streamlines the default installed engines. To avoid deleting Addon resources that are already in use during the upgrade, execute the following commands first.

    - Add the `"helm.sh/resource-policy": "keep"` for Addons. You can replace `-l app.kubernetes.io/name=kubeblocks` with your actual filter name.

         ```bash
         kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
         ```

    - Check Addons.

         Execute the following command to ensure that the Addon annotations contain `"helm.sh/resource-policy": "keep"`.

         ```bash
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. Delete the incompatible OpsDefinition.

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. Install the StorageProvider CRD before the upgrade.

    If the network is slow, it's recommended to download the CRD YAML file on your localhost before further operations.

    ```bash
    kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/dataprotection.kubeblocks.io_storageproviders.yaml
    ```

4. Upgrade KubeBlocks.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0
    ```

    :::note

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.0, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.0, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 \
    --set upgradeAddons=true
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Download kbcli v0.9.0.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.0
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.0 
    ```

    :::note

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.0, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.0, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.0 --set upgradeAddons=true
    ```

    :::

    kbcli will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing Addons are not deleted during the upgrade.

</TabItem>

</Tabs>

## Upgrade Addons

If you didn't specify `upgradeAddons` as `true` or your Addon is not included in the default installed Addons, you can upgrade Addons by running the commands provided below to use the v0.9.0 API.

:::note

If the Addon you want to upgrade is `mysql`, you need to upgrade this Addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.0.

If the Addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this Addon. Otherwise, these Addons cannot be used in KubeBlocks v0.9.0 normally.

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

# Update Addon version
helm upgrade -i {addon-release-name} kubeblocks-addons/{addon-name} --version x.y.z -n kb-system  
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
# View the Addon index list
kbcli addon index list

# Update one index and the default index is kubeblocks
kbcli addon index update kubeblocks

# Search available Addon versions
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

</TabItem>

</Tabs>

## FAQ

Refer to the [FAQ](./faq.md) to address common questions and issues that may arise when upgrading KubeBlocks. If your question isn't covered, you can [submit an issue](https://github.com/apecloud/kubeblocks/issues/new/choose) or [start a discussion](https://github.com/apecloud/kubeblocks/discussions) on upgrading on GitHub.
