---
title: 升级到 KubeBlocks v0.9
description: 如何升级到 KubeBlocks v0.9
keywords: [升级, 0.9]
sidebar_position: 1
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

KubeBlocks 0.9 可以兼容 KubeBlocks 0.8 的 API，但不保证兼容 0.8 之前版本的 API，如果您正在使用 KubeBlocks 0.7 或者更老版本的 Addon（版本号为 0.7.*, 0.6.*），请务必参考 [0.8 升级文档](./upgrade-kubeblocks-to-0.8.md)将 KubeBlocks 升级至 0.8 并将所有引擎（addon）升级至 0.8，以确保升级至 0.9 版本后服务的可用性。

## 从 0.8 版本升级

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. 设置 keepAddons。

    KubeBlocks v0.8 精简了默认安装的引擎。为避免升级时将已经在使用的引擎资源删除，请先执行如下命令。

    - 查看当前 KubeBlocks 版本。

         ```shell
         helm -n kb-system list | grep kubeblocks
         ```

    - 设置 keepAddons 参数为 true。

         ```shell
         helm repo add kubeblocks https://apecloud.github.io/helm-charts
         helm repo update kubeblocks
         helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version {VERSION} --set keepAddons=true
         ```

         请将以上 `{VERSION}` 替换为当前 KubeBlocks 的版本，比如 0.8.3。

    - 查看 addon。

         执行如下命令，确保 addon annotations 中添加了 `"helm.sh/resource-policy": "keep"`。

         ```shell
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. 删除不兼容的 OpsDefinition。

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. 安装 CRD。

   为避免 KubeBlocks Helm Chart 过大，0.8 版本将 CRD 从 Helm Chart 中移除了，升级前需要先安装 CRD。

    ```shell
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/kubeblocks_crds.yaml
    ```

4. 升级 KubeBlocks

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=false
    ```

    :::note

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9 时，默认不会升级已经安装的引擎版本，如果要升级 Addon 版本至 KubeBlocks v0.9 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=true
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 下载 0.9 版本 kbcli。

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

    kbcli 会默认为已有 addon 添加 annotation `"helm.sh/resource-policy": "keep"`，确保升级过程中已有 addon 不会被删除。

</TabItem>

</Tabs>

## 升级引擎

如果您在上述步骤中，没有将 `upgradeAddons` 指定为 `true`，或者您想要使用的引擎不在默认 addon 列表中，但您想要使用 v0.9.0 API，可使用如下方式升级引擎。

:::note

- 如果您想要升级的引擎是 `mysql`，您需要升级 addon 并重启集群。否则使用 KubeBlocks v0.8 创建的集群将无法在 v0.9 中使用。

- 如果您想要升级 `clickhouse/milvus/elasticsearch/llm`，您需要先升级 KubeBlocks，再升级 addon，否在将无法在 v0.9 中正常使用。

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

```bash
# 添加 Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# 如果您无法访问 GitHub 或者网速过慢，可使用以下 repo
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# 更新 helm repo
helm repo update

# 更新引擎版本
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
