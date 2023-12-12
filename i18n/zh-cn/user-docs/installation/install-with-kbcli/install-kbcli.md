---
title: 安装 kbcli 
description: 安装 kbcli 
keywords: [安装, kbcli,]
sidebar_position: 2
sidebar_label: 安装 kbcli
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 安装 kbcli

你可以将 `kbcli` 安装在笔记本电脑或者云上的虚拟机上。

## 环境准备

Windows 用户需配置 PowerShell 5.0 及以上的版本。

## 安装 kbcli

kbcli 目前支持 macOS、Windows 和 Linux 系统。

<Tabs>
<TabItem value="macOS" label="macOS" default>

使用 `curl` 或 `brew` 安装 kbcli。

- 选项 1：执行 `curl` 命令安装 kbcli

  1. 安装 kbcli。

      ```bash
      curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
      ```

      如果想安装 kbcli 的指定版本，请按照以下步骤进行操作：
      
      1. 在 [KubeBlocks Release 页面](https://github.com/apecloud/kubeblocks/releases/)中查看可用版本。
      2. 使用 `-s` 指定版本，并执行以下命令。
        
          ```bash
          curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s x.x.x
          ```
    
      :::note

      kbcli 默认安装最新版本。在安装 KubeBlocks 时，kbcli 会安装与之匹配的版本。请确保 kbcli 和 KubeBlocks 的主版本号相匹配。

      例如，你可以安装 kbcli v0.6.1 和 KubeBlocks v0.6.3。但是，如果安装的是 kbcli v0.5.0 和 KubeBlocks v0.6.0，就可能会报错，因为版本不匹配。

      :::

  2. 执行 `kbcli version` 命令，检查 kbcli 版本并确保已成功安装。

      :::note

      如果安装超时，请检查你的网络设置并重试。

      :::

- 选项 2：用 Homebrew 安装 kbcli

  1. 安装 ApeCloud 的 Homebrew 包（ApeCloud tap）。
     
      ```bash
      brew tap apecloud/tap
      ```

  2. 安装 kbcli。
     
      ```bash
      brew install kbcli
      ```
      
      如果想安装 kbcli 的指定版本，执行：

      ```bash
      # 查看可用版本 
      brew search kbcli

      # 安装指定版本
      brew install kbcli@x.x.x
      ```
     
  3. 确认 kbcli 是否已成功安装。

      ```bash
      kbcli -h
      ```

</TabItem>

<TabItem value="Windows" label="Windows">

有两种方法可以在 Windows 上安装 kbcli。

- 选项 1：使用脚本安装

:::note

默认情况下，脚本将安装在 C:\Program Files\kbcli-windows-amd64，且无法修改。

如果需要自定义安装路径，请使用压缩文件。

:::

1. 以**管理员**身份执行 PowerShell，并执行 `Set-ExecutionPolicy Unrestricted`。
2. 安装 `kbcli`。

   以下脚本将自动在 C:\Program Files\kbcli-windows-amd64 安装环境变量。

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1)))"
    ```

    如果想安装 kbcli 的指定版本，在上述命令后面加上 `-v` 和你想安装的版本号。

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1))) -v 0.5.2"
    ```
  
- 选项 2：使用安装包安装

1. 在 [KubeBlocks Release 页面](https://github.com/apecloud/kubeblocks/releases/)下载 kbcli 安装包。
2. 解压文件并将其添加到环境变量中。
   1. 单击 Windows 图标，选择**系统设置**。
   2. 点击**设置** -> **相关设置** -> **高级系统设置**。
   3. 在**高级**选项卡上，点击**环境变量**。
   4. 点击**新建**，将 kbcli 安装包的路径添加到用户和系统变量中。
   5. 点击**应用**和**确定**。

</TabItem>

<TabItem value="Linux" label="Linux">

使用 `curl` 命令安装 kbcli。

1. 安装 kbcli。

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
    ```

2. 执行 `kbcli version` 命令，检查 `kbcli` 版本并确保已成功安装。

:::note

如果安装超时，请检查你的网络设置并重试。

:::

</TabItem>
</Tabs>

## (可选) 启用 kbcli 的自动补全功能

`kbcli` 支持命令行自动补全。 

```bash
# 配置 SHELL-TYPE 为 bash、fish、PowerShell、zsh 中的一种
kbcli completion SHELL-TYPE -h
```

举个例子，如果想要启用 zsh 的 kbcli 自动补全功能：

***步骤：***

1. 查阅用户指南。

   ```bash
   kbcli completion zsh -h
   ```

2. 启用终端的补全功能。

   ```bash
   echo "autoload -U compinit; compinit" >> ~/.zshrc
   ```

3. 启用 `kbcli` 的自动补全功能。

   ```bash
   echo "source <(kbcli completion zsh); compdef _kbcli kbcli" >> ~/.zshrc
   ```
