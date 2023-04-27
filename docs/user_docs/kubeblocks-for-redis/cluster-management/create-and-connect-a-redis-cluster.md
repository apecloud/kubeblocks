---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [redis, create, connect, cluster, redis sentinel]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and Connect to a Redis cluster

KuebBlocks for Redis supports Standalone clusters and PrimarySecondary clusters.

But for your better high-availability experience, KubeBlocks creates a Redis PrimarySecondary by default.

## Create a Redis Cluster

### Before you start

* [Install `kbcli`](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kbcli).
* [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kubeblocks).
* View all the database types available for creating a cluster.

  ```bash
  kbcli clusterdefinition list
  ```

### Steps

Create a Redis cluster.

```bash
kbcli cluster create redis-cluster --cluster-definition redis 
```

***Result***

* A Redis PrimarySecondary is created in the default namespace. You can specify a namespace for your cluster by using `--namespace` or the abbreviated `-n` option. For example,

  ```bash
  kubectl create namespace demo

  kbcli cluster create redis-cluster -n demo --cluster-definition='redis'
  ```

* This cluster is created with built-in toleration which tolerates the node with the `kb-data=true:NoSchedule` taint.
* This cluster is created with built-in node affinity which first deploys the node with the `kb-data:true` label.
* For configuring pod affinity for a cluster, refer to [Configure pod affinity for database cluster](../../resource-scheduling/resource-scheduling.md).

:::note

If you want to create a Redis Standalone, run the command below:

```bash
kbcli cluster create redis-cluster --cluster-definition redis --set replicas=1
```

:::

To create a cluster with specified parameters, follow the steps below, and you have two options.

**Option 1.** (**Recommended**) Use --set option

Add the `--set` option when creating a cluster. Both Redis and Redis-Sentinel components can be set.

```bash
kbcli cluster create redis-cluster --cluster-definition redis --set cpu=200m,memory=500Mi,storage=30Gi,type=redis --set replicas=3,cpu=200m,memory=500Mi,memory=30Gi,type=redis-sentinel
```

:::note

Make sure there are enough CPU and memory resources. 

:::

**Option 2.** Change YAML file configurations

Change the corresponding parameters in the YAML file.

```bash
kbcli cluster create redis-cluster --cluster-definition="redis" --set-file -<<EOF
- name: redis
  replicas: 2
  componentDefRef: redis
  volumeClaimTemplates:
  - name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          cpu: 200m
          memory: 500Mi
          storage: 30Gi
- name: redis-sentinel
  replicas: 3
  componentDefRef: redis-sentinel
  volumeClaimTemplates:
  - name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          cpu: 100m
          memory: 500Mi
          storage: 30Gi
EOF
```

### kbcli cluster create options description

Except for the --set options, there are many options you can create cluster with, see Table 1.

📎 Table 1. kbcli cluster create options description

| Option                 | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|:-----------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--cluster-definition` | It specifies the cluster definition. Run `kbcli cd list` to show all available cluster definitions.                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is used by default.                                                                                                                                                                                                                                                                                                                                                           |
| `--enable-all-logs`    | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. This option is set as false by default.                                                                                                                                                                                                                                                                                                                                                                                               |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default.                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />```kbcli cluster create --cluster-definition='redis' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'```                                                                                                                                                                                                                                                                          |
| `--set`                | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1,memory=1Gi,replicas=1,storage=10Gi`.                                                                                                                                                                                                                                                                                                                                                                                  |
| `--termination-policy` | It specifies the termination policy of the cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a Redis Cluster

Run the command below to connect to a cluster. For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).

```bash
kbcli cluster connect redis-cluster
```
