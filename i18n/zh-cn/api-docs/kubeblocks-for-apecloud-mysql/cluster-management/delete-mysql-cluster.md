---
title: Delete an ApeCloud MySQL Cluster
description: How to delete a MySQL Cluster
keywords: [apecloud mysql, delete a cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

# Delete an ApeCloud MySQL Cluster

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
kubectl -n demo get cluster mycluster
>
NAME            CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mysql-cluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   37m
```

## Step

Run the command below to delete a specified cluster.

```bash
kubectl delete cluster mycluster -n demo
```

If you want to delete a cluster and its all related resources, you can set the termination policy as `WipeOut` and then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
