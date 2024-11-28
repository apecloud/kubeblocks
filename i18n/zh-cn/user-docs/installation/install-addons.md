---
title: 安装引擎
description: 使用 Helm 安装 KubeBlocks 支持的引擎
keywords: [引擎, helm, KubeBlocks]
sidebar_position: 4
sidebar_label: 安装引擎
---

# 安装引擎

KubeBlocks v0.8.0 发布后，数据库引擎插件（Addon）与 KubeBlocks 解耦，KubeBlocks 默认安装了部分引擎，如需体验其它引擎，需通过索引安装。如果您卸载了部分引擎，也可通过本文步骤，重新安装。

本文以 elasticsearch 为例，可根据实际情况替换引擎名称。

官网引擎索引仓库为 [KubeBlocks 索引](https://github.com/apecloud/block-index)。引擎代码维护在 [KubeBlocks 引擎插件仓库](https://github.com/apecloud/kubeblocks-addons)。

:::note

请确保 Addon 和 KubeBlocks 的大版本匹配。

例如，如果您当前使用的 KubeBlocks 版本是 v0.9.2，Addon 应安装对应的 v0.9.0，而不是其他版本（如 v0.8.0），否则系统将因版本不匹配产生报错。

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

1. （可选）添加 KubeBlocks 仓库。如果您使用 Helm 安装了 KubeBlocks 或此前已添加过该仓库，只需执行 `helm repo update`。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. 查看引擎版本。

   ```bash
   helm search repo kubeblocks/elasticsearch --devel --versions
   ```

3. 以 elasticsearch 为例，安装引擎。使用 `--version` 指定版本。

   ```bash
   helm install kb-addon-es kubeblocks/elasticsearch --namespace kb-system --create-namespace --version x.y.z
   ```

4. 验证该引擎是否安装成功。

   如果状态显示为 `deployed`，则表明该引擎已成功安装。

   ```bash
   helm list -A
   >
   NAME                 NAMESPACE	REVISION	UPDATED                                	STATUS  	 CHART                             APP VERSION
   ......
   kb-addon-es          kb-system	1       	2024-11-27 10:04:59.730127 +0800 CST   	deployed	 elasticsearch-0.9.0               	8.8.2   
   ```

5. （可选）您可以执行以下命令卸载引擎。如果您已使用该引擎创建集群，请先删除集群。

   ```bash
   helm uninstall kb-addon-es --namespace kb-system
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 查看引擎仓库索引。

   kbcli 默认创建名为 `kubeblocks` 的索引，可使用 `kbcli addon index list` 查看该索引。

   ```bash
   kbcli addon index list
   >
   INDEX        URL
   kubeblocks   https://github.com/apecloud/block-index.git 
   ```

   如果命令执行结果未显示或者你想要添加自定义索引仓库，则表明索引未建立，可使用 `kbcli addon index add <index-name> <source>` 命令手动添加索引。例如，

   ```bash
   kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git
   ```

   如果不确定索引是否为最新版本，可使用如下命令更新索引。

   ```bash
   kbcli addon index update kubeblocks
   ```

2. （可选）索引建立后，可以通过 `addon search` 命令检查想要安装的引擎是否在索引信息中存在。

   ```bash
   kbcli addon search elasticsearch
   >
   ADDON           VERSION         INDEX
   elasticsearch   0.7.0           kubeblocks
   elasticsearch   0.7.1           kubeblocks
   elasticsearch   0.7.2           kubeblocks
   elasticsearch   0.8.0           kubeblocks
   elasticsearch   0.9.0           kubeblocks
   ```

3. 安装引擎。

   当引擎有多个版本和索引源时，可使用 `--index` 指定索引源，`--version` 指定安装版本。系统默认以 `kubeblocks` 索引仓库为索引源，安装最新版本。

   ```bash
   kbcli addon install elasticsearch --index kubeblocks --version x.y.z
   ```

4. 验证该引擎是否安装成功。

   如果状态显示为 `Enabled`，则表明该引擎已成功安装。

   ```bash
   kbcli addon list
   >
   NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
   elasticsearch                  0.9.0          apecloud    Enabled    true
   ```

5. （可选）您可以执行以下命令停用引擎。如果您已使用该引擎创建集群，请先删除集群。

   ```bash
   kbcli addon disable elasticsearch
   ```

   您也可以卸载该引擎。

   ```bash
   kbcli addon uninstall elasticsearch
   ```

:::note

kbcli 支持启用/停用引擎。您可以按需调整引擎启用状态。此外，使用 kbcli 安装 KubeBlocks 时，系统默认安装但禁用了部分引擎，这类引擎的状态为 `Disabled`，您可以通过 kbcli 启用这类引擎。例如，

```bash
kbcli addon list
>
NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
alertmanager-webhook-adaptor   0.1.4          apecloud    Disabled   false
apecloud-otel-collector        0.1.2-beta.3   apecloud    Disabled   false
aws-load-balancer-controller   1.4.8          community   Disabled   false
csi-driver-nfs                 4.5.0          community   Disabled   false
csi-hostpath-driver            0.7.0          community   Disabled   false
grafana                        6.43.5         community   Disabled   false
llm                            0.9.0          community   Disabled   false
prometheus                     15.16.1        community   Disabled   false
qdrant                         0.9.1          community   Disabled   false
victoria-metrics-agent         0.8.41         community   Disabled   false
```

* 启用引擎。

   ```bash
   kbcli addon enable qdrant
   ```

* 禁用引擎。

   ```bash
   kbcli addon disable qdrant
   ```

启用/禁用引擎后，查看引擎列表，检查引擎状态是否按需变更。

```bash
kbcli addon list
>
NAME                           VERSION        PROVIDER    STATUS     AUTO-INSTALL
qdrant                         0.9.1          community   Enabled    false
```

:::

</TabItem>

</Tabs>
