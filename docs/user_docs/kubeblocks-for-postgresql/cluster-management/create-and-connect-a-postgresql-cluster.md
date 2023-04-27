---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [postgresql, create a cluster, connect a cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a PostgreSQL Cluster

## Create a PostgreSQL Cluster

### Before you start

* [Install `kbcli`](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kbcli).
* [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kubeblocks).
* View all the database types available for creating a cluster.

  ```bash
  kbcli clusterdefinition list
  ```

### Steps

Run the command below to create a PostgreSQL cluster.

```bash
kbcli cluster create pg-cluster --cluster-definition='postgresql'
```

***Result***

* A PostgreSQL PrimarySecondary is created in the default namespace. You can specify a namespace for your cluster by using `--namespace` or the abbreviated `-n` option. For example,

  ```bash
  kubectl create namespace demo

  kbcli cluster create -n demo --cluster-definition='postgresql'
  ```

* This cluster is created with built-in toleration which tolerates the node with the `kb-data=true:NoSchedule` taint.
* This cluster is created with built-in node affinity which first deploys the node with the `kb-data:true` label.
* For configuring pod affinity for a cluster, refer to [Configure pod affinity for database cluster](../../resource-scheduling/resource-scheduling.md).

:::note

If you want to create a PostgreSQL Standalone, run the command below:

```bash
kbcli cluster create pg-cluster --cluster-definition='postgresql' --set replicas=1
```

:::

To create a cluster with specified parameters, follow the steps below, and you have two options.

**Option 1.** (**Recommended**) Use `--set` option

Add the `--set` option when creating a cluster. For example,

```bash
kbcli cluster create pg-cluster --cluster-definition postgresql --set cpu=1000m,memory=1Gi,storage=10Gi
```

**Option 2.** Change YAML file configurations

Change the corresponding parameters in the YAML file.

```bash
kbcli cluster create pg-cluster --cluster-definition="postgresql" --set-file -<<EOF
- name: postgresql
  replicas: 2
  componentDefRef: postgresql
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

### kbcli cluster create options description

| Option                 | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|:-----------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--cluster-definition` | It specifies the cluster definition. Run `kbcli cd list` to show all available cluster definitions.                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is used by default.                                                                                                                                                                                                                                                                                                                                                           |
| `--enable-all-logs`    | It enables you to view all application logs. When this option is enabled, enabledLogs of component level will be ignored. This option is set as false by default.                                                                                                                                                                                                                                                                                                                                                                                               |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default.                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />```kbcli cluster create --cluster-definition='postgresql' --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'```                                                                                                                                                                                                                                                                     |
| `--set`                | It sets the cluster resource including CPU, memory, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,storage=10Gi,replicas=2`.                                                                                                                                                                                                                                                                                                                                                                                        |
| `--set-file`           | It uses a yaml file, URL, or stdin to set the cluster resource.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `--termination-policy` | It specifies the termination policy of the cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

## Connect to a PostgreSQL Cluster

Run the command below to connect to a cluster. For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).

```bash
kbcli cluster connect pg-cluster
```
