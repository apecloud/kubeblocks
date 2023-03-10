---
title: Restart MySQL cluster
description: How to restart a MySQL cluster
sidebar_position: 4
---

# Restart MySQL cluster
You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

> ***Note:*** 
> 
> All pods restart in the order of `learner -> follower -> leader` and the leader may change after the cluster restarts.

**How KubeBlocks restarts a cluster**

![Restart a cluster](../../../img/mysql_cluster_restart.png)

1. A user creates a restarting OpsRequest CR.
2. This CR passes the webhook validation.
3. The OpsRequest controller adds the restart annotation to the podTemplate of the StatefulSet/Deployment controller corresponding to the component.
4. The OpsRequest controller updates the cluster phase to `Rebooting`.
5. The component controller watches the StatefulSet/Deployment controller and pods.
6. When the component type is Stateful/Stateless, the Kubernetes StatefulSet/Deployment controller applies a rolling update to pods. When the component type is Consensus/ReplicationSet, the component controller applies the restarting operation to pods. 
7. When restarting is completed, the component controller updates the cluster CR component to `Running`.
8. The cluster controller watches the component phase changes and when all components are `Running`, the cluster controller updates the cluster phase to `Running`.
9. The OpsRequest controller reconciles the status when the cluster component status changes.

***Steps:***

1. Restart a cluster.
  You can use `kbcli` or create an OpsRequest to restart a cluster.
  
   **Option 1.** (Recommended) Use `kbcli`.
   
   Configure the values of `component-names` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.
   ```bash
   kbcli cluster restart NAME --component-names="mysql" \
   --ttlSecondsAfterSucceed=30
   ```
   - `component-names` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live after the restarting succeeds.

   **Option 2.** Create an OpsRequest.

   Run the command below to apply the restarting to a cluster. 
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterRef: mysql-cluster
     type: Restart 
     restart:
     - componentName: mysql
   EOF
   ```
2. Validate the restarting.
   Run the command below to check the cluster status to check the restarting status.
   ```bash
   kbcli cluster list <name>
   ```
   - STATUS=Updating: means the cluster is restarting.
   - STATUS=Running means the cluster is restarted.
   
   ***Example***

     ```bash
     kbcli cluster list mysql-cluster
     >
     NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
     mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
     ```
