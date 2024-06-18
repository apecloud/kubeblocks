---
title: Configure BackupPolicy
description: How to configure BackupPolicy
keywords: [backup, backup policy]
sidebar_position: 2
sidebar_label: Configure BackupPolicy
---

# Configure BackupPolicy

## Configure encryption key

To ensure that the restored cluster can access the data properly, KubeBlocks encrypts the cluster's credentials during the backup process and securely stores it in the Annotation of the Backup object. Therefore, to protect your data security, it is strongly recommended to carefully assign Get/List permissions for backup objects and specify an encryption key during the installation or upgrade of KubeBlocks. These measures will help ensure the proper protection of your data.

KubeBlocks has integrated data encryption functionality for datasafed since v0.9.0. Currently, the supported encryption algorithms include `AES-128-CFB`, `AES-192-CFB`, and `AES-256-CFB`. This function allows backup data to be encrypted before being written to storage. The encryption key then will be used to encrypt connection passwords and also to back up data. You can reference existing keys or create different secret keys for database clusters according to actual needs.

### Reference an existing key

If the secret already exists, you can choose to directly reference it without setting the `dataProtection.encryptionKey`. KubeBlocks provides a quick way to reference an existing key for encryption.

Assuming there is a pre-defined secret named `dp-encryption-key` and a key `encryptionKey` inside it. For example, a secret created by this command.

```bash
kubectl create secret generic dp-encryption-key \
    --from-literal=encryptionKey='S!B\*d$zDsb='
```

And then you can reference it when installing or upgrading KubeBlocks.

```bash
kbcli kubeblocks install \
    --set dataProtection.encryptionKeySecretKeyRef.name="dp-encryption-key" \
    --set dataProtection.encryptionKeySecretKeyRef.key="encryptionKey"
# The above command is equivalent to:
# kbcli kubeblocks install --set dataProtection.encryptionKey='S!B\*d$zDsb='
```

### Create a new key

If you do not need to enable backup encryption by default, or if you need to use a separate `encryptionKey`, just create a Secret and manually enable backup encryption by following the steps below.

1. Create a Secret to store the encryption key.

    ```bash
    kubectl create secret generic backup-encryption \
    --from-literal=secretKey='your secret key'
    ```

2. Enable encryption.
Remember to reference the key created before.

    ```bash
    kubectl --type merge patch backuppolicy mysqlcluster-mysql-backup-policy \
    -p '{"spec":{"encryptionConfig":{"algorithm":"AES-256-CFB","passPhraseSecretKeyRef":{"name":"backup-encryption","key":"secretKey"}}}}'
    ```

    :::note

    Try using kbcli to simplify the process. 

    ```bash
     # enable encryption
     kbcli cluster edit-backup-policy <backup-policy-name> --set encryption.algorithm=AES-256-CFB --set encryption.passPhrase="SECRET!"
      
     # disable encryption
     kbcli cluster edit-backup-policy <backup-policy-name> --set encryption.disabled=true
    ```

    :::

Complete. Now you can perform backups and restores as usual.

:::note

The secret created in Step 1 should not be modified or deleted; otherwise, decryption of backups may fail.

:::

By default, the `encrytpionKey` is only used for encrypting the connection password, if you want to use it to encrypt backup data as well, add `--set dataProtection.enableBackupEncryption=true` to the above command. After that, all newly-created clusters are enabled for backup encryption by default.

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

```bash
kbcli cluster list-backup-policy mysql-cluster
>
NAME                                       NAMESPACE   DEFAULT   CLUSTER         CREATE-TIME                  STATUS
mysql-cluster-mysql-backup-policy          default     true      mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
mysql-cluster-mysql-backup-policy-hscale   default     false     mysql-cluster   Oct 30,2023 14:34 UTC+0800   Available
```


The backup policy includes the backup methods supported by the cluster. Execute the following command to view the backup methods.



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



For a MySQL cluster, two default backup methods are supported: `xtrabackup` and `volume-snapshot`. The former uses the backup tool `xtrabackup` to backup MySQL data to an object storage, while the latter utilizes the volume snapshot capability of cloud storage to backup data through snapshots. When creating a backup, you can specify which backup method to use.
