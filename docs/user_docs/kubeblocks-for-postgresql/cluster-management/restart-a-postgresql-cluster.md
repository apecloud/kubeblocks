---
title: Restart PostgreSQL cluster
description: How to restart a PostgreSQL cluster
keywords: [postgresql, restart]
sidebar_position: 4
sidebar_label: Restart
---


# Restart PostgreSQL cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

Restarting a PostgreSQL cluster triggers a concurrent restart and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
     namesapce: demo
   spec:
     clusterRef: mycluster
     type: Restart 
     restart:
     - componentName: postgresql
   EOF
   ```

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   ***Example***

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
   mycluster   postgresql           postgresql-14.8.0   Delete               Running   30m
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.
