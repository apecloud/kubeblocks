---
title: Delete a kafka Cluster
description: How to delete a kafka Cluster
keywords: [kafka, delete a cluster, delete protection]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Delete a Kafka cluster

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

To check the termination policy, execute the following command.

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mycluster
>
NAME           CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
mycluster      kafka                kafka-3.3.2    Delete               Running    19m
```

</TabItem>

</Tabs>

## Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Run the command below to delete a specified cluster.

```bash
kbcli cluster delete mycluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Run the command below to delete a specified cluster.

```bash
kubectl delete -n demo cluster mycluster
```

If you want to delete a cluster and its all related resources, you can set the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

</Tabs>
