---
title: Delete a MongoDB Cluster
description: How to delete a MongoDB Cluster
keywords: [MongoDB, delete a cluster]
sidebar_position: 6
sidebar_label: Delete protection
---

# Delete a MongoDB Clusterr

## Termination policy

:::note

The termination policy determines how you delete a cluster.

:::

| **terminationPolicy** | **Deleting Operation**                           |
|:----------------------|:-------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` blocks delete operation.        |
| `Halt`                | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete`              | `Delete` deletes workload resources and PVCs but keep backups.   |
| `WipeOut`             | `WipeOut` deletes workload resources, PVCs and all relevant resources included backups.    |

To check the termination policy, execute the following command.

```bash
$ kubectl -n demo get cluster mongodb-cluster 
NAME              CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    AGE
mongodb-cluster   mongodb              mongodb-5.0.14   Delete               Running   17m
```

## Step

If you want to delete cluster and all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
$ kubectl patch -n demo cluster mongodb-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"
$ kubectl delete -n demo cluster mongodb-cluster
```