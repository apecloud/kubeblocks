---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [postgresql, create a postgresql cluster, connect to a postgresql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a PostgreSQL cluster

This tutorial shows how to create and connect to a PostgreSQL cluster.

## Create a PostgreSQL cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure the PostgreSQL addon is enabled.
  
  ```bash
  kbcli addon list
  >
  NAME                       TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  postgresql                 Helm   Enabled                   true
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
   ```

### Create a cluster

KubeBlocks supports creating two types of PostgreSQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with a Replication Cluster to support automatic failover. To ensure high availability, Primary and Secondary are distributed on different nodes by default.

Create a Standalone.

```bash
kbcli cluster create postgresql <clustername>
```

Create a Replication Cluster.

```bash
kbcli cluster create postgresql --mode replication <clustername>
```

If you only have one node for deploying a Replication, set the `availability-policy` as `none` when creating a Replication Cluster.

```bash
kbcli cluster create postgresql --mode replication --availability-policy none <clustername>
```

If you want to specify a cluster version, you can first view the available versions and use `--version` to specify a version.

```bash
kbcli clusterversion list

kbcli cluster create posrgresql pg-cluster --version postgresql-14.8.0
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.
* View more flags for creating a PostgreSQL cluster to create a cluster with customized specifications.
  
  ```bash
  kbcli cluster create postgresql --help
  ```

:::

## Connect to a PostgreSQL Cluster

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
