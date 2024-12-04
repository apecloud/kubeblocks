---
title: 升级到 KubeBlocks v0.9.1
description: KubeBlocks v0.9.1 升级文档
keywords: [升级, 0.9.1, Helm]
sidebar_position: 1
sidebar_label: 升级到 KubeBlocks v0.9.1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 升级到 KubeBlocks v0.9.1

本文档将介绍如何升级至 KubeBlocks v0.9.1。

:::note

- 在升级前，请先执行 `helm -n kb-system list | grep kubeblocks` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。
- v0.9.2 的升级操作与 v0.9.1 相同，可按需替换版本，按照本文档操作。

:::

## 兼容性说明

KubeBlocks v0.9.1 可以兼容 KubeBlocks v0.8 的 API，但不保证兼容 v0.8 之前版本的 API，如果您正在使用 KubeBlocks v0.7 或者更老版本的引擎（版本号为 `0.7.x`, `0.6.x`），请务必参考 [v0.8 升级文档](./upgrade-kubeblocks-to-0.8.md)将 KubeBlocks 升级至 v0.8 并将所有引擎升级至 0.8，以确保升级至 v0.9 版本后服务的可用性。

如果您是从 v0.8 升级到 v0.9，需要打开 webhook，以确保可用性。

## 从 v0.9.0 升级

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. 查看引擎，确认引擎是否已添加 `"helm.sh/resource-policy": "keep"` 注解。

    KubeBlocks 对默认安装的引擎做了精简。添加 `"helm.sh/resource-policy": "keep"` 注解可以避免升级时删除已经在使用的引擎资源。

    查看引擎中是否添加了 `"helm.sh/resource-policy": "keep"`。

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
    ```

    如果没有该注解，可以手动执行以下命令，为引擎添加注解。可以把 `-l app.kubernetes.io/name=kubeblocks` 替换为您所需的过滤条件。

    ```bash
    kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
    ```

2. 安装 CRD。

    为避免 KubeBlocks Helm Chart 过大，0.8 版本将 CRD 从 Helm Chart 中移除了，升级前需要先安装 CRD。

    ```bash
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.1/kubeblocks_crds.yaml
    ```

3. 升级 KubeBlocks。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 --set crd.enabled=false
    ```

    KubeBocks v0.9 到 v0.9.1 的升级不涉及 API 变更，可通过设置 `--set crd.enabled=false` 跳过 API 升级任务。

    :::warning

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.1 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9.1 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 \
      --set upgradeAddons=true \
      --set crd.enabled=false
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 下载 kbcli v0.9.1。

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.1
    ```

2. 升级 KubeBlocks。

    ```bash
    kbcli kb upgrade --version 0.9.1
    ```

    :::warning

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.1 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9.1 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    kbcli kb upgrade --version 0.9.1 --set upgradeAddons=true
    ```

    :::

   kbcli 会默认为已有引擎添加 `"helm.sh/resource-policy": "keep"` 注解，确保升级过程中已有引擎不会被删除。

</TabItem>

</Tabs>

## 从 v0.8.x 升级

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. 查看引擎，确认引擎是否已添加 `"helm.sh/resource-policy": "keep"` 注解。

    KubeBlocks 对默认安装的引擎做了精简。添加 `"helm.sh/resource-policy": "keep"` 注解可以避免升级时删除已经在使用的引擎资源。

    查看引擎中是否添加了 `"helm.sh/resource-policy": "keep"`。

    ```bash
    kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
    ```

    如果没有该注解，可以手动执行以下命令，为引擎添加注解。可以将 `-l app.kubernetes.io/name=kubeblocks` 替换为您所需的过滤条件。

    ```bash
    kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
    ```

2. 删除不兼容的 OpsDefinition。

   ```bash
   kubectl delete opsdefinitions.apps.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. 安装 CRD。

    为避免 KubeBlocks Helm Chart 过大，0.8 版本将 CRD 从 Helm Chart 中移除了，变更了 StorageProvider 的 group。升级前，需要先安装 StorageProvider CRD。

    如果网络较慢，建议先下载 CRD YAML 文件到本地，再执行操作。

    ```bash
    kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.1/dataprotection.kubeblocks.io_storageproviders.yaml
    ```

4. 升级 KubeBlocks。

    请关注以下选项：

    - 设置 `admissionWebhooks.enabled=true` 将启动 webhook，用于 ConfigConstraint API 多版本转换。
    - 设置 `admissionWebhooks.ignoreReplicasCheck=true` 默认只有在 3 副本部署 KubeBlocks 时才可开启 webhook。若只部署单副本 KubeBlocks，可配置该变量跳过检查。
    - 如果您当前运行的 KubeBlocks 使用的镜像仓库为 `infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com`，升级时请显式设置镜像仓库。具体可参考 [FAQ](./faq.md#升级时如何指定镜像仓库)

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts

    helm repo update kubeblocks

    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::warning

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.1 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9.1 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 \
      --set upgradeAddons=true \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true 
    ```

    :::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 下载 kbcli v0.9.1。

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.9.1
    ```

2. 升级 KubeBlocks。

    查看 kbcli 版本，确保您使用的 kbcli 版本为 v0.9.1。

    ```bash
    kbcli version
    ```

    请关注以下选项：

    - 设置 `admissionWebhooks.enabled=true` 将启动 webhook，用于 ConfigConstraint API 多版本转换。
    - 设置 `admissionWebhooks.ignoreReplicasCheck=true` 默认只有在 3 副本部署 KubeBlocks 时才可开启 webhook。若只部署单副本 KubeBlocks，可配置该变量跳过检查。
    - 如果您当前运行的 KubeBlocks 使用的镜像仓库为 `infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com`，升级时请显式设置镜像仓库。具体可参考 [FAQ](./faq.md#升级时如何指定镜像仓库)。

    ```bash
    kbcli kb upgrade --version 0.9.1 \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::warning

    为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.9.1 时，默认不会升级已经安装的引擎版本，如果要升级引擎版本至 KubeBlocks v0.9.1 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    kbcli kb upgrade --version 0.9.1 \
      --set upgradeAddons=true \
      --set admissionWebhooks.enabled=true \
      --set admissionWebhooks.ignoreReplicasCheck=true
    ```

    :::

    kbcli 会默认为已有引擎添加 `"helm.sh/resource-policy": "keep"` 注解，确保升级过程中已有引擎不会被删除。

</TabItem>

</Tabs>

## 升级引擎

如果您在上述步骤中，没有将 `upgradeAddons` 指定为 `true`，或者您想要使用的引擎不在默认列表中，但您想要使用 v0.9.1 API，可使用如下方式升级引擎。

:::note

- 如果您想要升级的引擎是 `mysql`，您需要升级引擎并重启集群。否则使用 KubeBlocks v0.8.x 创建的集群将无法在 v0.9.1 中使用。

- 如果您想要升级 `clickhouse/milvus/elasticsearch/llm`，您需要先升级 KubeBlocks，再升级引擎，否在将无法在 v0.9.1 中正常使用。

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

# 检索可用的引擎版本
helm search repo kubeblocks-addons/{addon-name} --versions --devel 

# 更新引擎版本
helm upgrade -i {addon-release-name} kubeblocks-addons/{addon-name} --version x.y.z -n kb-system   
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
