---
title: Delete an ApeCloud MySQL Cluster
description: How to delete an ApeCloud MySQL Cluster
keywords: [mysql, delete an apecloud cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Delete an ApeCloud MySQL Cluster

## Termination policy

:::note

The termination policy determines how a cluster is deleted.

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
kbcli cluster list -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Sep 19,2024 16:01 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   27m
```

</TabItem>

</Tabs>

## Step

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mycluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

</Tabs>
