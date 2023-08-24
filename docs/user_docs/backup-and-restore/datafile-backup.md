---
title: Datafile backup and restore
description: How to back up databases by datafiles
keywords: [backup and restore, datafile]
sidebar_position: 3
sidebar_label: Datafile
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Datafile backup

## Before you start

Prepare a cluster for testing the backup and restore function. The following instructions use MySQL as an example.

1. Create a cluster.

   ```bash
   kbcli cluster create mysql mysql-cluster
   ```

2. View the backup policy.

   ```bash
   kbcli cluster list-backup-policy mysql-cluster
   ```

   By default, all the backups are stored in the default global repository but you can specify a new repository by [editing the BackupPolicy resource](./backup-repo.md#optional-change-the-backup-repository-for-a-cluster).

## Create backup

The datafile backup of KubeBlocks supports two options: backup tool and snapshot backup.

### Backup tool

Both kbcli and kubectl are supported.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. View the cluster status and make sure the cluster is `Running`.

   ```bash
   kbcli cluster list mysql-cluster
   ```

2. Create a backup.

   ```bash
   kbcli cluster backup mysql-cluster --type=datafile
   ```

3. View the backup set.

   ```bash
   kbcli cluster list-backups mysql-cluster
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Run the command below to create a datafile backup named `mybackup`.

```bash
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
spec:
  backupPolicyName: mycluster-mysql-backup-policy
  backupType: datafile
EOF
```

</TabItem>

</Tabs>

### Snapshot backup

Both kbcli and kubectl are supported.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. View the cluster status and make sure the cluster is `Running`.

   ```bash
   kbcli cluster list mysql-cluster
   ```

2. Create a snapshot backup.

   ```bash
   kbcli cluster backup mysql-cluster --type=snapshot
   ```

3. View the backup set to check whether the backup is successful.

   ```bash
   kbcli cluster list-backups mysql-cluster
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Run the command below to create a snapshot backup named `mysnapshot`.

```bash
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mysnapshot
  namespace: default
spec:
  backupPolicyName: mycluster-mysql-backup-policy
  backupType: snapshot
EOF
```

</TabItem>

</Tabs>
