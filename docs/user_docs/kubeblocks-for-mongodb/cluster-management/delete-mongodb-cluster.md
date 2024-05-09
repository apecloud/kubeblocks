---
title: Delete a MongoDB Cluster
description: How to delete a MongoDB Cluster
keywords: [mongodb, delete a cluster, delete protection]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mongodb-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mongodb-cluster 
>
NAME              CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    AGE
mongodb-cluster   mongodb              mongodb-5.0   Delete               Running   17m
```

</TabItem>

</Tabs>

## Steps

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mongodb-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mongodb-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mongodb-cluster
```

</TabItem>

</Tabs>
