# Delete a MySQL Cluster
***Note:***
The termination policy determines how you delete a cluster.
terminationPolicy | Deleting Operation|
---|---|
|`DoNotTerminate`|`DoNotTerminate` blocks delete operation.|
`Halt`|`Halt` deletes workload resources such as statefulset, deployment workloads but keep PVCs.
`Delete`|`Delete` deletes workload resources and PVCs.
`WipeOut`|`WipeOut` deletes workload resources and PVCs and wipes out all volume snapshots and snapshot data from backup storage location.
***Steps:***
**Option 1.** Use kbcli.
Configure the cluster name and run the command below to delete the specified cluster.
```
kbcli cluster delete NAME
```
**Option 2.** Use kubectl.
Configure the cluster name and run the command below to delete the specified cluster.
```
kubectl delete cluster NAME
```
