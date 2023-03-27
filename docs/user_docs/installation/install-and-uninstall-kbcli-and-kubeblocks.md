---
title: Install and uninstall kbcli and KubeBlocks
description: Install KubeBlocks and kbcli developed by ApeCloud
sidebar_position: 1
sidebar_label: kbcli and KubeBlocks
---

# Install and uninstall kbcli and KubeBlocks

This guide introduces how to install KubeBlocks by `kbcli`, the command line tool of KubeBlocks.

## Before you start

1. A Kubernetes environment is required.
2. `kubectl` is required and can connect to your Kubernetes clusters. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for installation details.
   
## Install kbcli

1. Run the command below to install `kbcli`. `kbcli` can run on macOS and Linux.
    ```bash
    curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
    ```

    :::note

    Please try again if a time-out exception occurs during installation. It may relate to your network condition.

    :::

2. Run this command to check the version and verify whether `kbcli` is installed successfully.
    ```bash
    kbcli version
    ```

## Install KubeBlocks

1. Run the command below to install KubeBlocks.
    ```bash
    kbcli kubeblocks install
    ```
    ***Result***

    * KubeBlocks is installed with built-in toleration which tolerates the node with the `kb-controller=true:NoSchedule` taint.
    * KubeBlocks is installed with built-in node affinity which first deploys the node with the `kb-controller:true` label.
    * This command installs the latest version in your Kubernetes environment under the default namespace `kb-system` since your `kubectl` can connect to your Kubernetes clusters. If you want to install KubeBlocks in a specified namespace, run the command below.
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
    >
    Install KubeBlocks

    Examples:
      # Install KubeBlocks
      kbcli kubeblocks install

      # Install KubeBlocks with specified version
      kbcli kubeblocks install --version=0.4.0

      # Install KubeBlocks with other settings, for example, set replicaCount to 3
      kbcli kubeblocks install --set replicaCount=3

    Options:
       --check=true:
	        Check kubernetes environment before install

       --create-namespace=false:
	        Create the namespace if not present

       --monitor=true:
	        Set monitor enabled and install Prometheus, AlertManager and Grafana (default true)

       --set=[]:
	        Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)

       --set-file=[]:
	        Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)

       --set-json=[]:
	        Set JSON values on the command line (can specify multiple or separate values with commas:
	        key1=jsonval1,key2=jsonval2)

       --set-string=[]:
	        Set STRING values on the command line (can specify multiple or separate values with commas:
	        key1=val1,key2=val2)

       --timeout=30m0s:
	        Time to wait for installing KubeBlocks

       -f, --values=[]:
	        Specify values in a YAML file or a URL (can specify multiple)

       --version='0.4.0-beta.5':
	        KubeBlocks version

    Usage:
       kbcli kubeblocks install [flags] [options]

    Use "kbcli options" for a list of global command-line options (applies to all commands).
    ```
   
   * `-namespace` and its abbreviated version `-n` is used to name a namespace. `--create-namespace` is used to specify whether to create a namespace if it does not exist. `-n` is a global command line option. For global command line options, run `kbcli options` to list all options (applies to all commands).
   * Use `monitor` to specify whether to install the add-ons relevant to database monitoring and visualization.
   * Use `version` to specify the version you want to install. Find the supported version in [KubeBlocks Helm Charts](https://github.com/apecloud/helm-charts).

2. Run the command below to verify whether KubeBlocks is installed successfully.
    ```bash
    kubectl get pod -n <namespace>
    ```

    ***Example***

    ```bash
    kubectl get pod -n kb-system
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

## (Optional) Enable kbcli automatic command line completion

`kbcli` supports automatic command line completion. You can run the command below to enable this function.

```bash
# Configure SHELL-TYPE as one type from bash, fish, PowerShell, and zsh
kbcli completion SHELL-TYPE -h
```

Here we take zsh as an example.

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

Run the command below to uninstall KubeBlocks if you want to delete KubeBlocks after your trial.
   ```bash
   kbcli kubeblocks uninstall
   ```

Run the command below to uninstall `kbcli`.
   ```bash
   sudo rm /usr/local/bin/kbcli
   ```