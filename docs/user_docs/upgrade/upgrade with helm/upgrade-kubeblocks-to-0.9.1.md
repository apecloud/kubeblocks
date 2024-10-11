---
title: Upgrade to KubeBlocks v0.9.1
description: Upgrade to KubeBlocks v0.9.1, operation, tips and notes
keywords: [upgrade, 0.9.1]
sidebar_position: 1
sidebar_label: Upgrade to KubeBlocks v0.9.1
---

# Upgrade to KubeBlocks v0.9.1

:::note

Execute `helm -n kb-system list | grep kubeblocks` to check the current KubeBlocks version you are running before upgrading KubeBlocks.

:::

## Compatibility

KubeBlocks v0.9.1 is compatible with KubeBlocks v0.8 APIs, but compatibility with APIs from versions prior to v0.8 is not guaranteed. If you are using Addons from KubeBlocks v0.7 or earlier (v0.6, etc), DO [upgrade KubeBlocks and all Addons to v0.8 first](./upgrade-kubeblocks-to-0.8.md) to ensure service availability before upgrading to v0.9.

If you are upgrading from v0.8 to v0.9, it's recommended to enable webhook to ensure the availability.

## Upgrade from KubeBlocks v0.9.0

1. Check whether `keepAddons` is set as `true`.

    KubeBlocks v0.8 streamlines the default installed engines. To avoid deleting Addon resources that are already in use during the upgrade, execute the following commands first to check whether `keepAddons` is set as `true`.

    - Check the current KubeBlocks version.

         ```shell
         helm -n kb-system list | grep kubeblocks
         ```

    - Set `crd.enabled` as `false` to avoid automatic CRD installation.

         ```shell
         helm repo add kubeblocks https://apecloud.github.io/helm-charts
         helm repo update kubeblocks
         helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version {VERSION} --set crd.enabled=false
         ```

         Replace `{VERSION}` with your current KubeBlocks version, such as 0.9.0.

    - Check Addons.

         Execute the following command to ensure that the Addon annotations contain `"helm.sh/resource-policy": "keep"`.

         ```shell
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removes CRD from the Helm chart. Before upgrading, you need to install CRD.

    ```shell
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.1/kubeblocks_crds.yaml
    ```

3. Upgrade KubeBlocks.

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 --set crd.enabled=false
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed addons will not be upgraded by default. If you want to upgrade the addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 --set upgradeAddons=true --set crd.enabled=false
    ```

    :::

## Upgrade from KubeBlocks v0.8

1. Set `keepAddons` to preserve the Addon resources during the upgrade.

    KubeBlocks v0.8 streamlines the default installed engines. To avoid deleting Addon resources that are already in use during the upgrade, execute the following commands first.

    - Check the current KubeBlocks version.

         ```shell
         helm -n kb-system list | grep kubeblocks
         ```

    - Set the value of `keepAddons` as true.

         ```shell
         helm repo add kubeblocks https://apecloud.github.io/helm-charts
         helm repo update kubeblocks
         helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version {VERSION} --set keepAddons=true
         ```

         Replace `{VERSION}` with your current KubeBlocks version, such as 0.8.0.

    - Check Addons.

         Execute the following command to ensure that the Addon annotations contain `"helm.sh/resource-policy": "keep"`.

         ```shell
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. Delete the incompatible OpsDefinition.

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. Install CRD.

    To reduce the size of Helm chart, KubeBlocks v0.8 removes CRD from the Helm chart. Before upgrading, you need to install CRD.

    If the network is slow, it's recommended to download the CRD YAML file on your localhost before further operations.

    ```shell
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.1/kubeblocks_crds.yaml || kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.1/kubeblocks_crds.yaml 
    ```

4. Upgrade KubeBlocks.

    The command below sets `--set admissionWebhooks.enabled=true --set admissionWebhooks.ignoreReplicasCheck=true` to enable the webhook, facilitating support for multiple versions related to ConfigConstraint.

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 --set upgradeAddons=false --set admissionWebhooks.enabled=true --set admissionWebhooks.ignoreReplicasCheck=true --set crd.enabled=false 
    ```

    :::warning

    To avoid affecting existing database clusters, when upgrading to KubeBlocks v0.9, the versions of already-installed addons will not be upgraded by default. If you want to upgrade the Addons to the versions built into KubeBlocks v0.9, execute the following command. Note that this may restart existing clusters and affect availability. Please proceed with caution.

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 --set upgradeAddons=true --set admissionWebhooks.enabled=true --set admissionWebhooks.ignoreReplicasCheck=true  --set crd.enabled=false 
    ```

    :::

## Upgrade Addons

If you didn't specify `upgradeAddons` as `true` or your Addon is not included in the default installed addons, you can upgrade Addons by running the commands provided below to use the v0.9.x API.

:::note

- If the Addon you want to upgrade is `mysql`, you need to upgrade this Addon and restart the cluster. Otherwise, the cluster created in KubeBlocks v0.8 cannot be used in v0.9.

- If the Addon you want to use is `clickhouse/milvus/elasticsearch/llm`, you need to upgrade KubeBlocks first and then upgrade this Addon. Otherwise, these Addons cannot be used in KubeBlocks v0.9 normally.

:::

```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# If github is not accessible or the network is very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# Update helm repo
helm repo update

# Update addon version
helm upgrade -i {addon-release-name} kubeblocks-addons/{addon-name} --version x.y.z -n kb-system   
```
