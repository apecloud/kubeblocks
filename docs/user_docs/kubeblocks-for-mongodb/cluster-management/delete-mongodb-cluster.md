---
title: Delete a MongoDB Cluster
description: How to delete a MongoDB Cluster
keywords: [mongodb, delete a cluster, delete protection]
sidebar_position: 6
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
kbcli cluster list mongodb-cluster
```

## Option 1. Use kbcli

Configure the cluster name and run the command below to delete the specified cluster.

```bash
kbcli cluster delete mongodb-cluster
```

## Option 2. Use kubectl

Configure the cluster name and run the command below to delete the specified cluster.

```bash
kubectl delete cluster mongodb-cluster
```
