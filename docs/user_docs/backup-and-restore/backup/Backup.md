---
title: Backup
description: How to back up a cluster
keywords: [backup, backup policy, manual backup, automatic backup]
sidebar_position: 4
sidebar_label: Backup
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

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

By default, all the backups are stored in the default global repository. You can execute the following command to view all BackupRepos. When the DEFAULT field is true, the BackupRepo is the default BackupRepo.

```bash
# View BackupRepo
kbcli backuprepo list
```

## BackupPolicy

After creating a database cluster, an automatic backup policy (BackupPolicy) is created for databases that support backup. Execute the following command to view the backup policy of the cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
$ kbcli cluster list-backup-policy mysql-cluster
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
$ kbcli cluster describe-backup-policy mysql-cluster
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
 $ kubectl get backuppolicy mysql-cluster-mysql-backup-policy -o yaml
```

</TabItem>

</Tabs>

For a MySQL cluster, it supports two default backup methods: `xtrabackup` and `volume-snapshot`. The former uses the backup tool `xtrabackup` to backup MySQL data to an object storage, while the latter utilizes the volume snapshot capability of cloud storage to backup data through snapshots. When creating a backup, you can specify which backup method to use.

## Manual backup

KubeBlocks supports manual backups. The following command uses the `xtrabackup` backup method to create a backup named `mybackup`:

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# Create a backup
$ kbcli cluster backup mysql-cluster --name mybackup --method xtrabackup
Backup mybackup created successfully, you can view the progress:
        kbcli cluster list-backups --name=mybackup -n default
        
# View the backup
$ kbcli cluster list-backups --name=mybackup -n default
NAME       NAMESPACE   SOURCE-CLUSTER   METHOD       STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION
mybackup   default     mysql-cluster    xtrabackup   Completed   4426858      2m8s       Oct 30,2023 15:19 UTC+0800   Oct 30,2023 15:21 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
# Create a backup
$ kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
spec:
  backupMethod: xtrabackup
  backupPolicyName: mysql-cluster-mysql-backup-policy
EOF

# View the backup
$ kubectl get backup mybackup
NAME       POLICY                              METHOD       REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mysql-cluster-mysql-backup-policy   xtrabackup   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

</TabItem>

</Tabs>

To create a backup using the snapshot, the backupMethod in the YAML configuration file or the --method field in the kbcli command should be set to volume-snapshot.

:::caution

When creating backups using snapshots, ensure that the storage used supports the snapshot feature; otherwise, the backup may fail.

Backups created manually using kubectl or kbcli will not be automatically deleted. You need to manually delete them.

:::

## Automatic backup

KubeBlocks supports configuring automatic backups for clusters.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Configure automatic backups with kbcli:

```powershell
$ kbcli cluster update mysql-cluster --backup-enabled=true \
--backup-method=xtrabackup --backup-repo-name=my-repo \
--backup-retention-period=7d --backup-cron-expression="0 18 * * *"
```

- `--backup-enabled` indicates whether to enable automatic backups.
- `--backup-method` specifies the backup method. You can use the `kbcli cluster describe-backup-policy mysql-cluster` command to view the supported backup methods.
- `--backup-repo-name` specifies the name of the backupRepo.
- `--backup-retention-period` specifies the retention period for backups, which is 7 days in the example.
- `--backup-cron-expression` specifies the backup schedule using a cron expression in UTC timezone. Refer to https://en.wikipedia.org/wiki/Cron for the expression format.

</TabItem>

<TabItem value="kubectl" label="kubectl">

Modify the backup field with kubectl as follows:

```go
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # Enable automatic backups
    enabled: true
    # UTC timezone, the example below stands for 2 A.M. every Monday
    cronExpression: 0 18 * * *
    # Use xtrabackup for backups. If your storage supports snapshot, you can change it to volume-snapshot
    method: xtrabackup
    # Retention period for a backup set
    retentionPeriod: 7d
    # BackupRepo
    repoName: my-repo
```

</TabItem>

</Tabs>
  
After enabling automatic backups, execute the following command to check if a CronJob object has been created:

```bash
$ kubectl get cronjob
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mysql-cluster-default-xtrabackup   0 18 * * *   False     0        <none>          57m
```

You can also execute the following command to view cluster information, where the `Data Protection:` section displays the configuration details of automatic backups.

```bash
$ kbcli cluster describe mysql-cluster
...
Data Protection:
BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION
my-repo       Enabled       0 18 * * *        xtrabackup      7d
```
