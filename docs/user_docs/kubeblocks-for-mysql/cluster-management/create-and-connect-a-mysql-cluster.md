---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a MySQL cluster

This tutorial shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks by kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure the ApeCloud MySQL addon is enabled.
  
  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  apecloud-mysql                 Helm   Enabled                   true
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

KubeBlocks supports creating two types of MySQL clusters: Standalone and RaftGroup Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a RaftGroup Cluster, which creates a cluster with three replicas. And to ensure high availability, all replicas are distributed on different nodes by default.

Create a Standalone.

```bash
kbcli cluster create mysql <clustername>
```

Create a RaftGroup Cluster.

```bash
kbcli cluster create mysql --mode raftGroup <clustername>
```

If you only have one node for deploying a RaftGroup Cluster, set the `availability-policy` as `none` when creating a RaftGroup Cluster.

```bash
kbcli cluster create mysql --mode raftGroup --availability-policy none <clustername>
```

If you want to specify a cluster version, you can first view the available versions and use `--clusterversion` to specify a version.

```bash
kbcli clusterversion list

kbcli cluster create mysql mycluster --cluster-version ac-mysql-8.0.30
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.
* View more flags for creating a MySQL cluster to create a cluster with customized specifications.
  
  ```bash
  kbcli cluster create mysql --help
  ```

:::

## Connect to a MySQL Cluster

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
