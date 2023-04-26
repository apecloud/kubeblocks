---
title: Stop/Start a MySQL cluster
description: How to start/stop a MySQL cluster
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start MySQL Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

***Steps:***

1. Configure the name of your cluster and run the command below to stop this cluster. 

    ```bash
    kbcli cluster stop <name>
    ```
    **Example**

    ```
    kbcli cluster stop mongodb-cluster
    ```

2. Check the status of the cluster to see whether it is stopped.

    ```
    kbcli cluster list
    ```


## Start a cluster
  

Configure the name of your cluster and run the command below to stop this cluster. 

```bash
kbcli cluster start <name>
```

***Example***

```bash
kbcli cluster start mongodb-cluster
```

