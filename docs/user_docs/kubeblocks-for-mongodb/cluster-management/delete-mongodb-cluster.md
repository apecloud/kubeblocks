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
kbcli cluster list mongodb-cluster
```

## Steps

Run the command below to delete a specified cluster.

```bash
kbcli cluster delete mongodb-cluster
```
