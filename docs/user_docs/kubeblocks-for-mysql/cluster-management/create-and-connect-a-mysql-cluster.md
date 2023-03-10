---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
sidebar_position: 1
---

# Create and connect to a MySQL Cluster
## Create a MySQL Cluster

***Before you start***

* `kbcli`: Install `kbcli` on your host. Refer to [Install/Uninstall kbcli and KubeBlocks](../../install_kbcli_kubeblocks/install_and_unistall_kbcli_and_kubeblocks.md) for details.
  1. Run the command below to install `kbcli`.
   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```
  2. Run the command below to check the version and verify whether `kbcli` is installed successfully.
   ```bash
   kbcli version
   ```
* KubeBlocks: Install KubeBlocks on your host. Refer to [Install/Uninstall kbcli and KubeBlocks](../../install_kbcli_kubeblocks/install_and_unistall_kbcli_and_kubeblocks.md) for details.
  1. Run the command below to install KubeBlocks.
     ```bash
     kbcli kubeblocks install
     ```
     > ***Note:***
     > 
     > If you want to specify a namespace for KubeBlocks, use `--namespace` or the abbreviated `-n` to name your namespace and configure `--create-namespace` as `true` to create a namespace if it does not exist. For example,
     > ```bash
     > kbcli kubeblocks install -n kubeblocks --create-namespace=true
     > ```
  2. Run the command below to verify whether KubeBlocks is installed successfully
     ```bash
     kubectl get pod
     ```

     ***Result***

     Four pods starting with `kubeblocks` are displayed. For example,
     ```
     NAME                                                  READY   STATUS    RESTARTS   AGE
     kubeblocks-7d4c6fd684-9hjh7                           1/1     Running   0          3m33s
     kubeblocks-grafana-b765d544f-wj6c6                    3/3     Running   0          3m33s
     kubeblocks-prometheus-alertmanager-7c558865f5-hsfn5   2/2     Running   0          3m33s
     kubeblocks-prometheus-server-5c89c8bc89-mwrx7         2/2     Running   0          3m33s
     ```
* Run the command below to view all the database types available for creating a cluster. 
  ```bash
  kbcli clusterdefinition list
  ```

  ***Result***

  ```bash
  kbcli clusterdefinition list
  >
  NAME             MAIN-COMPONENT-TYPE   STATUS      AGE
  apecloud-mysql   mysql                 Available   7m52s
  ```

***Steps:***

1. Run the command below to list all the available kernel versions and choose the one that you need.
   ```bash
   kbcli clusterversion list
   ```

   ***Result***

   ```bash
   kbcli clusterversion list
   >
   NAME              CLUSTER-DEFINITION   STATUS      AGE
   ac-mysql-8.0.30   apecloud-mysql       Available   2m40s
   ```
2. Run the command below to create a MySQL cluster.
   ```bash
   kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
   ```
   ***Result***

   * A cluster then is created in the default namespace. You can specify a namespace for your cluster by using `--namespace` or the abbreviated `-n` option. For example,

     ```bash
     kubectl create namespace demo

     kbcli cluster create -n demo --cluster-definition='apecloud-mysql'
     ```
   * A cluster is created with built-in toleration which tolerates the node with the kb-data=true:NoSchedule taint.
   * A cluster is created with built-in node affinity which first deploys the node with the kb-data:true label.
  
   To create a cluster with specified parameters, follow the steps below, and you have three options.

   **Option 1.** (Recommended) Use `--set` option
   
    Add the --set option when creating a cluster. For example,
    ```bash
    kbcli cluster create mysql-cluster --cluster-definition apecloud-mysql --set cpu=1000m,memory=1Gi,storage=10Gi,replicas=3
    ```

   **Option 2.** Run `export`

   If you want to create a Paxos group, run `export KBCLI_CLUSTER_DEFAULT_REPLICAS=3` before creating a cluster. For example,
   ```bash
   export KBCLI_CLUSTER_DEFAULT_REPLICAS=3
   kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
   ```

   If you want to adjust the storage size, run `export KBCLI_CLUSTER_DEFAULT_STORAGE_SIZE=nGi` before creating a cluster. For example,
  
   ```bash
   export KBCLI_CLUSTER_DEFAULT_STORAGE_SIZE=20Gi
   kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
   ```

   **Option 2.** Change YAML file configurations

   Change the corresponding parameters in the YAML file.
   ```bash
   kbcli cluster create mysql-cluster --cluster-definition="apecloud-mysql" --set -<<EOF
   - name: mysql
     replicas: 3
     type: mysql
     volumeClaimTemplates:
     - name: data
       spec:
         accessModes:
         - ReadWriteOnce
         resources:
           requests:
             storage: 20Gi
   EOF
   ```

**`kbcli cluster create` options description**

| Option   | Description      |
| :--      | :--              |
| `--cluster-definition` | It specifies the cluster definition. Run `kbcli cd list` to show all available cluster definitions. |
| `--cluster-version` | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is used by default. |
| `--enable-all-logs` | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. This option is set as true by default. |
| `--help` | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`. |
| `--monitor` | It is used to enable the monitor function and inject metrics exporter. It is set as true by default. |
| `--node-labels` | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />```kbcli cluster create --cluster-definition='apecloud-mysql' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'``` |
| `--set` | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,replicas=3,storage=10Gi`. |
| `--termination-policy` | It specifies the termination policy of the cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a MySQL Cluster

Run the command below to connect to a cluster.
```bash
kbcli cluster connect mysql-cluster
```

***Example***

```bash
kbcli cluster connect mysql-cluster
>
Connect to instance mysql-cluster-mysql-0: out of mysql-cluster-mysql-0(leader), mysql-cluster-ac-mysql-1(follower), mysql-cluster-ac-mysql-2(follower)
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 16
Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision d6b8719

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>
```