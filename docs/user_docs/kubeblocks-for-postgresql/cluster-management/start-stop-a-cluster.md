---
title: Stop/Start a PostgreSQL cluster
description: How to start/stop a PostgreSQL cluster
keywords: [postgresql, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start PostgreSQL Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

Configure the name of your cluster and run the command below to stop this cluster.

```bash
kbcli cluster stop pg-cluster
```

## Start a cluster

Configure the name of your cluster and run the command below to start this cluster.

```bash
kbcli cluster start pg-cluster
```
