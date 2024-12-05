---
title: 升级至 v0.9.0
description: 升级至 KubeBlocks v0.9.0, 升级操作
keywords: [升级, 0.9.0]
draft: true
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 升级到 KubeBlocks v0.9.0

本文档将介绍如何升级至 KubeBlocks v0.9.0。

:::note

在升级前，请先执行 `helm -n kb-system list | grep kubeblocks` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。

:::

## 兼容性说明

KubeBlocks 0.9.0 可以兼容 KubeBlocks 0.8 的 API，但不保证兼容 0.8 之前版本的 API，如果您正在使用 KubeBlocks 0.7 或者更老版本的引擎（版本号为 `0.7.x`, `0.6.x`），请务必参考 [0.8 升级文档](./upgrade-kubeblocks-to-0.8.md)将 KubeBlocks 升级至 0.8 并将所有引擎升级至 0.8，以确保升级至 0.9.0 版本后服务的可用性。

## 从 v0.8 版本升级

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. 为引擎添加 `"helm.sh/resource-policy": "keep"` 注解。

    KubeBlocks v0.8 对默认安装的引擎做了精简。添加 `"helm.sh/resource-policy": "keep"` 注解可以避免升级时删除已经在使用的引擎资源。

    执行以下命令，为引擎添加 `"helm.sh/resource-policy": "keep"` 注解。可以将 `-l app.kubernetes.io/name=kubeblocks` 替换为您所需的过滤条件。

    ```bash
    kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
    ```

    查看引擎中是否已包含 `"helm.sh/resource-policy": "keep"`。

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
    ```

2. 删除不兼容的 OpsDefinition。

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. 安装 StorageProvider CRD。

   如果网络较慢，建议先下载 CRD YAML 文件到本地，再执行操作。

    ```shell
    kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/dataprotection.kubeblocks.io_storageproviders.yaml
    ```

4. 升级 KubeBlocks。

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0
    ```

    :::note

    避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.0 时，默认不会升级已经安装的引擎版本，如果要将引擎版本至 KubeBlocks v0.9 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 \
      --set upgradeAddons=true
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 下载 kbcli v0.9.0。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.0
    ```

2. 升级 KubeBlocks。

    ```bash
    kbcli kb upgrade --version 0.9.0 
    ```

    :::note

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.0 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9.0 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    kbcli kb upgrade --version 0.9.0 --set upgradeAddons=true
    ```

    :::

    kbcli 会默认为已有引擎添加 `"helm.sh/resource-policy": "keep"` 注解，确保升级过程中已有引擎不会被删除。

</TabItem>

</Tabs>

## 升级引擎

为了使用 v0.9.0 的 API，如果在上述步骤中，没有指定 `upgradeAddons`，或者您的引擎不在默认引擎列表里，可使用如下方式升级引擎。

:::note

- 如果您使用的引擎是 MySQL, 需要升级引擎并重启集群，否则 KubeBlocks v0.8 创建的集群将无法在 v0.9.0 中使用。

- 如果您要使用 `clickhouse/milvus/elasticsearch/llm` 等引擎，需要升级 KubeBlocks 之后，再升级引擎，否则无法在 v0.9.0 正常使用。

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

```bash
# 添加 Helm 仓库
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# 如果无法访问 Github 或者访问速度过慢，可使用以下仓库地址
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# 更新 Helm 仓库
helm repo update

# 升级引擎
helm upgrade -i xxx kubeblocks-addons/xxx --version x.y.z -n kb-system  
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
# 查看引擎索引列表
kbcli addon index list

# 更新某一个索引， 默认的是 kubeblocks
kbcli addon index update kubeblocks

# 检索可用的引擎版本
kbcli addon search {addon-name}

# 安装引擎
kbcli addon install {addon-name} --version x.y.z

# 更新引擎到指定版本
kbcli addon upgrade {addon-name} --version x.y.z

# 强制更新引擎到指定版本
kbcli addon upgrade {addon-name} --version x.y.z --force

# 查看指定引擎版本
kbcli addon list | grep {addon-name}
```

</TabItem>

</Tabs>

## FAQ

可查看 [FAQ](./../faq.md)，了解并解决升级 KubeBlocks 过程中的常见问题。如果您还遇到了其他问题，可以[提交 issue](https://github.com/apecloud/kubeblocks/issues/new/choose) 或者在 [GitHub 讨论区](https://github.com/apecloud/kubeblocks/discussions)提交您的问题。
