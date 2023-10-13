---
title: Install kbcli 
description: Install kbcli 
keywords: [install, kbcli,]
sidebar_position: 2
sidebar_label: Install kbcli
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install kbcli

You can install kbcli on your laptop or virtual machines on the cloud.

## Environment preparation

For Windows users, PowerShell version should be 5.0 or higher.

## Install kbcli

kbcli now supports macOS, Windows, and Linux.

<Tabs>
<TabItem value="macOS" label="macOS" default>

You can install kbcli with `curl` or `brew`.

- Option 1: Install kbcli using the `curl` command.

1. Install kbcli.

   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```

   If you want to install kbcli with a specified version, follow the steps below.

   1. Check the available versions in [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/).
   2. Specify a version with `-s` and run the command below.

      ```bash
      curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash -s x.x.x
      ```

  :::note

  By default, kbcli installs the latest release version and then when installing KubeBlocks, kbcli installs the matched version. Ensure that the major versions of kbcli and KubeBlocks match.

  For instance, you can install kbcli v0.6.1 and KubeBlocks v0.6.3, but mismatched versions like kbcli v0.5.0 and KubeBlocks v0.6.0 may result in errors.

  :::

2. Run `kbcli version` to check the version of kbcli and ensure that it is successfully installed.

  :::note

  If a timeout exception occurs during installation, please check your network settings and retry.

  :::

- Option 2: Install kbcli using Homebrew.

1. Install ApeCloud tap, the Homebrew package of ApeCloud.

   ```bash
   brew tap apecloud/tap
   ```

2. Install kbcli.

   ```bash
   brew install kbcli
   ```

   If you want to install kbcli with a specified version, run the commands below.

   ```bash
   # View the available version
   brew search kbcli

   # Specify a version
   brew install kbcli@x.x.x
   ```

3. Verify that kbcli is successfully installed.

   ```bash
   kbcli -h
   ```

</TabItem>

<TabItem value="Windows" label="Windows">

There are two ways to install kbcli on Windows:

- Option 1: Install using the script.

:::note

By default, the script will be installed at C:\Program Files\kbcli-windows-amd64 and cannot be modified.

If you need to customize the installation path, use the zip file.

:::

1. Run PowerShell as an **administrator** and execute `Set-ExecutionPolicy Unrestricted`.
2. Install kbcli.  

   The following script will automatically install the environment variables at C:\Program Files\kbcli-windows-amd64.

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1)))"
    ```

    To install a specified version of kbcli, use `-v` after the command and describe the version you want to install.

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1))) -v 0.5.2"
    ```

- Option 2: Install using the installation package.

1. Download the kbcli installation zip package from [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/).
2. Extract the file and add it to the environment variables.
    1. Click the Windows icon and select **System Settings**.
    2. Click **Settings** -> **Related Settings** -> **Advanced system settings**.
    3. Click **Environment Variables** on the **Advanced** tab.
    4. Click **New** to add the path of the kbcli installation package to the user and system variables.
    5. Click **Apply** and **OK**.

</TabItem>

<TabItem value="Linux" label="Linux">

Install kbcli using the `curl` command.

1. Install kbcli.

   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```

2. Run `kbcli version` to check the version of kbcli and ensure that it is successfully installed.

:::note

- If a timeout exception occurs during installation, please check your network settings and retry.

:::

</TabItem>
</Tabs>

## (Optional) Enable auto-completion for kbcli

`kbcli` supports command line auto-completion.

```bash
# Configure SHELL-TYPE as one type from bash, fish, PowerShell, and zsh
kbcli completion SHELL-TYPE -h
```

For example, enable kbcli auto-completion for zsh.

***Steps:***

1. Check the user guide.

    ```bash
    kbcli completion zsh -h
    ```

2. Enable the completion function of your terminal first.

    ```bash
    echo "autoload -U compinit; compinit" >> ~/.zshrc
    ```

3. Enable the `kbcli` automatic completion function.

    ```bash
    echo "source <(kbcli completion zsh); compdef _kbcli kbcli" >> ~/.zshrc
    ```
