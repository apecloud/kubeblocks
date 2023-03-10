---
title: Delete a MySQL Cluster
description: How to delete a MySQL Cluster
sidebar_position: 6
---

# Delete a MySQL Cluster

> ***Note:*** 
>
> The termination policy determines how you delete a cluster.

| **terminationPolicy**  | **Deleting Operation**                    |
|:--                     | :--                                       |
| `DoNotTerminate`       | `DoNotTerminate` blocks delete operation. |
| `Halt`                 | `Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs. |
| `Delete `              | `Delete` deletes workload resources and PVCs. |
| `WipeOut`              | `WipeOut` deletes workload resources and PVCs and wipes out all volume snapshots and snapshot data from backup storage location. |

To check the termination policy, execute the following command.

```bash
kbcli cluster list <name>
```

***Example***

```bash
kbcli cluster list mysql-cluster
>
NAME   	        NAMESPACE	CLUSTER-DEFINITION	VERSION        	TERMINATION-POLICY	STATUS 	CREATED-TIME
mysql-cluster	default  	apecloud-mysql    	ac-mysql-8.0.30	Delete            	Running	Feb 06,2023 18:27 UTC+0800
```

***Steps:***

**Option 1.** Use `kbcli`.

Configure the cluster name and run the command below to delete the specified cluster.

```bash
kbcli cluster delete <name>
```

***Example***

```bash
kbcli cluster delete mysql-cluster
```

**Option 2.** Use `kubectl`.

Configure the cluster name and run the command below to delete the specified cluster.

```bash
kubectl delete cluster <name>
```

***Example***

```bash
kubectl delete cluster mysql-cluster
```
