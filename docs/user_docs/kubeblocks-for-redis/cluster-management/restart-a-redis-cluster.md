---
title: Restart a Redis cluster
description: How to restart a Redis cluster
keywords: [redis, restart]
sidebar_position: 4
sidebar_label: Restart
---

# Restart a Redis cluster
You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

All pods restart in the order of learner -> follower -> leader and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.
   
   You can use `kbcli` or create an OpsRequest to restart a cluster.
  
   **Option 1.** (**Recommended**) Use kbcli
   
   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.
   ```bash
   kbcli cluster restart redis-cluster --components="redis" \
   --ttlSecondsAfterSucceed=30
   ```
   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

   **Option 2.** Create an OpsRequest

   Run the command below to apply the restarting to a cluster. 
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterRef: redis-cluster
     type: Restart 
     restart:
     - componentName: redis
   EOF
   ```
2. Validate the restarting.
   
   Check the cluster status to identify the restarting status.

   ```bash
   kbcli cluster list <name>
   ```
   - STATUS=Restarting: it means the cluster restarting is in progress.
   - STATUS=Running: it means the cluster has been restarted.
   
   ***Example***

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.x        Delete                    Running        Apr 10,2023 19:20 UTC+0800
   ```
