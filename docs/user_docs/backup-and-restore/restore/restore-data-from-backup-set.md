---
title: Restore from backup set
description: How to restore data from backup set
keywords: [backup and restore, restore, backup set]
sidebar_position: 6
sidebar_label: Restore from backup set
---

# Restore data from backup set

KubeBlocks supports restoring data from a backup set.

## Option 1. Restore by kbcli cluster restore command

1. View the backup set.

   ```bash
   kbcli cluster list-backups
   ```

2. Specify a new cluster and a backup set to restore data.

   ```bash
   kbcli cluster restore new-mysql-cluster --backup backup-default-mysql-cluster-20230418124113
   ```

3. View this new cluster and make sure it is `Running`.

   ```bash
   kbcli cluster list new-mysql-cluster
   ```

## Option 2. Restore by kbcli cluster create command

```bash
kbcli cluster create --backup backup-default-mycluster-20230616190023
```
