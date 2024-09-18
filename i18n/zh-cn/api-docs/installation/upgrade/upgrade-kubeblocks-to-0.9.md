---
title: 升级到 KubeBlocks v0.9
description: 升级到 KubeBlocks v0.9, 升级操作
keywords: [升级, 0.9]
sidebar_position: 1
sidebar_label: 升级到 KubeBlocks v0.9
---

# 升级到 KubeBlocks v0.9

本文档将介绍如何升级至 KubeBlocks v0.9。

:::note

在升级前，请先执行 `helm -n kb-system list | grep kubeblocks` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。

:::

## 兼容性说明

KubeBlocks 0.9 可以兼容 KubeBlocks 0.8 的 API，但不保证兼容 0.8 之前版本的 API，如果您正在使用 KubeBlocks 0.7 或者更老版本的 Addon（版本号为 `0.7.*`, `0.6.*`），请务必参考 [0.8 升级文档](./upgrade-kubeblocks-to-0.8.md)将 KubeBlocks 升级至 0.8 并将所有引擎升级至 0.8，以确保升级至 0.9 版本后服务的可用性。

## 从 v0.8 版本升级

1. 设置 keepAddons。

    KubeBlocks 0.8 对默认安装的引擎做了精简。为避免升级时将已经在使用的引擎资源删除，请先执行如下命令确保升级过程中这些引擎不会被删除。

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

    - 查看引擎。

         执行如下命令，确保 addon annotations 中添加了 `"helm.sh/resource-policy": "keep"`。

         ```shell
         kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
         ```

2. 删除不兼容的OpsDefinition

   ```bash
   kubectl delete opsdefinitions.operations.kubeblocks.io kafka-quota kafka-topic kafka-user-acl switchover
   ```

3. 安装 CRD。

   为避免 KubeBlocks Helm Chart 过大，0.8 版本将 CRD 从 Helm Chart 中移除了，升级前需要先安装 CRD。

    ```shell
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/kubeblocks_crds.yaml
    ```

4. 升级 KubeBlocks。

    ```shell
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=false
    ```

    :::note

    避免影响已有的数据库集群，升级 KubeBlocks 至 0.9 时，默认不会升级已经安装的引擎版本，如果要将引擎版本至 KubeBlocks 0.9 内置引擎的版本，可以执行如下命令，这可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.0 --set upgradeAddons=true
    ```

    :::

## 升级引擎

为了使用 v0.9.0 的API，如果在上述步骤中，没有指定 `upgradeAddons`，或者您的引擎不在默认引擎列表里，可使用如下方式升级引擎。

:::note

如果您使用的引擎是 MySQL, 需要升级引擎并重启集群，否则 KubeBlocks v0.8 创建的集群将无法在 v0.9 中使用。

如果您要使用 clickhouse/milvus/elasticsearch/llm 等引擎，需要升级 KubeBlocks 之后，再升级引擎，否则无法在 v0.9 正常使用。

:::

```bash
# 添加 Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts

# 如果无法访问 Github 或者访问速度过慢，可使用以下仓库地址
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# 更新 helm repo
helm repo update

# 升级引擎 version
helm upgrade -i xxx kubeblocks-addons/xxx --version x.x.x -n kb-system  
```
