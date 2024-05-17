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

   You can use `kbcli` or create an OpsRequest to restart a cluster.
  

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart NAME --components="postgresql" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

  

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list <name>
   ```

   ***Example***

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME         NAMESPACE   CLUSTER-DEFINITION          VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   pg-cluster   default     postgresql-cluster          postgresql-14.7.0   Delete               Running   Mar 03,2023 18:28 UTC+0800
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

