---
title: Delete a Redis Cluster
description: How to delete a Redis Cluster
keywords: [redis, delete a cluster, delete protection]
sidebar_position: 6
sidebar_label: Delete protection
---

# Delete a Redis Cluster

## Termination policy

:::note

The termination policy determines how a cluster is deleted.

:::

| **terminationPolicy**  | **Deleting Operation**                    |
|:--                     | :--                                       |
| `DoNotTerminate`       | `DoNotTerminate` blocks delete operation. |
| `Halt`                 | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete`               | `Delete` deletes workload resources and PVCs but keep backups. |
| `WipeOut`              | `WipeOut` deletes workload resources, PVCs and all relevant resources included backups. |

To check the termination policy, execute the following command.

```bash
kubectl -n demo get cluster mycluster
>
NAME    CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
redis   redis                redis-7.0.6   Delete               Running   10m
```

## Step

Run the command below to delete a specified cluster.

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```