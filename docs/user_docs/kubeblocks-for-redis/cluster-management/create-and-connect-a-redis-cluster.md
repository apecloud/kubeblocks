---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [redis, create, connect, cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and Connect to a Redis cluster

KuebBlocks for Redis supports standalone clusters and primary-secondary clusters.

## Create a Redis Cluster

### Before you start

* `kbcli`: Install `kbcli` on your host. Refer to [Install/Uninstall kbcli and KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) for details.
* KubeBlocks: Install KubeBlocks on your host. Refer to [Install/Uninstall kbcli and KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) for details.
* Run the command below to view all the database types available for creating a cluster. 
  ```bash
  kbcli clusterdefinition list
  ```

### Steps

1. Run the command below to list all the available kernel versions and choose the one that you need.
   ```bash
   kbcli clusterversion list
   ```

2. Run the command below to create a Redis cluster.
   ```bash
   kbcli cluster create redis-cluster --cluster-definition='redis'
   ```
   ***Result***

   * A cluster then is created in the default namespace. You can specify a namespace for your cluster by using `--namespace` or the abbreviated `-n` option. For example,

     ```bash
     kubectl create namespace demo

     kbcli cluster create -n demo --cluster-definition='redis'
     ```
   * A cluster is created with built-in toleration which tolerates the node with the `kb-data=true:NoSchedule` taint.
   * A cluster is created with built-in node affinity which first deploys the node with the `kb-data:true` label.
   * For configuring pod affinity for a cluster, refer to [Configure pod affinity for database cluster](../../resource-scheduling/resource-scheduling.md).
  
   To create a cluster with specified parameters, follow the steps below, and you have three options.

   **Option 1.** (**Recommended**) Use --set option
   
    Add the `--set` option when creating a cluster. For example,
    ```bash
    kbcli cluster create redis-cluster --cluster-definition redis --set cpu=1,memory=1Gi,storage=10Gi,replicas=3
    ```

   **Option 2.** Change YAML file configurations

   Change the corresponding parameters in the YAML file.
   ```bash
   kbcli cluster create redis-cluster --cluster-definition="redis" --set -<<EOF
   - name: redis
     replicas: 3
     type: redis
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

### kbcli cluster create options description

| Option   | Description      |
| :--      | :--              |
| `--cluster-definition` | It specifies the cluster definition. Run `kbcli cd list` to show all available cluster definitions. |
| `--cluster-version` | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is used by default. |
| `--enable-all-logs` | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. This option is set as true by default. |
| `--help` | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`. |
| `--monitor` | It is used to enable the monitor function and inject metrics exporter. It is set as true by default. |
| `--node-labels` | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />```kbcli cluster create --cluster-definition='redis' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'``` |
| `--set` | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1,memory=1Gi,replicas=3,storage=10Gi`. |
| `--termination-policy` | It specifies the termination policy of the cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a Redis Cluster

Run the command below to connect to a cluster. For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
```bash
kbcli cluster connect redis-cluster
```