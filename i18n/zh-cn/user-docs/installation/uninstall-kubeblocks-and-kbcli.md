---
title: 卸载 KubeBlocks
description: 卸载 KubeBlocks
keywords: [kubeblocks, 卸载]
sidebar_position: 5
sidebar_label: 卸载 KubeBlocks 及 kbcli
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 卸载 KubeBlocks 及 kbcli

卸载顺序：

1. 卸载前，请确保所有集群和备份已删除

   ```bash
   kubectl delete cluster <clustername> -n namespace
   ```

   ```bash
   kubectl delete backup <backname> -n namespace
   ```

2. 卸载 KubeBlocks。

## 卸载 KubeBlocks

<Tabs>

<TabItem value="Helm" label="Helm" default>

在执行以下命令前，请删除之前创建的所有集群和资源，否则卸载可能无法成功。

```bash
helm uninstall kubeblocks --namespace kb-system
```

Helm 不会删除 CRD 对象。请使用以下命令删除 KubeBlocks 创建的对象。

```bash
kubectl get crd -o name | grep kubeblocks.io | xargs kubectl delete
```

</TabItem>

<TabItem value="YAML" label="YAML">

从 KubeBlocks chart 生成 YAML 文件，并使用 `kubectl` 进行卸载。使用 `--version x.y.z` 指定版本，确保卸载的版本与安装的版本相同。

```bash
helm template kubeblocks kubeblocks/kubeblocks --version x.y.z --namespace kb-system | kubectl delete -f -
```

</TabItem>

</Tabs>

## 卸载 kbcli

如果想在试用结束后删除 kbcli，请选择与安装 kbcli 时所使用的相同选项。

<Tabs>

<TabItem value="macOS" label="macOS" default>

如果你使用的是 `curl`，执行以下命令：

```bash
sudo rm /usr/local/bin/kbcli
```

如果你使用的是 `brew`，执行以下命令：

```bash
brew uninstall kbcli
```

kbcli 会在 HOME 目录下创建一个名为 `~/.kbcli` 的隐藏文件夹，用于存储配置信息和临时文件。你可以在卸载 kbcli 后删除此文件夹。

</TabItem>

<TabItem value="Windows" label="Windows">

1. 进入 `kbcli` 的安装路径，并删除安装文件夹。
  
    - 如果你通过脚本安装了 `kbcli`，请前往 `C:\Program Files` 并删除 `kbcli-windows-amd64` 文件夹。
    - 如果你自定义了安装路径，请前往指定路径，并删除安装文件夹。
  
2. 删除环境变量。
   1. 点击 Windows 图标，然后点击 **系统**。
   2. 进入 **设置** -> **相关设置** -> **高级系统设置**。
   3. 在 **高级** 标签页，点击 **环境变量**。
   4. 在 **用户变量** 或 **系统变量** 列表中，双击 **Path**。
       - 如果你通过脚本安装了 `kbcli`，双击 **用户变量** 中的 Path。
       - 如果你自定义了安装路径，请根据之前创建变量的位置，双击相应的 **Path**。
   5. 选择 `C:\Program Files\kbcli-windows-amd64` 或自定义的路径，并删除它。此操作需要二次确认。

3. 删除名为 `.kbcli` 的文件夹。

    kbcli 会在 C:\Users\username 目录下创建一个名为 `.kbcli` 的文件夹，用于存储配置信息和临时文件。你可以在卸载 kbcli 后删除此文件夹。

</TabItem>

<TabItem value="Linux" label="Linux">

使用 `curl` 命令卸载 kbcli。

```bash
sudo rm /usr/local/bin/kbcli
```

kbcli 会在 HOME 目录下创建一个名为 `~/.kbcli` 的隐藏文件夹，用于存储配置信息和临时文件。你可以在卸载 kbcli 后删除此文件夹。

</TabItem>

</Tabs>

