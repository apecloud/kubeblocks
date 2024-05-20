---
title: Failure simulation and automatic recovery
description: Automatic recovery of cluster
keywords: [mysql, high availability, failure simulation, automatic recovery]
sidebar_position: 1
---

# Failure simulation and automatic recovery

This guide illustrates the high availability capability of MySQL community edition.

## Recovery simulation

:::note

The faults here are all simulated by deleting a pod. When there are sufficient resources, the fault can also be simulated by machine downtime or container deletion, and its automatic recovery is the same as described here.

:::

### Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Create an ApeCloud MySQL RaftGroup, refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md).
* Run `kubectl get cd apecloud-mysql -o yaml` to check whether _rolechangedprobe_ is enabled in the ApeCloud MySQL RaftGroup (it is enabled by default). If the following configuration exists, it indicates that it is enabled:

  ```bash
  probes:
    roleProbe:
      failureThreshold: 2
      periodSeconds: 1
      timeoutSeconds: 1
  ```

### Primary pod fault

***Steps:***

1. View the pod role of the MySQL cluster. In this example, the leader pod's name is `mysql-cluster-1`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_pod](./../../../img/api-ha-grep-role.png)
2. Delete the primary pod `mysql-cluster-mysql-1` to simulate a pod fault.

    ```bash
    kubectl delete pod mysql-cluster-mysql-1 -n demo
    ```

    ![delete_pod](./../../../img/api-ha-delete-leader-pod.png)
3. Run `kbcli cluster describe` and `kbcli cluster connect` to check the status of the pods and cluster connection.

    ***Results***

    The following example shows that the roles of pods have changed after the old leader pod was deleted and `mysql-cluster-mysql-0` is elected as the new leader pod.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_after](./../../../img/api-ha-delete-leader-pod-after.png)

    Connect to this cluster to check the pod roles and status. This cluster can be connected within seconds.

    ```bash
    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
    >
    root

    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
    >
    pt2mmdlp4

    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_after](./../../../img/api-ha-leader-pod-connect-check.png)

   ***How the automatic recovery works***

   After the primary pod is deleted, the MySQL cluster elects a new primary. In this example, `mysql-cluster-mysql-0` is elected as the new primary. KubeBlocks detects that the primary has changed, and sends a notification to update the access link. The original exception node automatically rebuilds and recovers to the normal Replication state. It normally takes 30 seconds from exception to recovery.

### Secondary pod exception

***Steps:***

1. View the pod role again and in this example, the secondary pod is `mysql-cluster-mysql-0`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-grep-role-single-follower-pod.png)
2. Delete the secondary pod `mysql-cluster-mysql-0`.

    ```bash
    kubectl delete pod mycluster-mysql-1 -n demo
    ```

    ![delete_follower_pod](./../../../img/api-ha-single-follower-pod-delete.png)
3. Open another terminal page and view the pod status. You can find the follower pod `mysql-cluster-mysql-1` is `Terminating`.

    ```bash
    kubectl get pod -n demo
    ```

    ![view_cluster_follower_status](./../../../img/api-delete-single-follower-pod-status.png)

    View the pod roles again.

    ![describe_cluster_follower](./../../../img/api-ha-single-follower-pod-grep-role-after.png)

4. Connect to this cluster and you can find this secondary exception doesn't affect the R/W of the cluster.

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_follower](./../../../img/api-ha-connect-single-follower-pod.png)

### All pods exception

***Steps:***

1. Run the command below to view the ApeCloud MySQL RaftGroup information and view the pods' names in `Topology`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-all-pods-grep-role.png)
2. Delete all pods.

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-0 mycluster-mysql-2 -n demo
    ```

    ![delete_three_pods](./../../../img/api-ha-all-pods-delete.png)
3. Open another terminal page and view the pod status. You can find the pods are pending.

    ```bash
    kubectl get pod -n demo
    ```

    ![describe_three_clusters](./../../../img/api-ha-all-pods-get-status.png)
4. View the pod roles and you can find a new leader pod is selected.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_follower](./../../../img/api-ha-all-pods-grep-role-after.png)
5. Connect to this cluster after a few seconds and you can find the pods in this cluster work normally again.

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_three_clusters](./../../../img/api-ha-all-pods-connect-after.png)

   ***How the automatic recovery works***

   Every time the pod is deleted, recreation is triggered. And then the MySQL cluster automatically completes the cluster recovery and the election of a new primary. After the election of the primary is completed, KubeBlocks detects the new primary and updates the access link. This process takes less than 30 seconds.
