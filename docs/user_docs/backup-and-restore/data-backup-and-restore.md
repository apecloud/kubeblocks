---
title: Data file backup and restore
description: How to back up and restore databses by data files
keywords: [backup and restore, data file]
sidebar_position: 3
sidebar_label: Data file
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Data file backup and restore

## Before you start

Prepare a cluster for testing the backup and restore function. The following insstructions uses MySQL as an example.

1. Create a cluster.

   ```bash
   kbcli cluster create mysql mysql-cluster
   ```

2. View the backup policy.

   ```bash
   kbcli cluster list-backup-policy mysql-cluster
   ```

   By default, all the backups are stored in the default global repository. You can specify a new repository by editing the BackupPolicy resource.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster edit-backup-policy mysql-cluster --set="datafile.backupRepoName=my-repo"
   ```

   </TabItem>

   <TabItem value="kubectl" label="kubectl">

   ```bash
   kubectl edit backuppolicy mysql-cluster-mysql-backup-policy
   ...
   spec:
     datafile:
       ... 
       # Specify a backup repository name
       backupRepoName: my-repo
   ```

   </TabItem>

   </Tabs>

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

2. Create backup.

   ```bash
   kbcli cluster backup mysql-cluster --type=datafile
   ```

3. View the backup set.

   ```bash
   kbcli cluster list-backups mysql-cluster
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Run the command below to create a data backup named `mybackup`.

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

2. Create backup.

   ```bash
   kbcli cluster backup mysql-cluster --type=snapshot
   ```

3. View the backup set to check whether the backup is successful.

   ```bash
   kbcli cluster list-backups mysql-cluster
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Run the command below to create a data backup named `mysnapshot`.

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
