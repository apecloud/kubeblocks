---
title: Delete a Redis Cluster
description: How to delete a Redis Cluster
keywords: [redis, delete a cluster, delete protection]
sidebar_position: 6
sidebar_label: Delete protection
---

# Delete a Redis Cluster

## Termination policy

:::note

The termination policy determines how a cluster is deleted.

:::

| **terminationPolicy**  | **Deleting Operation**                    |
|:--                     | :--                                       |
| `DoNotTerminate`       | `DoNotTerminate` blocks delete operation. |
| `Halt`                 | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete`               | `Delete` delete workload resources and PVCs but keep backups. |
| `WipeOut`              | `WipeOut` deletes workload resources, PVCs and all relevant resources included backups. |

To check the termination policy, execute the following command.

```bash
kbcli cluster list <name>
```

***Example***

```bash
kbcli cluster list redis-cluster
>
NAME   	        NAMESPACE	CLUSTER-DEFINITION	VERSION        	TERMINATION-POLICY	STATUS 	     CREATED-TIME
redis-cluster	default  	redis    	        redis-7.0.6	    Delete            	Running	     Apr 10,2023 20:27 UTC+0800
```

## Step

Configure the cluster name and run the command below to delete the specified cluster.

```bash
kbcli cluster delete <name>
```

***Example***

```bash
kbcli cluster delete redis-cluster
```
