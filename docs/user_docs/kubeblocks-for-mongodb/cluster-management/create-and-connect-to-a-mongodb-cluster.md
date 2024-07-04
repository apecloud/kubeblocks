---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [mogodb, create a mongodb cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MongoDB cluster

This tutorial shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure the MongoDB addon is enabled. If this addon is not enabled, [enable it](./../../overview/supported-addons.md#use-addons) first.

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  mongodb                        Helm   Enabled                   true
  ...
  ```

* View all the database types and versions available for creating a cluster.

  ```bash
  kbcli clusterdefinition list

  kbcli clusterversion list
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### Create a cluster

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with two replicas to support automatic failover. To ensure high availability, all replicas are distributed on different nodes by default.

Create a Standalone.

```bash
kbcli cluster create mongodb <clustername>
```

Create a ReplicatSet.

```bash
kbcli cluster create mongodb --mode replicaset <clustername>
```

If you only have one node for deploying a ReplicaSet, set the `availability-policy` as `none` when creating a ReplicaSet.

```bash
kbcli cluster create mongodb --mode replicaset --availability-policy none <clustername>
```

If you want to specify a cluster version, you can first view the available versions and use `--version` to specify a version.

```bash
kbcli clusterversion list

kbcli cluster create mongodb <clustername> --version mongodb-6.0
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.
* View more flags for creating a MongoDB cluster to create a cluster with customized specifications.

  ```bash
  kbcli cluster create mongodb --help
  ```

:::

## Connect to a MongoDB Cluster

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
