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

Restarting a Redis cluster triggers a concurrent restart and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.

   You can create an OpsRequest to restart a cluster.
  
   Run the command below to restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterRef: mycluster
     type: Restart 
     restart:
     - componentName: redis
   EOF
   ```

2. Validate the restart operation.

   Check the cluster status to identify the restart status.

   ```bash
   kubectl get cluster mycluster
   ```

   - STATUS=Restarting: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

   ***Example***

   ```bash
   kubectl get cluster mycluster
   >
   NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
   mycluster   redis                redis-7.0.6    Delete               Running   4d18h
   ```
