---
title: Configure BackupPolicy
description: How to configure BackupPolicy
keywords: [backup, backup policy]
sidebar_position: 2
sidebar_label: Configure BackupPolicy
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure BackupPolicy

## Configure encryption key

To ensure that the restored cluster can access the data properly, KubeBlocks encrypts the cluster's credentials during the backup process and securely stores it in the Annotation of the Backup object. Therefore, to protect your data security, it is strongly recommended to carefully assign Get/List permissions for backup objects and specify an encryption key during the installation or upgrade of KubeBlocks. These measures will help ensure the proper protection of your data.

```shell
kbcli kubeblocks install --set dataProtection.encryptionKey="your key"
or
kbcli kubeblocks upgrade --set dataProtection.encryptionKey="your key"
```

## Create cluster

Prepare a cluster for testing the backup and restore function. The following instructions use MySQL as an example.

```shell
# Create a MySQL cluster
kbcli cluster create mysql mysql-cluster

# View backupPolicy
kbcli cluster list-backup-policy mysql-cluster
>
NAME                                       DEFAULT   CLUSTER         CREATE-TIME                  STATUS      
mysql-cluster-mysql-backup-policy          true      mysql-cluster   May 23,2023 19:53 UTC+0800   Available   
mysql-cluster-mysql-backup-policy-hscale   false     mysql-cluster   May 23,2023 19:53 UTC+0800   Available
```

By default, all the backups are stored in the default global repository. You can execute the following command to view all BackupRepos. When the `DEFAULT` field is `true`, the BackupRepo is the default BackupRepo.

```bash
# View BackupRepo
kbcli backuprepo list
```

## View BackupPolicy

After creating a database cluster, a BackupPolicy is created automatically for databases that support backup. Execute the following command to view the BackupPolicy of the cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list-backup-policy mysql-cluster
>
NAME                                       NAMESPACE   DEFAULT   CLUSTER         CREATE-TIME                  STATUS
mysql-cluster-mysql-backup-policy          default     true      mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
mysql-cluster-mysql-backup-policy-hscale   default     false     mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get backuppolicy | grep mysql-cluster
>
mysql-cluster-mysql-backup-policy                            Available   35m
mysql-cluster-mysql-backup-policy-hscale                     Available   35m
```

</TabItem>

</Tabs>

The backup policy includes the backup methods supported by the cluster. Execute the following command to view the backup methods.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster describe-backup-policy mysql-cluster
> 
kbcli cluster describe-backup-policy mysql-cluster
Summary:
  Name:               mysql-cluster-mysql-backup-policy
  Cluster:            mysql-cluster
  Namespace:          default
  Default:            true

Backup Methods:
NAME              ACTIONSET                           SNAPSHOT-VOLUMES
xtrabackup        xtrabackup-for-apecloud-mysql       false
volume-snapshot   volumesnapshot-for-apecloud-mysql   true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get backuppolicy mysql-cluster-mysql-backup-policy -o yaml
```

</TabItem>

</Tabs>

For a MySQL cluster, two default backup methods are supported: `xtrabackup` and `volume-snapshot`. The former uses the backup tool `xtrabackup` to backup MySQL data to an object storage, while the latter utilizes the volume snapshot capability of cloud storage to backup data through snapshots. When creating a backup, you can specify which backup method to use.
