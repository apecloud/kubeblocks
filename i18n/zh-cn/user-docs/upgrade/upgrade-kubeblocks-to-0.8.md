---
title: 升级到 KubeBlocks v0.8
description: 升级到 KubeBlocks v0.8, 升级操作
keywords: [升级, 0.8]
sidebar_position: 3
sidebar_label: 升级到 KubeBlocks v0.8
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 升级到 KubeBlocks v0.8

本文档介绍如何从低版本 KubeBlocks 升级至 KubeBlocks 0.8 版本。

:::note

在升级前，请先执行 `helm -n kb-system list | grep kubeblocks` 或者 `kbcli version` 查看正在使用的 KubeBlocks 版本，并根据不同版本，执行升级操作。

:::

## 从 v0.7 升级

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. 设置 keepAddons。

    KubeBlocks v0.8 精简了默认安装的引擎，将引擎从 KubeBlocks operator 分离到了 KubeBlocks 引擎插件代码仓库中，例如 greptime，influxdb，neon，oracle-mysql，orioledb，tdengine，mariadb，nebula，risingwave，starrocks，tidb，zookeeper。为避免升级时将已经在使用的引擎资源删除，请先执行如下命令。

    - 查看当前 KubeBlocks 版本。

       ```bash
       helm -n kb-system list | grep kubeblocks
       ```

    - 查看并添加引擎注解。

        执行如下命令，查看引擎注解中是否添加了 `"helm.sh/resource-policy": "keep"`。

        ```bash
        kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
        ```

        如果没有该注解，可以手动执行以下命令，为引擎添加注解。可以将 `-l app.kubernetes.io/name=kubeblocks` 替换为您所需的过滤条件。

        ```bash
        kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
        ```

2. 安装 CRD。

    为避免 KubeBlocks 的 Helm chart 过大，v0.8 版本将 CRD 从 Helm chart 中移除了，升级前需要先安装 CRD。

    ```bash
    kubectl replace -f https://github.com/apecloud/kubeblocks/releases/download/v0.8.1/kubeblocks_crds.yaml
    ```

3. 升级 KubeBlocks。

    ```bash
    helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.8.1 --set dataProtection.image.datasafed.tag=0.1.0
    ```

:::note

为避免影响已有的数据库集群，升级 KubeBlocks 至 v0.8 时，默认不会升级已经安装的引擎版本。如果要将引擎升级至 KubeBlocks v0.8 内置引擎的版本，可以执行如下命令。注意，该操作可能导致已有集群发生重启，影响可用性，请务必谨慎操作。

```bash
helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.8.1 --set upgradeAddons=true
```

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 下载 kbcli v0.8 版本。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.8.1
    ```

2. 升级 KubeBlocks。

    ```shell

    kbcli kb upgrade --version 0.8.1 --set dataProtection.image.datasafed.tag=0.1.0

    ```

    kbcli 会默认为已有 addon 添加 annotation `"helm.sh/resource-policy": "keep"`，确保升级过程中已有的 addon 不会被删除。

</TabItem>

</Tabs>

## 从 v0.6 版本升级

如果当前使用的是 KubeBlocks v0.6 版本，请先将 KubeBlocks 升级至 v0.7.2 版本，操作如下：

1. 下载 kbcli v0.7.2 版本。

    ```shell
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s 0.7.2
    ```

2. 升级至 KubeBlocks v0.7.2。

    ```shell
    kbcli kb upgrade --version 0.7.2
    ```

## FAQ

可查看 [FAQ](./../faq.md)，了解并解决升级 KubeBlocks 过程中的常见问题。如果您还遇到了其他问题，可以[提交 issue](https://github.com/apecloud/kubeblocks/issues/new/choose) 或者在 [GitHub 讨论区](https://github.com/apecloud/kubeblocks/discussions)提交您的问题。
