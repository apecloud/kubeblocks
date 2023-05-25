---
title: Restart PostgreSQL cluster
description: How to restart a PostgreSQL cluster
sidebar_position: 4
sidebar_label: Restart
---


# Restart PostgreSQL cluster
You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

All pods restart in the order of learner -> follower -> leader and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.
  You can use `kbcli` or create an OpsRequest to restart a cluster.
  
   **Option 1.** (**Recommended**) Use kbcli
   
   Configure the values of `component-names` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.
   ```bash
   kbcli cluster restart NAME --component-names="pg-replication" \
   --ttlSecondsAfterSucceed=30
   ```
   - `component-names` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of OpsRequest job after the restarting succeeds.

   **Option 2.** Create an OpsRequest

   Run the command below to apply the restarting to a cluster. 
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterRef: pg-cluster
     type: Restart 
     restart:
     - componentName: pg-replication
   EOF
   ```
2. Validate the restarting.
   Run the command below to check the cluster status to check the restarting status.
   ```bash
   kbcli cluster list <name>
   ```
   - STATUS=Updating: it means the cluster is restarting.
   - STATUS=Running: it means the cluster is restarted.
   
   ***Example***

     ```bash
     kbcli cluster list pg-cluster
     >
     NAME         NAMESPACE   CLUSTER-DEFINITION          VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
     pg-cluster   default     postgresql-cluster          postgresql-14.7.0   Delete               Running   Mar 03,2023 18:28 UTC+0800
     ```
