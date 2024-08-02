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

The pod role may change after the cluster restarts.

:::

## Steps

1. Restart a cluster.
  
   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart NAME --components="kafka" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list cluster-name
   >
   NAME    CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
   kafka   kafka                kafka-3.3.2    Delete               Running    19m
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.
