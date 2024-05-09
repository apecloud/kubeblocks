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
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: postgresql
   EOF
   ```

2. Validate the restarting.
