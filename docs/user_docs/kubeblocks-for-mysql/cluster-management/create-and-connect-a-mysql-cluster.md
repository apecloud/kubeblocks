---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a cluster, connect a cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MySQL cluster

This document shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install `kbcli`](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kbcli).
* [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kubeblocks).
* View all the database types available for creating a cluster.

  ```bash
  kbcli clusterdefinition list
  ```

### Create a MySQL cluster with default settings

***Steps***

 Create a MySQL cluster with default settings.

   ```bash
   kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
   ```

   ***Result***

* A cluster then is created in the default namespace. You can specify a namespace for your cluster by using `--namespace` or the abbreviated `-n` option. For example,

     ```bash
     kubectl create namespace demo

     kbcli cluster create -n demo --cluster-definition='apecloud-mysql'
     ```

* This cluster is created with built-in toleration which tolerates the node with the `kb-data=true:NoSchedule` taint.
* This cluster has built-in node affinity which first deploys the node with the `kb-data:true` label.
* For configuring pod affinity for a cluster, refer to [Configure pod affinity for database cluster](../../resource-scheduling/resource-scheduling.md).
* The CPU and memory set for the cluster are 1C1G.
  
### (Recommended) Create a cluster with specified class type

A class is a set of resource configurations of CPU, memory and storage, to offer convenience and also set a constraints on the resources applied to the cluster. See [Cluster types](./../cluster-type/cluster-types.md).

***Steps:***

1. List all classes with `kbcli class list` command and choose the one you need, or check [class type](./../cluster-type/cluster-types.md) document for reference.

   ```bash
   kbcli class list --cluster-definition apecloud-mysql  
   ```

   :::note
  
   If there is no suitable class listed, you can [customize your own class](./../cluster-type/customize-class-type.md) template and apply the class here.

   Creating clusters that does not meet the constraints is invalid and system creates the cluster with the minimum CPU value specified.

   :::

2. Use `--set` option with `kbcli cluster create` command.

   ```bash
   kbcli cluster create myclsuter --cluster-definition apecloud-mysql --set class=general-2c2g
   ```

Except for the `--set` options, there are many options you can create cluster with, see Table 1.
📎 Table 1. kbcli cluster create options description

| Option                 | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|:-----------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--cluster-definition` | It specifies the cluster definition. Run `kbcli cd list` to show all available cluster definitions.                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is used by default.                                                                                                                                                                                                                                                                                                                                                           |
| `--enable-all-logs`    | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. This option is set as true by default.                                                                                                                                                                                                                                                                                                                                                                                                |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default.                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />```kbcli cluster create --cluster-definition='apecloud-mysql' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'```                                                                                                                                                                                                                                                                 |
| `--set`                | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,replicas=3,storage=10Gi`.                                                                                                                                                                                                                                                                                                                                                                              |
| `--set-file`           | It uses a yaml file, URL, or stdin to set the cluster resource.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `--termination-policy` | It specifies the termination policy of the cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a MySQL Cluster

Run the command below to connect to a cluster. For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).

```bash
kbcli cluster connect mysql-cluster
```
