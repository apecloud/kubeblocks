---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [redis, create a redis cluster, connect to a redis cluster, cluster, redis sentinel]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and Connect to a Redis cluster

This tutorial shows how to create and connect to a Redis cluster.

## Create a Redis cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure the Redis addon is enabled. If this addon is not enabled, [enable it](./../../overview/supported-addons.md#use-addons) first.

  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  redis                     Helm   Enabled                   true
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

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover. To ensure high availability, Primary and Secondary are distributed on different nodes by default.

Create a Standalone.

```bash
kbcli cluster create --cluster-definition redis --set replicas=1 <clustername>
```

Create a Replication Cluster.

```bash
kbcli cluster create --cluster-definition redis --set replicas=2 <clustername>
```

If you only have one node for deploying a Replication, set the `availability-policy` as `none` when creating a Replication Cluster.

```bash
kbcli cluster create --cluster-definition redis --set replicas=2 --topology-keys null <clustername>
```

If you want to specify a cluster version, you can first view the available versions and use `--version` to specify a version.

```bash
kbcli clusterversion list

kbcli cluster create --cluster-definition redis --version redis-7.2.4 <clustername>
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.
* View more flags for creating a Redis cluster to create a cluster with customized specifications.

  ```bash
  kbcli cluster create --help
  ```

:::

## Connect to a Redis Cluster

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
