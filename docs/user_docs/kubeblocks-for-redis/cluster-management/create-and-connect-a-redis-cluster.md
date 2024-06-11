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

KuebBlocks for Redis supports Standalone clusters and Replication Cluster.

For your better high-availability experience, KubeBlocks creates a Redis Replication Cluster by default.

## Create a Redis cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure the Redis addon is enabled.
  
 
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

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover. And to ensure high availability, Primary and Secondary are distributed on different nodes by default.


Create a Standalone.

```bash
kbcli cluster create redis --mode standalone <clustername>
```

Create a Replication Cluster.

```bash
kbcli cluster create redis --mode replication <clustername>
```

If you only have one node for deploying a Replication, set the `availability-policy` as `none` when creating a Replication Cluster.

```bash
kbcli cluster create redis --mode replication --availability-policy none <clustername>
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease cluster availability.
* Run the command below to view the flags for creating a Redis cluster and the default values.
  
  ```bash
  kbcli cluster create redis -h
  ```

:::

## Connect to a Redis Cluster


```bash
kbcli cluster connect <clustername>  --namespace <name>
```


For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
