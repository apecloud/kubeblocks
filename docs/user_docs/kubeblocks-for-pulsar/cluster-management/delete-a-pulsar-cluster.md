---
title: Delete a PostgreSQL Cluster
description: How to delete a PostgreSQL Cluster
keywords: [postgresql, delete a cluster]
sidebar_position: 7
sidebar_label: Delete protection
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Delete a Pulsar Cluster

:::note

The termination policy determines how a cluster is deleted.

:::

## Termination policy

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
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        pulsar               pulsar-3.0.2        Delete               Running   Sep 28,2024 16:47 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mycluster
>
NAME           CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS     AGE
mycluster      pulsar               pulsar-3.0.2     Delete               Running    19m
```

</TabItem>

</Tabs>

## Step

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl delete cluster mycluster -n demo
```

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, and then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

</Tabs>
