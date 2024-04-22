---
title: Scheduled backup
description: How to back up databases by schedule
keywords: [backup and restore, schedule, automatic backup, scheduled backup]
sidebar_position: 3
sidebar_label: Scheduled backup
---

# Scheduled backup

KubeBlocks supports configuring scheduled backups for clusters.

Modify the backup field with kubectl as follows. Here "mycluster" is the name of the sample cluster. 

```bash
kubectl edit cluster -n default mycluster
>
spec:
  ...
  backup:
    # Whether to enable automatic backups
    enabled: true
    # UTC timezone, the example below stands for 2 A.M. every Monday
    cronExpression: 0 18 * * *
    # Use xtrabackup for backups. If your storage supports snapshot, you can change it to volume-snapshot
    method: xtrabackup
    # Whether to enable PITR
    pitrEnabled: false
    # Retention period for a backup set
    retentionPeriod: 7d
    # BackupRepo
    repoName: my-repo
```

In the above YAML file, you can set whether to enable automatic backups and PITR as needed, and also specify backup methods, repo names, retention periods, etc.

After the scheduled backup is enabled, execute the following command to check if a CronJob object has been created:

```bash
kubectl get cronjob
>
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mycluster-default-xtrabackup       0 18 * * *   False     0        <none>          57m
```
