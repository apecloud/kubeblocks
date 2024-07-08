---
title: Stop/Start an ApeCloud MySQL cluster
description: How to stop/start an ApeCloud MySQL cluster
keywords: [mysql, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start an ApeCloud MySQL cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

## Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

   ```bash
   kbcli cluster stop mysql-cluster
   ```

2. Check the status of the cluster to see whether it is stopped.

    ```bash
    kbcli cluster list
    ```

## Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.

   ```bash
   kbcli cluster start mysql-cluster
   ```

2. Check the status of the cluster to see whether it is running again.

    ```bash
    kbcli cluster list
    ```
