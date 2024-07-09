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
  
   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart redis-cluster --components="redis" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restart operation.

   Check the cluster status to identify the restart status.

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6        Delete                    Running        Apr 10,2023 19:20 UTC+0800
   ```

   - STATUS=Updating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
