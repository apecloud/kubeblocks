---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [mogodb, create a mongodb cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a MongoDB cluster

This tutorial shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to create and connect a cluster by kbcli.
* Install KubeBlocks: You can install KubeBlocks by [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the MongoDB add-on is enabled.
  


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

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with a three replicas to support automatic failover. And to ensure high availability, all replicas are distributed on different nodes by default.


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

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease cluster availability.
* Run the command below to view the flags for creating a MongoDB cluster and the default values.
  
  ```bash
  kbcli cluster create mongodb -h
  ```

:::



## Connect to a MongoDB Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```


For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
