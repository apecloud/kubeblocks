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

You can configure the encryption key using `--set` with the helm install and helm upgrade commands. For example:

```shell
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
--set dataProtection.encryptionKey="your key"
```

## Create cluster

Prepare a cluster for testing the backup and restore function. The following instructions use a MySQL cluster named "mycluster" as an example.

By default, all the backups are stored in the default global repository. You can execute the following command to view all BackupRepos. When the `DEFAULT` field is `true`, the BackupRepo is the default BackupRepo.

```bash
# View BackupRepo
kubectl get backuprepo
```

## View BackupPolicy

After creating a database cluster, a BackupPolicy is created automatically for databases that support backup. Execute the following command to view the BackupPolicy of the cluster.

```bash
kubectl get backuppolicy | grep mycluster
>
mycluster-mysql-backup-policy                            Available   35m
mycluster-mysql-backup-policy-hscale                     Available   35m
```

The backup policy includes the backup methods supported by the cluster. Execute the following command to view the backup methods.

```bash
kubectl get backuppolicy mycluster-mysql-backup-policy -o yaml
```

For a MySQL cluster, two default backup methods are supported: `xtrabackup` and `volume-snapshot`. The former uses the backup tool `xtrabackup` to backup MySQL data to an object storage, while the latter utilizes the volume snapshot capability of cloud storage to backup data through snapshots. When creating a backup, you can specify which backup method to use.
