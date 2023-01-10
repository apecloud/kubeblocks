# Quick start with KuebBlocks

This guide shows you the quickest way to get started with KubeBlocks with the KubeBlocks CLI tool `kbcli` to create a single-node ApeCloud MySQL cluster, and how to connect to, view and delete this cluster in a playground.

## Before you start

Install all the followings:

- MySQL Shell
  
    Install MySQL Shell in your local host to visit MySQL instances. Refer to [Install MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for details.

- `kubectl`
    Run the following command to install `kubectl` in your local host for visiting Kubernetes clusters. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.

      ```
      curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
      ```

- Kubernetes cluster
  
    `kbcli` visits a Kubernetes cluster which can be specified by the `kubeconfig` condition variable or the `kubeconfig` parameter. If no Kubernetes cluster is specified, `kbcli` reads the content in `~/.kube/config` file by default.

## Step 1. Install `kbcli`

1. Run the command below to install `kbcli`.

    ```
    curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_cli.sh |bash
    ```

2. Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.

## Step 2. Install KubeBlocks

Run the command below to install KubeBlocks.

```
kbcli kubeblocks install
```

> Note:
> You can use `--version` to specify the version of KubeBlocks. Fins the latest version of KubeBlocks on the [Release](https://github.com/apecloud/kubeblocks/releases) page.


## Step 3. Create a single-node MySQL cluster

Use `kbcli` and a YAML file to create a cluster with specified component specifications.

1. Create a component configuration file, named as `mycluster.yaml`. The specifications you can refer to are as follows. 
   ```
   - name: ac-mysql
     type: replicasets
     replicas: 1
     volumeClaimTemplates:
     - name: data
       spec:
         accessModes:
           - ReadWriteOnce
         resources:
           requests:
             storage: 1Gi
    ```

2. Run the command below to create a cluster.
   ```
   kbcli cluster create ac-cluster --cluster-definition=ac-mysql  --cluster-version=wesql-8.0.30 --components=mycluster.yaml --termination-policy=WipeOut
   ```

## Step 4. View the cluster details

After the cluster is well-created, you can view its information and status.

1. Run the command below to view all the clusters.
   ```
   kbcli cluster list
   ```

2. Run the command below to view the detailed information of the specified cluster.
   ```
   kbcli cluster describe ac-cluster
   ```

## Step 5. Connect to the cluster

1. Run this command to connect to this cluster.
   ```
   kbcli cluster connect ac-cluster
   ```

2. Choose a language or MySQL client to view the cluster connection information.


## Step 6. (Optional) Delete the cluster (termination policy)

Run this command to delete the created cluster.
```
kbcli cluster delete ac-cluster
```


## Next steps

* [Learn KubeBlocks](Introduction/introduction.md)
* [Install KubeBlocks](installation/install_kubeblocks.md)

## More details

This guide gives you a quick tour of how to create a cluster with KubeBlocks. For more detailed information on how to use KubeBlocks to create your cluster with other options, see the following guides:

- [Create and manage a cluster](installation/create_and_manege_a_cluster.md)
- [Lifecycle management](lifecycle_management/lifecycle_management_api.md)