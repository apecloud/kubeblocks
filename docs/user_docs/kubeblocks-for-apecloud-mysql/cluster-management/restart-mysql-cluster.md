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

1. Create an OpsRequest to restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: mysql
   EOF
   ```

2. Check the cluster status to validate the restarting.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME             NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
   mycluster        demo             apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        April 25,2024 17:25 UTC+0800
   ```

   - STATUS=Restarting: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
