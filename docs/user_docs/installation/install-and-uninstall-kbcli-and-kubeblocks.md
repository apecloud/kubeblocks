---
title: Install and uninstall kbcli and KubeBlocks
description: Install KubeBlocks and kbcli developed by ApeCloud
keywords: [kbcli, kubeblocks, install, uninstall, windows, macOS]
sidebar_position: 1
sidebar_label: kbcli and KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install and uninstall kbcli and KubeBlocks

This guide introduces how to install KubeBlocks by `kbcli`, the command line tool of KubeBlocks.

## Before you start

1. A Kubernetes environment is required.
2. `kubectl` is required and can connect to your Kubernetes clusters. Refer to [Install and Set Up kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) for installation details.

## Install kbcli

`kbcli` can be installed on macOS and Windows.

<Tabs>
<TabItem value="macOS" label="macOS" default>

For macOS, cURL and Homebrew options are supported.

<TabItem value="cURL" label="cURL" default>

1. Run the command below to install `kbcli`.

    ```bash
    curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
    ```

    :::note

    Please try again if a time-out exception occurs during installation. It may relate to your network condition.

    :::

2. Check the version and verify whether `kbcli` is installed successfully.

    ```bash
    kbcli version
    ```

</TabItem>
<TabItem value="Homebrew" label="Homebrew">

1. Install the ApeCloud tap, a repository of all Homebrew packages of ApeCloud.

   ```bash
   brew tap apecloud/tap
   ```

   <details>

   <summary>Expected output</summary>

   ```bash
   ==> Tapping apecloud/tap
   Cloning into '/opt/homebrew/Library/Taps/apecloud/homebrew-tap'...
   remote: Enumerating objects: 59, done.
   remote: Counting objects: 100% (59/59), done.
   remote: Compressing objects: 100% (44/44), done.
   remote: Total 59 (delta 8), reused 48 (delta 5), pack-reused 0
   Receiving objects: 100% (59/59), 14.21 KiB | 4.74 MiB/s, done.
   Resolving deltas: 100% (8/8), done.
   Tapped 2 formulae (16 files, 37.8KB). 
   ```

   </details>

2. Install `kbcli`.

   ```bash
   brew install kbcli
   ```

   <details>

   <summary>Expected output</summary>

   ```bash
   ==> Fetching apecloud/tap/kbcli
   ==> Downloading https:/.../kubeblocks/v0.4.0/kbcli-darwin-arm64-v0.4.0.tar.gz
   ==> Installing kbcli from apecloud/tap
   ==> Caveats
   zsh completions have been installed to:
     /opt/homebrew/share/zsh/site-functions
   ==> Summary
   🍺  /opt/homebrew/Cellar/kbcli/0.4.0: 5 files, 87.7MB, built in 2 seconds
   ==> Running `brew cleanup kbcli`...
   Disable this behaviour by setting HOMEBREW_NO_INSTALL_CLEANUP.
   Hide these hints with HOMEBREW_NO_ENV_HINTS (see `man brew`). 

   </details>

3. Verify whether `kbcli` is installed successfully.

   ```bash
   kbcli -h
   ```

   <details>

   <summary>Expect output</summary>

   ```bash
   =============================================
    __    __ _______   ______  __       ______
   |  \  /  \       \ /      \|  \     |      \
   | ▓▓ /  ▓▓ ▓▓▓▓▓▓▓\  ▓▓▓▓▓▓\ ▓▓      \▓▓▓▓▓▓
   | ▓▓/  ▓▓| ▓▓__/ ▓▓ ▓▓   \▓▓ ▓▓       | ▓▓
   | ▓▓  ▓▓ | ▓▓    ▓▓ ▓▓     | ▓▓       | ▓▓
   | ▓▓▓▓▓\ | ▓▓▓▓▓▓▓\ ▓▓   __| ▓▓       | ▓▓
   | ▓▓ \▓▓\| ▓▓__/ ▓▓ ▓▓__/  \ ▓▓_____ _| ▓▓_
   | ▓▓  \▓▓\ ▓▓    ▓▓\▓▓    ▓▓ ▓▓     \   ▓▓ \
    \▓▓   \▓▓\▓▓▓▓▓▓▓  \▓▓▓▓▓▓ \▓▓▓▓▓▓▓▓\▓▓▓▓▓▓

   =============================================
   A Command Line Interface for KubeBlocks

   Available Commands:
     addon               Addon command.
     alert               Manage alert receiver, include add, list and delete receiver.
     backup-config       KubeBlocks backup config.
     bench               Run a benchmark.
     cluster             Cluster command.
     clusterdefinition   ClusterDefinition command.
     clusterversion      ClusterVersion command.
     completion          Generate the autocompletion script for the specified shell
     dashboard           List and open the KubeBlocks dashboards.
     kubeblocks          KubeBlocks operation commands.
     playground          Bootstrap a playground KubeBlocks in local host or cloud.
     version             Print the version information, include kubernetes, KubeBlocks and kbcli version.

   Usage:
     kbcli [flags] [options]

   Use "kbcli <command> --help" for more information about a given command.
   Use "kbcli options" for a list of global command-line options (applies to all commands). 
   ```

   </details>

If you have installed `kbcli` before and want to upgrade it, run

```bash
brew upgrade kbcli
```

If you want to install a specified version,

1. Get a full list of versions.

   ```bash
   brew search kbcli
   ```

2. Install a specified version of `kbcli`. For example, install v0.4.0.

   ```bash
   brew install kbcli@0.4.0
   ```

</TabItem>
</TabItem>

<TabItem value="Windows" label="Windows">

The script option installs `kbcli` under the `C:\Program Files\kbcli-windows-amd64` path by default and this path cannot be changed.

If you want to customize the installation path, use the Zip file option.

<TabItem value="Script" label="Script">

1. Run PowerShell as admin and run `Set-ExecutionPolicy Unrestricted` to grant the script execution permission to PowerShell. Enter Y to confirm the execution policy.

2. Install `kbcli` and the script adds an environment variable in your host automatically.

   ```bash
   powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1)))"
   ```

   :::note

   Specify a version by adding the `-v` option. If you do not specify a version, the above command installs the latest release version on your host.

   ```bash
   powershell -Command " & ([scriptblock]::Create((iwr https://www.kubeblocks.io/installer/install_cli.ps1))) -v 0.5.0-beta.1"
   ```

</TabItem>

<TabItem value="Zip" label="Zip">

1. Download a Zip file of `kbcli` that suits your host from [the GitHub repository](https://github.com/apecloud/kubeblocks/releases).
2. Decompress the package under your prefferred path.
3. Add this path to the system environment variable.

   1. Click the Windows icon and click **System**.
   2. Go to **Settings** -> **Related Settings** -> **Advanced system settings**.
   3. On the **Advanced** tab, click **Environment Variables**.
   4. Click **New** to add the path under which you decompress the `kbcli` package in **User variables** or **System variables**.
   5. Click **Apply** and then **OK** to apply the change.
</TabItem>

</TabItem>

## Install KubeBlocks

:::note

For the local environment, it is recommended to run `kbcli playground init` to install KubeBlocks and create database clusters.

:::

***Steps:***

1. Run the command below to install KubeBlocks. Both `kbcli` and Helm installation options are supported.

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli kubeblocks install
    ```

    ***Result***

    * KubeBlocks is installed with built-in toleration which tolerates the node with the `kb-controller=true:NoSchedule` taint.
    * KubeBlocks is installed with built-in node affinity which first deploys the node with the `kb-controller:true` label.
    * This command installs the latest release version in your Kubernetes environment under the default namespace `kb-system` since your `kubectl` can connect to your Kubernetes clusters. If you want to install KubeBlocks in a specified namespace, run the command below.

       ```bash
       kbcli kubeblocks install -n <name> --create-namespace=true
       ```

       ***Example***

       ```bash
       kbcli kubeblocks install -n kubeblocks --create-namespace=true
       ```

    You can also run the command below to check the parameters that can be specified during installation.

    ```bash
    kbcli kubeblocks install --help
    ```

   * `-namespace` and its abbreviated version `-n` is used to name a namespace. `--create-namespace` is used to specify whether to create a namespace if it does not exist. `-n` is a global command line option. For global command line options, run `kbcli options` to list all options (applies to all commands).
   * Use `monitor` to specify whether to install the add-ons relevant to database monitoring and visualization.
   * Use `version` to specify the version you want to install. Find the supported version in [KubeBlocks Helm Charts](https://github.com/apecloud/helm-charts).

   </TabItem>

   <TabItem value="Helm" label="Helm">

   ```bash
   helm repo add kubeblocks  https://apecloud.github.io/helm-charts

   helm install kubeblocks kubeblocks/kubeblocks -n kb-system --create-namespace
   ```

   </TabItem>

2. Run the command below to verify whether KubeBlocks is installed successfully.

    ```bash
    kubectl get pod -n <namespace>
    ```

    ***Example***

    ```bash
    kubectl get pod -n kubeblocks
    ```

    ***Result***

    When the following pods are `Running`, it means KubeBlocks is installed successfully.

    ```bash
    NAME                                                     READY   STATUS      RESTARTS   AGE
    kb-addon-alertmanager-webhook-adaptor-5549f94599-fsnmc   2/2     Running     0          84s
    kb-addon-grafana-5ddcd7758f-x4t5g                        3/3     Running     0          84s
    kb-addon-prometheus-alertmanager-0                       2/2     Running     0          84s
    kb-addon-prometheus-server-0                             2/2     Running     0          84s
    kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s
    ```

### Handle an exception

If installing KubeBlocks by `kbcli` fails, run preflight checks to find whether your environment meets the requirements of running KubeBlocks and clusters.

```bash
kbcli kubeblocks preflight
```

Add the `--verbose` sub-command to output the details of the preflight checks.

```bash
kbcli kubeblocks preflight --verbose
```

***Result***

There are three types of results:

* `warn`: The target environment affects the stability and performance of KubeBlocks and clusters, but running KubeBlocks and clusters is not affected, and you can continue the following installation.
* `fail`: The environment requirements for installing KubeBlocks are not met, and KubeBlocks can only be installed after these requirements are met. It is required to check these items again and re-run the preflight checks.
* `congratulation`: All checks pass and you can continue the following installation.

## Enable add-ons

An add-on provides extension capabilities, i.e., manifests or application software, to the KubeBlocks control plane.

By default, all add-ons supported are automatically installed.

To list supported add-ons, run `kbcli addon list` command.

**Example**

```bash
kbcli addon list
```

:::note

Some add-ons have an environment requirement. If a certain requirement is not met, the automatic installation is invalid.

:::

You can perform the following steps to check and enable the add-on.

***Steps:***

1. Check the *Installable* part in the output information.
  
    **Example**

    ```bash
    kbcli addon describe snapshot-controller
    ```

    For certain add-ons, the installable part might say when the kubeGitVersion content includes *eks* and *ack*, the auto-install is enabled.

    In this case, you can check the version of the Kubernetes cluster, and run the following command.

    ```bash
    kubectl version -ojson | jq '.serverVersion.gitVersion'
    >
    "v1.24.4+eks"
    >
    ```

    As the printed output suggested, *eks* is included. And you can go on with the next step. In case that *eks* is not included, it is invalid to enable the add-on.

2. To enable the add-on, use `kbcli addon enable`.

    **Example**

    ```bash
    kbcli addon enable snapshot-controller
    ```

3. List the add-ons again to check whether it is enabled.

    ```bash
    kbcli addon list
    ```

## (Optional) Enable kbcli automatic command line completion

`kbcli` supports automatic command line completion. You can run the command below to enable this function.

```bash
# Configure SHELL-TYPE as one type from bash, fish, PowerShell, and zsh
kbcli completion SHELL-TYPE -h
```

Here we take zsh as an example.

***Steps:***

1. Run the command below.

    ```bash
    kbcli completion zsh -h
    ```

2. Enable the completion function of your terminal first.

    ```bash
    echo "autoload -U compinit; compinit" >> ~/.zshrc
    ```

3. Run the command below to enable the `kbcli` automatic completion function.

    ```bash
    echo "source <(kbcli completion zsh); compdef _kbcli kbcli" >> ~/.zshrc
    ```

## Uninstall KubeBlocks and kbcli

:::note

Uninstall KubeBlocks first.

:::

### Uninstall KubeBlocks

Uninstall KubeBlocks if you want to delete KubeBlocks after your trial.

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli kubeblocks uninstall
```

</TabItem>

<TabItem value="helm" label="helm" default>

```bash
helm uninstall kubeblocks -n kb-system
```

</TabItem>

### Uninstall kbcli

Uninstall `kbcli` if you want to delete KubeBlocks after your trial.

<TabItem value="macOS" label="macOS" default>

<TabItem value="cURL" label="cURL" default>

```bash
sudo rm /usr/local/bin/kbcli
```

</TabItem>

<TabItem value="Homebrew" label="Homebrew">

```bash
brew uninstall kbcli
```

</TabItem>

</TabItem>

<TabItem value="Windows" label="Windows">

1. Go to the `kbcli` installation path and delete the installation folder.
   * If you install `kbcli` by script, go to `C:\Program Files` and delete the `kbcli-windows-amd64` folder.
   * If you customize the installation path, go to your path and delete the installation folder.

2. Delete the environment variable.

   1. Click the Windows icon and click **System**.
   2. Go to **Settings** -> **Related Settings** -> **Advanced system settings**.
   3. On the **Advanced** tab, click **Environment Variables**.
   4. Double-click **Path** in **User variables** or **System variables** list.
      * If you install `kbcli` by script, double-click **Path** in **User variables**.
      * If you customize the installation path, double-click **Path** based on where you created the variable before.
   5. Select `C:\Program Files\kbcli-windows-amd64` or your customized path and delete it. This operation requires double confirmation.

</TabItem>

</Tabs>