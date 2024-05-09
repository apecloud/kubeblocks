---
title: Delete a MySQL Cluster
description: How to delete a MySQL Cluster
keywords: [mysql, delete a cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Delete a MySQL Cluster

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mysql-cluster
>
NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mysql-cluster   default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Feb 06,2023 18:27 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mysql-cluster
>
NAME            CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mysql-cluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   67m
```

</TabItem>

</Tabs>

## Step

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mysql-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mysql-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mysql-cluster
```

</TabItem>

</Tabs>
