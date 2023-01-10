# Quick start with KuebBlocks

This guide shows you the quickest way to get started with KubeBlocks with the KubeBlocks CLI tool kbcli to create a single-node ApeCloud MySQL cluster, and how to connect to, view and delete this cluster in a playground.

## Preparation

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

Choose one of the methods to install `kbcli`.

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
  <TabItem value="curl" label="make" default>
    curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_cli.sh |bash
  </TabItem>
  <TabItem value="orange" label="Orange">
    # Switch to `main` branch
        git checkout main
        git pull

    # Make `kbcli`
        GIT_VERSION=`git describe --always --abbrev=0 --tag`
        VERSION=`echo "${GIT_VERSION/v/}"`
        make kbcli
  </TabItem>
</Tabs>

Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.

When the status is `Running`, the instance is deployed successfully.

## Step 2. Create a single-node MySQL cluster

Use `kbcli` and a YAML file to create a cluster with specified component specifications.

1. Create a component configuration file, named as `mycluster.yaml`. The specifications you can refer to are as follows. You can find this example file in the GitHub repository.
   ```
   - name: apecloud-mysql
        type: replicasets
        monitor: false
        volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem
    ```

2. Run the command below to create a cluster.
   ```
   kbcli cluster create apecloud-cluster --cluster-definition=apecloud-wesql  --cluster-version=wesql-8.0.30 --components=mycluster.yaml --termination-policy=Halt
   ```

## Step 3. View the cluster details

After the cluster is well-created, you can view its information and status.

1. Run the command below to view all the clusters.
   ```
   kbcli cluster list
   ```

2. Run the command below to view the detailed information of the specified cluster.
   ```
   kbcli cluster describe apecloud-cluster
   ```

## Step 4. Connect to the cluster

1. Run this command to connect to this cluster.
   ```
   kbcli cluster connect
   ```

2. Choose a language or MySQL client to view the cluster connection information.


## Step 5. (Optional) Delete the cluster (termination policy)

   Run this command to delete the created cluster.
   ```
   kbcli cluster delete apecloud-cluster
   ```


## Next steps

* Learn KubeBlocks [links to be completed]
* [Install KubeBlocks](installation/install_kubeblocks.md)

## More details

This guide gives you a quick tour of how to create a cluster with KubeBlocks. For more detailed information on how to use KubeBlocks to create your cluster with other options, see the following guides:

- [Create and manage a cluster](installation/create_and_manege_a_cluster.md)
- [Lifecycle management](lifecycle_management/lifecycle_management_api.md)