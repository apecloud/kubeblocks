---
title: Restart Kafka cluster
description: How to restart a Kafka cluster
keywords: [kafka, restart]
sidebar_position: 4
sidebar_label: Restart
---


# Restart a Kafka cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

Restarting a Kafka cluster triggers a concurrent restart and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.

  ```bash
  kubectl apply -f - <<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: ops-restart
  spec:
    clusterName: 
    type: Restart 
    restart:
    - componentName: kafka
  EOF
  ```

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

