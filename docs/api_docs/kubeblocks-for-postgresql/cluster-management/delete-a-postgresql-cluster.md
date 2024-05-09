---
title: Delete a PostgreSQL Cluster
description: How to delete a PostgreSQL Cluster
keywords: [postgresql, delete a cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

# Delete a PostgreSQL Cluster

:::note

The termination policy determines how a cluster is deleted.

:::

## Termination policy

| **terminationPolicy** | **Deleting Operation**                                                                     |
|:----------------------|:-------------------------------------------------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` blocks delete operation.                                                  |
| `Halt`                | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete`              | `Delete` deletes workload resources and PVCs but keep backups.                              |
| `WipeOut`             | `WipeOut` deletes workload resources, PVCs and all relevant resources included backups.    |

To check the termination policy, execute the following command.

```bash
kubectl -n demo get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
mycluster   postgresql           postgresql-14.8.0   Delete               Running   29m
```

## Step

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, and then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
