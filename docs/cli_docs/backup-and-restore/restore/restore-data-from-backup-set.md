---
title: Restore data from backup set
description: How to restore data from backup set
keywords: [backup and restore, restore, backup set]
sidebar_position: 1
sidebar_label: Restore from backup set
---

# Restore data from backup set

KubeBlocks supports restoring clusters from backups with the following instructions.

1. View backups.

   For existing clusters, execute:

   ```shell
   kbcli cluster list-backups mysql-cluster
   ```

   If the cluster has been deleted, execute:

   ```bash
   kbcli dataprotection list-backups
   ```

2. Restore clusters from a specific backup.

    ```powershell
    # Restore new cluster
    kbcli cluster restore myrestore --backup mybackup
    >
    Cluster myrestore created

    # View the status of the restored cluster
    kbcli cluster list myrestore
    NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
    myrestore   default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Oct 30,2023 16:26 UTC+0800
    ```


3. Connect to the restored cluster for verification.

    Once the cluster status is `Running`, run the following command to connect to the cluster for verification:

    ```bash
    kbcli cluster connect myrestore
    ```
