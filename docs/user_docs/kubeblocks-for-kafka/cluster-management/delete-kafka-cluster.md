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

To check the termination policy, execute the following command.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list kafka
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
~ kubectl -n default get cluster kafka
NAME    CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS     AGE
kafka      kafka                                kafka-3.3.2    Delete                               Running    19m
```

</TabItem>

</Tabs>

## Steps

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete kafka
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster kafka -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster kafka
```

</TabItem>

</Tabs>
