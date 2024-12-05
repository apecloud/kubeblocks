---
title: Upgrade to v0.9.2
description: Upgrade to KubeBlocks v0.9.2, operation, tips and notes
keywords: [upgrade, 0.9.2]
sidebar_position: 1
sidebar_label: Upgrade to v0.9.2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Upgrade to KubeBlocks v0.9.2

:::note

- Before upgrading, check your current KubeBlocks version:

   Run `helm -n kb-system list | grep kubeblocks` or `kbcli version`.
- For upgrading to different versions:

   - For v0.9.1, follow this upgrade tutorial, replacing the version number with v0.9.1.
   - [v0.9.0 upgrade guide](https://github.com/apecloud/kubeblocks/blob/main/docs/user_docs/upgrade-kubeblocks/upgrade-to-0.9.0.md).
   - [v0.8.x upgrade guide](https://github.com/apecloud/kubeblocks/blob/main/docs/user_docs/upgrade-kubeblocks/upgrade-to-0.8.md).

   Installing the latest version is recommended for better performance and features.

:::

## Compatibility

KubeBlocks v0.9.2 is compatible with KubeBlocks v0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using Addons from KubeBlocks v0.7 or earlier (v0.6, etc), DO [upgrade KubeBlocks and all Addons to v0.8 first](./upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

If you are upgrading from v0.8 to v0.9, it's recommended to enable webhook to ensure the availability.

## Upgrade from KubeBlocks v0.9.0

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. View Addon and check whether the `"helm.sh/resource-policy": "keep"` annotation exists.

    KubeBlocks streamlines the default installed engines. Add the `"helm.sh/resource-policy": "keep"` annotation to avoid deleting Addon resources that are already in use during the upgrade.

    Check whether the `"helm.sh/resource-policy": "keep"` annotation is added.

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
    ```

    If the annotation doesn't exist, run the command below to add it. You can replace `-l app.kubernetes.io/name=kubeblocks` with your actual filter name.

    ```bash
    kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
    ```

2. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removes CRD from the Helm chart. Before upgrading, you need to install CRD.

    ```bash
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks_crds.yaml
    ```

3. Upgrade KubeBlocks.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.2 --set crd.enabled=false
    ```

    Upgrading from v0.9.0 to v0.9.2 doesn't include API change, so you can set `--set crd.enabled=false` to skip the API upgrade task.

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.2, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.2, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.2 \
      --set upgradeAddons=true \
      --set crd.enabled=false
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Download kbcli v0.9.2.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.2
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kb upgrade --version 0.9.2
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.2, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.2, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.2 --set upgradeAddons=true
    ```

    :::

   `kbcli` will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing Addons are not deleted during the upgrade.

</TabItem>

</Tabs>

## Upgrade from KubeBlocks v0.8.x

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. View Addon and check whether the `"helm.sh/resource-policy": "keep"` annotation exists.

    KubeBlocks streamlines the default installed engines. Add the `"helm.sh/resource-policy": "keep"` annotation to avoid deleting Addon resources that are already in use during the upgrade.

    Check whether the `"helm.sh/resource-policy": "keep"` annotation is added.

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
    ```

    If the annotation doesn't exists, run the command below to add it. You can replace `-l app.kubernetes.io/name=kubeblocks` with your actual filter name.

    ```bash
    kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
    ```

2. Delete the incompatible OpsDefinition.

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removed CRD from the Helm chart and changed the group of StorageProvider. Before upgrading, you need to install StorageProvider CRD first.

    If the network is slow, it's recommended to download the CRD YAML file on your localhost before further operations.

    ```bash
    kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/dataprotection.kubeblocks.io_storageproviders.yaml
    ```

4. Upgrade KubeBlocks.

    Here are some options that need your attention before the upgrade.

    - Setting `admissionWebhooks.enabled=true` enables the webhook, supporting the multi-version conversion of the ConfigConstraint API.
    - Setting `admissionWebhooks.ignoreReplicasCheck=true` enables the webhook by default only when KubeBlocks is deployed with 3 replicas. If only a single replica is deployed, you can configure this variable to bypass the check.
    - If the KubeBlocks you are running uses the image registry starting with `infracreate-registry`, it is recommended to explicitly configure the image registry during the upgrade. Refer to [FAQ](./faq.md#specify-an-image-registry-during-upgrading-kubeblocks) for details.

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts

    helm repo update kubeblocks

    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.2 \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.2, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.2, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.2 \
      --set upgradeAddons=true \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true 
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Download kbcli v0.9.2.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.2
    ```

2. Upgrade KubeBlocks.

    Check the kbcli version and make sure you're using kbcli v0.9.2.

    ```bash
    kbcli version
    ```

    Here are some options that need your attention before the upgrade.

    - Setting `admissionWebhooks.enabled=true` enables the webhook, supporting the multi-version conversion of the ConfigConstraint API.
    - Setting `admissionWebhooks.ignoreReplicasCheck=true` enables the webhook by default only when KubeBlocks is deployed with 3 replicas. If only a single replica is deployed, you can configure this variable to bypass the check.
    - If the KubeBlocks you are running uses the image registry starting with `infracreate-registry`, it is recommended to explicitly configure the image registry during the upgrade. Refer to [FAQ](./faq.md#specify-an-image-registry-during-upgrading-kubeblocks) for details.

    ```bash
    kbcli kb upgrade --version 0.9.2 \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9.2, the versions of already-installed Addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9.2, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    kbcli kb upgrade --version 0.9.2 \
      --set upgradeAddons=true \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::

    `kbcli` will automatically add the annotation `"helm.sh/resource-policy": "keep"` to ensure that existing Addons are not deleted during the upgrade.

</TabItem>

</Tabs>

## Upgrade Addons

If you didn't specify `upgradeAddons` as `true` or your Addon is not included in the default installed Addons, you can upgrade Addons by running the commands provided below to use the v0.9.x API.

:::note

- If the Addon you want to upgrade is `mysql`, you need to upgrade this Addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8.x cannot be used in v0.9.x.

- If the Addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this Addon. Otherwise, these Addons cannot be used in KubeBlocks v0.9.x normally.

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# If github is not accessible or the network is very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# Update helm repo
helm repo update

# Search the available Addon versions
helm search repo kubeblocks-addons/{addon-name} --versions --devel 

# Update addon version
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
kbcli addon search {addon-name}

# Install an Addon
kbcli addon install {addon-name} --version x.y.z

# Upgrade this Addon to a specified version
kbcli addon upgrade {addon-name} --version x.y.z

# Force to upgrade to a specified version
kbcli addon upgrade {addon-name} --version x.y.z --force

# View the available Addon versions
kbcli addon list | grep {addon-name}
```

</TabItem>

</Tabs>

## FAQ

Refer to the [FAQ](./../faq.md) to address common questions and issues that may arise when upgrading KubeBlocks. If your question isn't covered, you can [submit an issue](https://github.com/apecloud/kubeblocks/issues/new/choose) or [start a discussion](https://github.com/apecloud/kubeblocks/discussions) on upgrading on GitHub.
