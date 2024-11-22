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

| **terminationPolicy** | **Deleting Operation**                           |
|:----------------------|:-------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` blocks delete operation.        |
| `Halt`                | `Halt` deletes Cluster resources like Pods and Services but retains Persistent Volume Claims (PVCs), allowing for data preservation while stopping other operations. Halt policy is deprecated in v0.9.1 and will have same meaning as DoNotTerminate. |
| `Delete`              | `Delete` extends the Halt policy by also removing PVCs, leading to a thorough cleanup while removing all persistent data.   |
| `WipeOut`             | `WipeOut` deletes all Cluster resources, including volume snapshots and backups in external storage. This results in complete data removal and should be used cautiously, especially in non-production environments, to avoid irreversible data loss.   |

To check the termination policy, execute the following command.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mycluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
