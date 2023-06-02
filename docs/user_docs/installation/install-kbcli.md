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

 You can install kbcli on both local and cloud environment.

## Environment preparation

- Minimum system requirements:
  - MacOS:
    - CPU: 4 cores
    - Memory: 4 GB
    Check CPU with the following command: `sysctl hw.physicalcpu`; 
    Check memory with the following command: `top -d`.
  - Windows:
    - 64-bit
- Ensure the following tools are installed on your local host:
  - Docker: v20.10.5 (runc ≥ v1.0.0-rc93) or higher. For installation details, see Get Docker.
  - kubectl: used to interact with Kubernetes clusters.
  - For Windows environment, PowerShell version 5.0 or higher is required.

## Install kbcli

You can install the kbcli and KubeBlocks on your local host, and now MacOS and Windows are supported.

<Tabs>
<TabItem value="MacOS" label="MacOS" default>

You can install kbcli with `curl` or `homebrew`.

- Option 1: Install kbcli using the `curl` command.

1. Install kbcli.

   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```

2. Run `kbcli version` to check the version of kbcli and ensure that it is successfully installed.

:::note

- If a timeout exception occurs during installation, please retry and check your network settings.

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

1. Use PowerShell and run Set-ExecutionPolicy Unrestricted.
2. Install kbcli.The following script will automatically install the environment variables at C:\Program Files\kbcli-windows-amd64.

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr [https://www.kubeblocks.io/installer/install_cli.ps1])))"
    ```

    To install a specified version of kbcli, use -v after the command and describe the version you want to install.

    ```bash
    powershell -Command " & ([scriptblock]::Create((iwr <https://www.kubeblocks.io/installer/install_cli.ps1>))) -v 0.5.0-beta.1"
    ```

- Option 2: Install using the installation package.

1. Download the kbcli installation zip package from GitHub.
2. Extract the file and add it to the environment variables.
    1. Click the Windows icon and select **System Settings**.
    2. Click **Settings** -> **Related Settings** -> **Advanced system settings**.
    3. Click **Environment Variables** on the **Advanced** tab.
    4. Click **New** to add the path of the kbcli installation package to the user and system variables.
    5. Click **Apply** and **OK**.

</TabItem>
</Tabs>

## (Optional) Enable kbcli automatic command line completion

`kbcli` supports automatic command line completion. You can run the command below to view the user guide and enable this function.

```bash
# Configure SHELL-TYPE as one type from bash, fish, PowerShell, and zsh
kbcli completion SHELL-TYPE -h
```

For example, enable command line completion for zsh.

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