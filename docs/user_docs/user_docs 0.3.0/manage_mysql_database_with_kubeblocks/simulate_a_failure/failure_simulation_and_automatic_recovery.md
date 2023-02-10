# Failure simulation and automatic recovery

As an open-source data management platform, Kubeblocks supports two database forms, ReplicationSet and ConsensusSet. ReplicationSet can be used for single source with multiple replicas, and non-automatic switching database management, such as MySQL and Redis. ConsensusSet can be used for database management with multiple replicas and automatic switching capabilities, such as ApeCloud MySQL Paxos group with multiple replicas, MongoDB, etc. The ConsensusSet database management capability has been released in KubeBlocks v0.3.0, and ReplicationSet is under development. This guide takes ApeCloud MySQL as an example to introduce the high availability capability of the database in the form of ConsensusSet. This capability is also applicable to other database engines.

***Before you start***

* Install a Kubernetes cluster and KubeBlocks, refer to [Install KubeBlocks](../../install_kbcli_kubeblocks/install_and_unistall_kbcli_and_kubeblocks.md).
* Create an ApeCloud MySQL Paxos group, refer to [Create a MySQL cluster](create_and_connect_a_mysql_cluster.md).
* Run `kubectl get cd apecloud-mysql -o yaml` to check whether _rolechangedprobe_ is enabled in the ApeCloud MySQL Paxos group (it is enabled by default). If the following configuration exists, it indicates that it is enabled:
  ```
  probes:
  roleChangedProbe:
    failureThreshold: 3
    periodSeconds: 2
    timeoutSeconds: 1
  ```

## Recovery simulation

> ***Note:*** 
> 
> The faults here are all simulated by deleting a pod. When there are sufficient resources, the fault can also be simulated by machine downtime or container deletion, and its automatic recovery is the same as described here.

### Leader pod fault

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information. View the leader pod name in `Topology`. In this example, the leader pod's name is mysql-cluster.
   ```
   kbcli cluster describe mysql-cluster
   ```
   ![describe_cluster](../../image/describe_cluster.png)
2. Delete the leader pod `wesql-replicasets-1` to simulate a pod fault.
   ```
   kubectl delete pod mysql-cluster-mysql-1
   ```

   ![delete_pod](../../image/delete_pod.png)
3. Run `kbcli cluster describe` and `kbcli cluster connect` to check the status of the pods and Paxos group connection.
   
   ***Results***

   The following example shows that the roles of pods have changed after the old leader pod was deleted and `xxx` is elected as the new leader pod.
   ```
   kbcli cluster describe mysql-cluster
   ```
   ![describe_cluster_after](../../image/describe_cluster_after.png)
   It shows that this ApeCloud MySQL Paxos group can be connected within seconds.
   ```
   kbcli cluster connect mysql-cluster
   ```
   ![connect_cluster_after](../../image/connect_cluster_after.png)

   ***How the automatic recovery works***

   After the leader pod is deleted, the ApeCloud MySQL Paxos group elects a new leader. In this example, `xxx` is elected as the new leader. Kubeblocks detects that the leader has changed, and sends a notification to update the access link. The original exception node automatically rebuilds and recovers to the normal Paxos group state. It normally takes 30 seconds from exception to recovery.

### Single follower pod exception

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information and view the follower pod name in `Topology`. In this example, the follower pods are xxx and xxx.
   ```
   kbcli cluster describe mysql-cluster
   ```
   ![describe_cluster](../../image/describe_cluster.png)
2. Delete the follower pod xxx.
   ```
   kubectl delete pod mysql-cluster-mysql-0
   ```

   ![delete_follower_pod](../../image/delete_follower_pod.png)
3. Run the command below to view the Paxos group status and you can find the follower pod is being terminated in `Component.Instance`.
   ```
   kbcli cluster describe mysql-cluster
   ```

   ![describe_cluster_follower](../../image/describe_cluster_follower.png)
4. Run the command below to connect to the Paxos group and you can find this single follower exception doesn't affect the R/W of the cluster.
   ```
   kbcli cluster connect mysql-cluster
   ```

   ![connect_cluster_follower](../../image/connect_cluster_follower.png)

   ***How the automatic recovery works***

   One follower exception doesn't trigger re-electing of the leader or access link switch, so the R/W of the cluster is not affected. Follower exception triggers recreation and recovery. The process takes no more than 30 seconds. 

### Two pods exception

The availability of the cluster generally requires the majority of pods to be in a normal state. When most pods are exceptional, the original leader will be automatically downgraded to a follower. Therefore, any two exceptional pods result in only one follower pod remaining. 
Therefore, whether exceptions occur to one leader and one follower or exceptions occur to two followers, failure performance and automatic recovery are the same. 

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information and view the follower pod name in `Topology`. In this example, the follower pods are xxx and xxx.
   ```
   kbcli cluster describe mysql-cluster
   ```
   ![describe_cluster](../../image/describe_cluster.png)
2. Delete these two follower pods.
   ```
   kubectl delete pod mysql-cluster-mysql-1 mysql-cluster-mysql-0
   ```

   ![delete_two_pods](../../image/delete_two_pods.png)
3. Run the command below to view the Paxos group status and you can find the follower pods are being terminated in `Component.Instance`.
   ```
   kbcli cluster describe mysql-cluster
   ```

   ![describe_two_clusters](../../image/describe_two_clusters.png)
4. Run `kbcli cluster describe xxx` again after a few seconds and you can find the pods in the Paxos group work normally again in `Component.Instance`.
   ```
   kbcli cluster connect mysql-cluster
   ```

   ![connect_two_clusters](../../image/connect_two_clusters.png)

   ***How the automatic recovery works***

   When two pods of the ApeCloud MySQL Paxos group are exceptional, pods are unavailable and cluster R/W is unavailable. After the recreation of pods, a new leader is elected to recover to R/W status. The process takes less than 30 seconds.

### All pods exception

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information and view the pods' names in `Topology`.
   ```
   kbcli cluster describe mysql-cluster
   ```
   ![describe_cluster](../../image/describe_cluster.png)
2. Delete all pods.
   ```
   kubectl delete pod mysql-cluster-mysql-1 mysql-cluster-mysql-0 mysql-cluster-mysql-2
   ```

   ![delete_three_pods](../../image/delete_three_pods.png)
3. Run the command below to view the deleting process. You can find the pods are being deleted in `Component.Instance` and the follower pod is the last one to be deleted.
   ```
   kbcli cluster describe mysql-cluster
   ```

   ![describe_three_clusters](../../image/describe_three_clusters.png)
4. Run `kbcli cluster describe xxx` again after a few seconds and you can find the pods in the Paxos group work normally again in `Component.Instance`.
   ```
   kbcli cluster connect mysql-cluster
   ```

   ![connect_three_clusters](../../image/connect_three_clusters.png)

   ***How the automatic recovery works***

    Every time the pod is deleted, recreation is triggered. And then ApeCloud MySQL automatically completes the cluster recovery and the election of a new leader. After the election of the leader is completed, Kubeblocks detects the new leader and updates the access link. This process takes less than 30 seconds.