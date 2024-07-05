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

1. Restart a cluster.  

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster --components="mysql" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Check the cluster status to validate the restarting.

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   default     mysql                mysql-8.0.33   Delete               Updating   Jul 05,2024 19:01 UTC+0800
   ```

   - STATUS=Updating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
