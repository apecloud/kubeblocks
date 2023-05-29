---
title: Restart MySQL cluster
description: How to restart a MySQL cluster
keywords: [mysql, restart, restart a cluster]
sidebar_position: 4
sidebar_label: Restart
---

# Restart MySQL cluster

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
   kbcli cluster restart <name> --components="mysql" \
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
     clusterRef: mysql-cluster
     type: Restart 
     restart:
     - componentName: mysql
   EOF
   ```

2. Check the cluster status to validate the restarting.

   ```bash
   kbcli cluster list mysql-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
   mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
   ```

   - STATUS=Restarting: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
