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

1. Restart a cluster with `kbcli cluster restart` command and enter the cluster name again.

   ```bash
   kbcli cluster restart mongodb-cluster
   >
   OpsRequest mongodb-cluster-restart-pzsbj created successfully, you can view the progress:
         kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n default
   ```

2. Validate the restart operation.

   Check the cluster status to identify the restart status.

   ```bash
   kbcli cluster list mongodb-cluster
   >
   NAME                   NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   mongodb-cluster        default          mongodb                   mongodb-5.0        Delete                    Running        Apr 26,2023 12:50 UTC+0800
   ```

   - STATUS=Updating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
