---
title: Restart MongoDB cluster
description: How to restart a MongoDB cluster
sidebar_position: 4
sidebar_label: Restart
---

# Restart MongoDB cluster
You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

All pods restart in the order of learner -> follower -> leader and the leader may change after the cluster restarts.

:::

## Steps

1. Restart a cluster with `kbcli cluster restart` command and input the cluster name again.

  ```
  kbcli cluster restart mongodb-cluster
  Please type the name again(separate with white space when more than one): mongodb-cluster
OpsRequest mongodb-cluster-restart-pzsbj created successfully, you can view the progress:
        kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n default
  ```
  
  
2. Validate the restarting with the request code randomly generated, in this guide, it is `pzsbj`, see step 1.

  ```
   kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n default
  ```