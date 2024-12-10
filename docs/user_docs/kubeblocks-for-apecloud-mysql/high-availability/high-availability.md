---
title: Failure simulation and automatic recovery
description: Automatic recovery of cluster
keywords: [mysql, high availability, failure simulation, automatic recovery]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Failure simulation and automatic recovery

As an open-source data management platform, Kubeblocks currently supports over thirty database engines and is continuously expanding. Due to the varying high availability capabilities of databases, KubeBlocks has designed and implemented a high availability (HA) system for database instances. The KubeBlocks HA system uses a unified HA framework to provide high availability for databases, allowing different databases on KubeBlocks to achieve similar high availability capabilities and experiences.

This tutorial uses ApeCloud MySQL RaftGroup Cluster as an example to demonstrate its fault simulation and recovery capabilities.

## Recovery simulation

:::note

The faults here are all simulated by deleting a pod. When there are sufficient resources, the fault can also be simulated by machine downtime or container deletion, and its automatic recovery is the same as described here.

:::

### Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Create an ApeCloud MySQL RaftGroup, refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-an-apecloud-mysql-cluster.md).
* Run `kubectl get cd apecloud-mysql -o yaml` to check whether _rolechangedprobe_ is enabled in the ApeCloud MySQL RaftGroup (it is enabled by default). If the following configuration exists, it indicates that it is enabled:

  ```bash
  probes:
    roleProbe:
      failureThreshold: 2
      periodSeconds: 1
      timeoutSeconds: 1
  ```

### Leader pod fault

***Steps:***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. View the pod role of the ApeCloud MySQL RaftGroup Cluster. In this example, the leader pod's name is `mycluster-mysql-1`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_pod](./../../../img/api-ha-grep-role.png)
2. Delete the leader pod `mycluster-mysql-1` to simulate a pod fault.

    ```bash
    kubectl delete pod mycluster-mysql-1 -n demo
    ```

    ![delete_pod](./../../../img/api-ha-delete-leader-pod.png)
3. Check the status of the pods and RaftGroup Cluster connection.

    The following example shows that the roles of pods have changed after the old leader pod was deleted and `mycluster-mysql-0` is elected as the new leader pod.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_after](./../../../img/api-ha-delete-leader-pod-after.png)

    Connect to this cluster to check the pod roles and status. This cluster can be connected within seconds.

    ```bash
    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
    >
    root

    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
    >
    pt2mmdlp4

    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_after](./../../../img/api-ha-leader-pod-connect-check.png)

***How the automatic recovery works***

After the leader pod is deleted, the ApeCloud MySQL RaftGroup Cluster elects a new leader. In this example, `mycluster-mysql-0` is elected as the new leader. KubeBlocks detects that the leader has changed, and sends a notification to update the access link. The original exception node automatically rebuilds and recovers to the normal RaftGroup Cluster state. It normally takes 30 seconds from exception to recovery.

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. View the ApeCloud MySQL RaftGroup information. View the leader pod name in `Topology`. In this example, the leader pod's name is `mycluster-mysql-2`.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster](./../../../img/failure_simulation_describe_cluster.png)
2. Delete the leader pod `mycluster-mysql-2` to simulate a pod fault.

    ```bash
    kubectl delete pod mycluster-mysql-2 -n demo
    ```

    ![delete_pod](./../../../img/failure_simulation_delete_pod.png)
3. Run `kbcli cluster describe` and `kbcli cluster connect` to check the status of the pods and RaftGroup connection.

    ***Results***

    The following example shows that the roles of pods have changed after the old leader pod was deleted and `mycluster-mysql-1` is elected as the new leader pod.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster_after](./../../../img/failure_simulation_describe_cluster_after.png)
    It shows that this ApeCloud MySQL RaftGroup can be connected within seconds.

    ```bash
    kbcli cluster connect mycluster -n demo
    ```

    ![connect_cluster_after](./../../../img/failure_simulation_connect_cluster_after.png)

***How the automatic recovery works***

After the leader pod is deleted, the ApeCloud MySQL RaftGroup Cluster elects a new leader. In this example, `mycluster-mysql-1` is elected as the new leader. KubeBlocks detects that the leader has changed, and sends a notification to update the access link. The original exception node automatically rebuilds and recovers to the normal RaftGroup Cluster state. It normally takes 30 seconds from exception to recovery.

</TabItem>

</Tabs>

### Single follower pod exception

***Steps:***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. View the pod role again and in this example, the follower pods are `mycluster-mysql-1` and `mycluster-mysql-2`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-grep-role-single-follower-pod.png)
2. Delete the follower pod `mycluster-mysql-1`.

    ```bash
    kubectl delete pod mycluster-mysql-1 -n demo
    ```

    ![delete_follower_pod](./../../../img/api-ha-single-follower-pod-delete.png)
3. Open another terminal page and view the pod status. You can find the follower pod `mycluster-mysql-1` is `Terminating`.

    ```bash
    kubectl get pod -n demo
    ```

    ![view_cluster_follower_status](./../../../img/api-delete-single-follower-pod-status.png)

    View the pod roles again.

    ![describe_cluster_follower](./../../../img/api-ha-single-follower-pod-grep-role-after.png)

4. Connect to this cluster and you can find this single follower exception doesn't affect the R/W of the cluster.

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_follower](./../../../img/api-ha-connect-single-follower-pod.png)

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. View the ApeCloud MySQL RaftGroup information and view the follower pod name in `Topology`. In this example, the follower pods are `mycluster-mysql-0` and m`mycluster-mysql-2`.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster](./../../../img/failure_simulation_describe_cluster_after.png)
2. Delete the follower pod `mycluster-mysql-0`.

    ```bash
    kubectl delete pod mycluster-mysql-0 -n demo
    ```

    ![delete_follower_pod](./../../../img/failure_simulation_delete_follower_pod.png)
3. View the RaftGroup status and you can find the follower pod is being terminated.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster_follower](./../../../img/failure_simulation_describe_cluster_follower.png)
4. Connect to the RaftGroup and you can find this single follower exception doesn't affect the R/W of the cluster.

    ```bash
    kbcli cluster connect mycluster -n demo
    ```

    ![connect_cluster_follower](./../../../img/failure_simulation_connect_cluster_follower.png)

</TabItem>

</Tabs>

***How the automatic recovery works***

One follower exception doesn't trigger re-electing of the leader or access link switch, so the R/W of the cluster is not affected. Follower exception triggers recreation and recovery. The process takes no more than 30 seconds.

### Two pods exception

The availability of the cluster generally requires the majority of pods to be in a normal state. When most pods are exceptional, the original leader will be automatically downgraded to a follower. Therefore, any two exceptional pods result in only one follower pod remaining.

In this way, whether exceptions occur to one leader and one follower or two followers, failure performance and automatic recovery are the same.

***Steps:***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. View the pod role again. In this example, the follower pods are `mycluster-mysql-1` and `mycluster-mysql-2`.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-two-pods-grep-role.png)
2. Delete these two follower pods.

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-2 -n demo
    ```

    ![delete_two_pods](./../../../img/api-ha-two-pod-get-status.png)
3. Open another terminal page and view the pod status. You can find the follower pods `mycluster-mysql-1` and `mycluster-mysql-2` is `Terminating`.

    ```bash
    kubectl get pod -n demo
    ```

    ![view_cluster_follower_status](./../../../img/api-ha-two-pod-get-status.png)

    View the pod roles and you can find a new leader pod is selected.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_follower](./../../../img/api-ha-two-pods-grep-role-after.png)

4. Connect to this cluster after a few seconds and you can find the pods in the RaftGroup Cluster work normally again.

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_two_pods](./../../../img/api-ha-two-pods-connect-after.png)

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. View the ApeCloud MySQL RaftGroup information and view the follower pod name in `Topology`. In this example, the follower pods are `mycluster-mysql-0` and `mycluster-mysql-2`.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster](./../../../img/failure_simulation_describe_cluster_2.png)
2. Delete these two follower pods.

    ```bash
    kubectl delete pod mycluster-mysql-0 mycluster-mysql-2 -n demo
    ```

    ![delete_two_pods](./../../../img/failure_simulation_delete_two_pods.png)
3. View the RaftGroup status and you can find the follower pods are pending and a new leader pod is selected.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_two_pods](./../../../img/failure_simulation_describe_two_pods.png)
4. Run `kbcli cluster connect mycluster` again after a few seconds and you can find the pods in the RaftGroup work normally again.

    ```bash
    kbcli cluster connect mycluster -n demo
    ```

    ![connect_two_pods](./../../../img/failure_simulation_connect_two_pods.png)

</TabItem>

</Tabs>

***How the automatic recovery works***

When two pods of the ApeCloud MySQL RaftGroup are exceptional, pods are unavailable and cluster R/W is unavailable. After the recreation of pods, a new leader is elected to recover to R/W status. The process takes less than 30 seconds.

### All pods exception

***Steps:***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. View the role of pods.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-all-pods-grep-role.png)
2. Delete all pods.

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-0 mycluster-mysql-2 -n demo
    ```

    ![delete_three_pods](./../../../img/api-ha-all-pods-delete.png)
3. Open another terminal page and view the pod status. You can find the pods are terminating.

    ```bash
    kubectl get pod -n demo
    ```

    ![describe_three_clusters](./../../../img/api-ha-all-pods-get-status.png)
4. Run the command below to view the pod status. After a few seconds, you can find all pods are running again and a new leader is selected.

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_follower](./../../../img/api-ha-all-pods-grep-role-after.png)
5. Connect to the cluster and the connection works normally again.

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_three_clusters](./../../../img/api-ha-all-pods-connect-after.png)

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Run the command below to view the ApeCloud MySQL RaftGroup information and view the pods' names in `Topology`.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_cluster](./../../../img/failure_simulation_describe_two_pods.png)
2. Delete all pods.

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-0 mycluster-mysql-2 -n demo
    ```

    ![delete_three_pods](./../../../img/failure_simulation_delete_three_pods.png)
3. Run the command below to view the cluster and pod status. After a few seconds, you can find all pods are running again and a new leader is selected.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

    ![describe_three_clusters](./../../../img/failure_simulation_describe_three_pods.png)
4. Connect to the cluster and the connection works normally again.

    ```bash
    kbcli cluster connect mycluster -n demo
    ```

    ![connect_three_clusters](./../../../img/failure_simulation_connect_three_pods.png)

</TabItem>

</Tabs>

***How the automatic recovery works***

Every time the pod is deleted, recreation is triggered. And then ApeCloud MySQL automatically completes the cluster recovery and the election of a new leader. After the election of the leader is completed, KubeBlocks detects the new leader and updates the access link. This process takes less than 30 seconds.
