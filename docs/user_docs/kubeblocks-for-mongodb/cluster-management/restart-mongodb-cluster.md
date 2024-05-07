---
title: Restart a MongoDB cluster
description: How to restart a MongoDB cluster
keywords: [mongodb, restart a cluster]
sidebar_position: 4
sidebar_label: Restart
---

# Restart MongoDB cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

## Steps

1. Restart a cluster.

   You can use `kbcli` or create an OpsRequest to restart a cluster.

   Run the command below to restart a cluster.

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
     - componentName: mongodb
   EOF
   ```

2. Check the cluster status to validate the restarting.

   ```bash
   kubectl get cluster mycluster
   >
   NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
   mycluster   mongodb              mongodb-5.0   Delete               Running   27m
   ```

   - STATUS=Restarting: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
