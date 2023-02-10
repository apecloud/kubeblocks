# Install/Uninstall `kbcli` and KubeBlocks

This guide introduces how to install KubeBlocks by `kbcli`, the command line tool of KubeBlocks.

## Before you start

1. A Kubernetes environment is required.
2. If you need to visit the MySQL cluster created by KubeBlocks via `kbcli`, MySQL client is required. `kbcli` communicates with the created MySQL cluster by calling MySQL client. Refer to [Installing MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for installation details.
3. `kubectl` is required and can connect to your Kubernetes clusters. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for installation details.
   
## Step 1. Install KubeBlocks by `kbcli`

1. Run the command below to install `kbcli`. `kbcli` can run on macOS and Linux.
   ```
   curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_cli.sh |bash
   ```

   > Note:
   > 
   > Please try again if a time-out exception occurs during installation. It may relate to your network condition.
2. Run this command to check the version and verify whether kbcli is installed successfully.
   ```
   kbcli version
   ```
3. Run the command below to uninstall kbcli after your trial.
   ```
   sudo rm /usr/local/bin/kbcli
   ```

## Step 2. Enable `kbcli` automatic command line completion

`kbcli` supports automatic command line completion. You can run the command below to enable this function.

```
# Configure SHELL-TYPE as one type from bash, fish, PowerShell, and zsh
kbcli completion SHELL-TYPE -h
```

Here we take zsh as an example.

1. Run the command below.
```
kbcli completion zsh -h
```
2. Enable the completion function of your terminal first.
```
$ echo "autoload -U compinit; compinit" >> ~/.zshrc
```
3. Run the command below to enable the kbcli automatic completion function.
```
$ echo "source <(kbcli completion zsh); compdef _kbcli kbcli" >> ~/.zshrc
```

## Step 3. Install KubeBlocks

1. Run the command below to install KubeBlocks.
   ```
   kbcli kubeblocks install
   ```
   ***Result***
   
   This command installs the latest version in your Kubernetes environment since your `kubectl` can connect to your Kubernetes clusters.
   You can also run the command below to check the parameters that can be specified during installation.
   ```
   $ kbcli kubeblocks install --help

   Install KubeBlocks

   Examples:
     # Install KubeBlocks
    kbcli kubeblocks install
  
     # Install KubeBlocks with specified version
     kbcli kubeblocks install --version=0.2.0
  
     # Install KubeBlocks with other settings, for example, set replicaCount to 3
     kbcli kubeblocks install --set replicaCount=3

   Options:
       --create-namespace=false:
           create the namespace if not present

       --monitor=true:
           Set monitor enabled and install Prometheus, AlertManager and Grafana (default true)

       --set=[]:
           Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)

       --version='0.3.0-alpha.6':
           KubeBlocks version

   Usage:
     kbcli kubeblocks install [flags] [options]

   Use "kbcli options" for a list of global command-line options (applies to all commands).
   ```

   | **Option**       | **Usage**         |
   | :--              | :--               |
   | `create-namespace` | Use `create-namespace` to specify whether to create a namespace.|
   | `monitor`      | Use `monitor` to specify whether to install the addons relevant to database monitoring and visualization.|
   | `version`      | User `version` to specify the version you want to install. Find the supported version in [KubeBlocks Helm Charts](https://github.com/apecloud/helm-charts).|
   | `set snapshot-controller.enabled=true` | When this parameter is set as `true`, the snapshot backup function of the database instance is enabled (only applied to the EKS environment). Refer to [Backup and restore for MySQL single node](../manage_mysql_database_with_kubeblocks/backup_restore/backup_and_restore_for_MySQL_standalone.md) for details.|
   | `set loadbalancer.enabled=true` | When this parameter is set as `true`, the loadbalancer function is enabled (only applied to the EKS environment). This function provides a stable virtual IP address externally to facilitate client access within the same VPC but outside the Kubernetes cluster.|
2. Run the command below to verify whether KubeBlocks is installed successfully.
   ```
   kubectl get pod
   ```
   ***Result***<br>
   Four pods starting with `kubeblocks` are displayed. For example,
   ```
   NAME                                                  READY   STATUS    RESTARTS   AGE
   kubeblocks-7d4c6fd684-9hjh7                           1/1     Running   0          3m33s
   kubeblocks-grafana-b765d544f-wj6c6                    3/3     Running   0          3m33s
   kubeblocks-prometheus-alertmanager-7c558865f5-hsfn5   2/2     Running   0          3m33s
   kubeblocks-prometheus-server-5c89c8bc89-mwrx7         2/2     Running   0          3m33s
3. Run the command below to uninstall KubeBlocks if you want to delete KubeBlocks after your trial.
   ```
   kbcli kubeblocks uninstall
   ```