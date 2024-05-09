---
title: Delete a MongoDB Cluster
description: How to delete a MongoDB Cluster
keywords: [mongodb, delete a cluster, delete protection]
sidebar_position: 7
sidebar_label: Delete protection
---

# Delete a MongoDB cluster

## Termination policy

:::note

The termination policy determines how a cluster is deleted. Set the policy when creating a cluster.

:::

| **terminationPolicy**  | **Deleting Operation**                    |
|:--                     | :--                                       |
| `DoNotTerminate`       | `DoNotTerminate` blocks delete operation. |
| `Halt`                 | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete`               | `Delete` deletes workload resources and PVCs but keep backups. |
| `WipeOut`              | `WipeOut` deletes workload resources, PVCs and all relevant resources included backups. |

To check the termination policy, execute the following command.

```bash
kubectl -n demo get cluster mongodb-cluster 
>
NAME              CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    AGE
mongodb-cluster   mongodb              mongodb-5.0   Delete               Running   17m
```

## Steps

Run the command below to delete the cluster.

```bash
kubectl delete -n demo cluster mycluster
```

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, and then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
