---
title: Stop/Start a Kafka cluster
description: How to start/stop a Kafka cluster
keywords: [mongodb, stop a kafka cluster, start a kafka cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start a Kafka Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

***Steps:***

1. Configure the name of your cluster and run the command below to stop this cluster.

    ```bash
    kbcli cluster stop kafka
    ```

2. Check the status of the cluster to see whether it is stopped.

    ```bash
    kbcli cluster list
    ```

## Start a cluster
  
Configure the name of your cluster and run the command below to start this cluster.

```bash
kbcli cluster start kafka
```
