---
title: Delete a PostgreSQL Cluster
description: How to delete a PostgreSQL Cluster
keywords: [postgresql, delete a cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list pg-cluster
>
NAME         NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
pg-cluster   default     postgresql           postgresql-14.7.0   Delete               Running   Mar 03,2023 18:49 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster pg-cluster
>
NAME         CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
pg-cluster   postgresql           postgresql-14.8.0   Delete               Running   29m
```

</TabItem>

</Tabs>

## Step

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete pg-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster pg-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster pg-cluster
```

</TabItem>

</Tabs>
