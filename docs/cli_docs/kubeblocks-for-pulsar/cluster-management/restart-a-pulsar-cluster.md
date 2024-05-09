---
title: Restart Pulsar cluster
description: How to restart a Pulsar cluster
keywords: [pulsar, restart]
sidebar_position: 4
sidebar_label: Restart
---


# Restart a Pulsar cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

Restarting a Kafka cluster triggers a concurrent restart and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.

  You can use `kbcli` or create an OpsRequest to restart a cluster.
  
   **Option 1.** (**Recommended**) Use kbcli

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart NAME --components="pulsar" \
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
    clusterRef: pulsar
    type: Restart 
    restart:
    - componentName: pulsar
  EOF
  ```

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list <name>
   ```

   ***Example***

   ```bash
   kbcli cluster list kafka
   >
   NAME    CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS     AGE
   pulsar      pulsar                                pulsar-2.11    Delete                               Running    19m
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

