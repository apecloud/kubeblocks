---
title: Restart an ApeCloud MySQL Cluster
description: How to restart an ApeCloud MySQL Cluster
keywords: [apecloud mysql, restart, restart an apecloud mysql cluster]
sidebar_position: 4
sidebar_label: Restart
---

# Restart an ApeCloud MySQL Cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

All pods restart in the order of learner -> follower -> leader and the leader may change after the cluster restarts.

:::

## Steps

1. Create an OpsRequest to restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: mysql
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo
   >
   NAME                READY   STATUS        RESTARTS   AGE
   mycluster-mysql-0   4/4     Running       0          5m32s
   mycluster-mysql-1   4/4     Running       0          6m36s
   mycluster-mysql-2   3/4     Terminating   0          7m37s

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

   If an error occurs, you can troubleshoot with `kubectl describe` command to view the events of this operation.
