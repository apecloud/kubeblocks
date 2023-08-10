---
title: Restore data from backup set
description: How to restore data from backup set
keywords: [backup and restore, restore, backup set]
sidebar_position: 6
sidebar_label: Restore
---

# Restore data from backup set

1. View the log file backup to view the backup set.

   ```bash
   kbcli cluster list-backups
   ```

2. Specify a backup set to restore data.

   ```bash
   kbcli cluster restore new-mysql-cluster --backup backup-default-mysql-cluster-20230418124113
   ```

3. View this new cluster and make sure it is `Running`.

   ```bash
   kbcli cluster list new-mysql-cluster
   ```
