---
title: Install kbcli and KubeBlocks on new Kubernetes cluster
description: Install kbcli and KubeBlocks on new Kubernetes cluster, the environment is clean
keywords: [playground, install, KubeBlocks]
sidebar_position: 2
sidebar_label: Install kbcli and KubeBlocks on new Kubernetes cluster
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install kbcli and KubeBlocks on new Kubernetes cluster

To install kbcli and KubeBlocks to an new Kubernetes cluster on both local and cloud environment, you can use Playground. To install kbcli and KubeBlocks on existed Kubernetes cluster, see [Install kbcli and KubeBlocks on the existed Kubernetes cluster](./install-kbcli-and-kubeblocks-on-the-existed-kubernetes-clusters.md).

## Install kbcli and KubeBlocks on local host

### Environment preparation

- Minimum system requirements:
  - MacOS:
    - CPU: 4 cores
    - Memory: 4 GB
    Check CPU with the following command: `sysctl hw.physicalcpu; Check memory with the following command: top -d`.
  - Windows:
    - 64-bit
- Ensure the following tools are installed on your local host:
  - Docker: v20.10.5 (runc ≥ v1.0.0-rc93) or higher. For installation details, see Get Docker.
  - kubectl: used to interact with Kubernetes clusters.
  - For Windows environment, PowerShell version 5.0 or higher is required.

### Installation steps

You can install the kbcli and KubeBlocks on your local host, and now MacOS and Windows are supported.

**Step 1. Install kbcli.**

<Tabs>
<TabItem value="MacOS" label="MacOS">

- Option 1: Install kbcli using the `CurL` command.

1. Install kbcli.

   ```
   curl -fsSL [https://www.kubeblocks.io/installer/install_cli.sh] | bash
   ```

   To customize the version of kbcli, use the following command.

   ```
   curl -fsSL [https://kubeblocks.io/installer/install_cli.sh] |bash -s versionnumber
   ```

2. Run `kbcli version` to check the version of kbcli and ensure that it is successfully installed.

:::note

- If a timeout exception occurs during installation, please retry and check your network settings.

:::

- Option 2: Install kbcli using `Homebrew`.

    1. Install ApeCloud tap, the Homebrew package of ApeCloud.

        ```
        brew tap apecloud/tap
        ```

    2. Install kbcli.

        ```
        brew install kbcli
        ```

        To install a custom version of kbcli.

        ```
        brew install kbcli@0.4.0
        ```

    3. Verify that kbcli is successfully installed.

        ```
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

    ```
    powershell -Command " & ([scriptblock]::Create((iwr [https://www.kubeblocks.io/installer/install_cli.ps1])))"
    ```

    To install a specified version of kbcli, use -v after the command and describe the version you want to install.

    ```
    powershell -Command " & ([scriptblock]::Create((iwr <https://www.kubeblocks.io/installer/install_cli.ps1>))) -v 0.5.0-beta.1"
    ```

- Option 2: Install using the installation package.

1. Download the kbcli installation zip package from GitHub.
2. Extract the file and add it to the environment variables.
    1. Click the Windows icon and select System Settings.
    2. Click Settings -> Related Settings -> Advanced system settings.
    3. Click Environment Variables on the Advanced tab.
    4. Click New to add the path of the kbcli installation package to the user and system variables.
    5. Click Apply and OK.

</TabItem>
</Tabs>

**Step 2: One-click Deployment of KubeBlocks**
Use the `kbcli playground init` command. This command:
    - Creates a Kubernetes cluster in a K3d container.
    - Deploys KubeBlocks in the Kubernetes cluster.
    - Creates a high-availability ApeCloud MySQL cluster named mycluster in the default namespace.
Check the created cluster. When the status is Running, it indicates that the cluster has been successfully created.
    ```
    kbcli cluster list
    ```

## Install kbcli and KubeBlocks on cloud

This section shows how to install kbcli and KubeBlocks on new Kubernetes clusters on cloud.
When deploying to the cloud, you can use the Terraform scripts maintained by ApeCloud to initialize the cloud resources. Click on [Terraform script](https://github.com/apecloud/cloud-provider) to use Terraform.

When deploying a Kubernetes cluster in the cloud, kbcli clones the above repository to the local host, invokes the Terraform command to initialize the cluster, and then deploys KubeBlocks on that cluster.

**Step 1. Install kbcli.**

Use the following command.

   ```
   curl -fsSL [https://www.kubeblocks.io/installer/install_cli.sh] | bash
   ```

   To customize the version of kbcli, use the following command.

   ```
   curl -fsSL [https://kubeblocks.io/installer/install_cli.sh] |bash -s versionnumber
   ```
2. Configure and connect to cloud environment. See the table below.

| Cloud Environment                 | Commands                                                                                                                                                                |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| EKS v1.22 / v1.23 / v1.24 / v1.25 | $ export AWS_ACCESS_KEY_ID="anaccesskey"  $ export  AWS_SECRET_ACCESS_KEY="asecretkey"  kbcli playground init  --cloud-provider aws --region regionname                 |
| ACK v1.22 / v1.24                 | export ALICLOUD_ACCESS_KEY="************"  export ALICLOUD_SECRET_KEY="************"  kbcli playground init --cloud-provider alicloud --region regionname               |
| TKE v1.22 / v1.24                 | export TENCENTCLOUD_SECRET_ID=YOUR_SECRET_ID  export  TENCENTCLOUD_SECRET_KEY=YOUR_SECRET_KEY  kbcli playground init  --cloud-provider tencentcloud --region regionname |
| GKE v1.24 / v1.25                 | gcloud init  gcloud auth application-default login   export GOOGLE_PROJECT= <project name> kbcli playground init --cloud-provider gcp  --region regionname              |

**Step 2: One-click Deployment of KubeBlocks**

Use the `kbcli playground init` command. This command:
    - Creates a Kubernetes cluster in a K3d container.
    - Deploys KubeBlocks in the Kubernetes cluster.
    - Creates a high-availability ApeCloud MySQL cluster named mycluster in the default namespace.
Check the created cluster. When the status is Running, it indicates that the cluster has been successfully created.
    ```
    kbcli cluster list
    ```
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