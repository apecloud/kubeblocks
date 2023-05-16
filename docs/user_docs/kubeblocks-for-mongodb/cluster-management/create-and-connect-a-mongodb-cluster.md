---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [mogodb, create a mongodb cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MongoDB cluster

This document shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install `kbcli`](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) .
* [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md).
* Make sure MongoDB addon is installed with `kbcli addon list`.

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  mongodb                        Helm   Enabled                   true
  ...
  ```

### Create a MongoDB cluster with default settings

***Steps***

1. Create a MongoDB cluster with default settings.

   ```bash
   kbcli cluster create mongodb-cluster --cluster-definition mongodb --termination-policy WipeOut
   ```

   * The cluster is created with built-in toleration which tolerates the node with the `kb-data=true:NoSchedule` taint.
   * The cluster created has built-in node affinity and is first deployed on the node with the `kb-data:true` label.
   * For configuring pod affinity for a cluster, refer to [Configure pod affinity for database cluster](../../resource-scheduling/resource-scheduling.md).

2. Check the cluster status by executing `kbcli cluster describe`.

   ```bash
   kbcli cluster describe mongodb-cluster
   ```

### (Recommended) Create a cluster with specified class type

**Option 1.** (**Recommended**) Use `--set` option

Add the `--set` option when creating a cluster. For example,

```bash
kbcli cluster create mongodb-cluster --cluster-definition mongodb --set cpu=1,memory=1Gi,storage=10Gi
```

**Option 2.** Change YAML file configurations

Change the corresponding parameters in the YAML file.

```bash
kbcli cluster create mongodb-cluster --cluster-definition="mongodb" --set-file -<<EOF
- name: mongodb-cluster
  replicas: 2
  componentDefRef: mongodb
  volumeClaimTemplates:
  - name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
EOF
```

📎 Table 1. kbcli cluster create options description

| Option                 | Description             |
|:-----------------------|:------------------------|
| `--cluster-definition` | It specifies the cluster definition, choose the database type. Run `kbcli cd list` to show all available cluster definitions.   |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is applied by default.  |
| `--enable-all-logs`    | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. For logs settings, refer to [Access Logs](./../../observability/access-logs.md)  |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`. |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default. |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />`kbcli cluster create --cluster-definition='apecloud-mysql' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'`  |
| `--set`                | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,replicas=3,storage=10Gi`.  |
| `--set-file`           | It uses a yaml file, URL, or stdin to set the cluster resource. |
| `--termination-policy` | It specifies how a cluster is deleted. Set the policy when creating a cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a MongoDB Cluster

```bash
kbcli cluster connect mongodb-cluster
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
