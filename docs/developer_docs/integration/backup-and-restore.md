---
title: Backup and restore
description: Create backup and restore
keywords: [add-on, backup, restore]
sidebar_position: 3
sidebar_label: Backup and restore
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Backup and restore

This tutorial takes Oracle MySQL as an example and introduces how to create backups and restore data in KubeBlocks. The full PR can be found at [Learn KubeBlocks Add-on](https://github.com/apecloud/learn-kubeblocks-addon/tree/main/tutorial-2-backup-restore/).

Different classification results in different types of backups, including volume snapshot backup and file backup, data backup and log backup, full backup and incremental backup, as well as scheduled backup and on-demand backup, which differ in terms of their methods, contents, volumes, and timing.

This tutorial illustrates how to realize the most frequently used snapshot backup and file backup in KubeBlocks.

- Snapshot backup relies on the volume snapshot capability of Kubernetes.
- File backup relies on backup tools provided by database engines.

Now take a quick look at the basic concepts of KubeBlocks in the table below, which also are elaborated in the following tutorial.

:paperclip: Table 1. Terminology

| Term | Description | Scope |
| :--- | :---------- | :---- |
| Backup | Backup object <br /> It defines the entity to be backed up. | Namespace |
| BackupPolicy | Backup policy <br /> It defines the policy for each backup type, such as scheduling, retention time, and tools. | Namespace |
| BackupTool | Backup tool <br /> It is the carrier of backup tools in KubeBlocks and should realize the backup and restoration logic of corresponding tools. | Cluster |
| BackupPolicyTemplate | Template of backup policy <br /> It is the bridge between the backup and ClusterDefinition. When creating a cluster, KubeBlocks automatically generates a default backup policy for each cluster according to BackupPolicyTemplate. | Cluster |

## Before you start

- Finish the configuration in [Add an add-on to KubeBlocks](./how-to-add-an-add-on.md).
- Grasp the basics of K8s concepts, such as Pod, PVC, PV, VolumeSnapshot, etc.

## Step 1. Prepare environment

1. Install CSI Driver.

   Since volume snapshot is only available for CSI Drivers, make sure your Kubernetes is properly configured.

   - For the localhost, you can quickly install `csi-host-driver` by KubeBlocks add-on.

     ```bash
     kbcli addon enable csi-hostpath-driver
     ```

   - For a cloud environment, configure the corresponding CSI Driver based on your environment.

2. Set the `storageclass` to the default to make it easier to create clusters.

   ```bash
   kubectl get sc
   >
   NAME                        PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   csi-hostpath-sc (default)   hostpath.csi.k8s.io     Delete          WaitForFirstConsumer   true                   35s
   ```

## Step 2. Specify a volume type

Specify a volume type in the ClusterDefinition and it is required.

```yaml
  componentDefs:
    - name: mysql-compdef
      characterType: mysql
      workloadType: Stateful
      service:
        ports:
          - name: mysql
            port: 3306
            targetPort: mysql
      volumeTypes:
        - name: data
          type: data
```

`volumeTypes` is used to specify volume type and name.

There are mainly two kinds of volume types (`volumeTypes.type`):

- `data`: Data information
- `log`: Log information

KubeBlocks supports different backup methods for data and logs. In this tutorial, only data volume information is configured.

## Step 3. Add backup configuration

Prepare `BackupPolicyTemplate.yml` and `BackupTool.yml` to add the backup configuration.

### BackupPolicy template

It is a template of backup policy, and covers:

1. Which cluster components to back up
2. Whether the backups are scheduled
3. How to set up snapshot backup
4. How to set up file backup

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: oracle-mysql-backup-policy-template
  labels:
    clusterdefinition.kubeblocks.io/name: oracle-mysql # Specify scope through labels (Required)
spec:
  clusterDefinitionRef: oracle-mysql  # Specify the scope, indicating which ClusterDef generates the cluster
  backupPolicies:
  - componentDefRef: mysql-compdef    # Specify the scope, indicating which component is involved
    schedule:                         # Specify the timing of scheduled backups and startup status
      snapshot:
        enable: true                  # Enable scheduled snapshot backups
        cronExpression: "0 18 * * *"
      datafile:                       # Disable scheduled datafile backups
        enable: false
        cronExpression: "0 18 * * *"        
    snapshot:                         # Snapshot backup, which keeps the latest 5 versions by default
      backupsHistoryLimit: 5
    datafile:                         # Datafile backup which depends on backup tools
      backupToolName: oracle-mysql-xtrabackup
```

If a scheduled task is enabled, KubeBlocks creates a CronJob in the background.

After a new cluster is created, KubeBlocks discovers the corresponding template name by `clusterdefinition.kubeblocks.io/name` and creates the corresponding BackupPolicy.

:::note

If you have added `BackupPolicyTemplate` but there is no default BackupPolicy for the new cluster, check whether the following requirements:

1. Whether `ClusterDefinitionRef` is correct.
2. Whether the `BackupPolicyTemplate` label is correct.
3. Whether there are multiple BackupPolicyTemplates. If yes, mark one as the default template using annotations.

   ```yaml
     annotations:
      dataprotection.kubeblocks.io/is-default-policy-template: "true"
   ```

:::

### BackupTool

:::note

`BackupTool` mainly serves datafile backup. If you only need snapshot backups, there is no need to configure BackupTool.

:::

`BackTool.yml` describes the detailed execution logic of a backup tool and mainly serves datafile backup. It should cover:

1. Image of backup tools
2. Scripts of backup
3. Scripts of restore

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupTool
metadata:
  name: oracle-mysql-xtrabackup
  labels:
spec:
  image: docker.io/perconalab/percona-xtrabackup:8.0.32  # Back up via xtrabackup
  env:                         # Inject the name of dependent environment variables
    - name: DATA_DIR
      value: /var/lib/mysql
  physical:
    restoreCommands:           # Restore commands
      - sh
      - -c
      ...
  backupCommands:             # Backup commands
    - sh
    - -c
    ...
```

The configuration of `BackupTool` is closely related to the tools used.

For example, if you back up via Percona Xtrabackup, you need to fill in scripts in `backupCommands` and `restoreCommands`.

## Step 4. Back up and restore a cluster

With everything ready, try to back up a cluster and restore data to a new cluster.

### 4.1 Create a cluster

Since `BackupPolicyTemplate` has been added, after a cluster is created, KubeBlocks can discover the backup policy and create a `BackupPolicy` for this cluster.

1. Create a cluster.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster create mycluster --cluster-definition oracle-mysql 
   ```

   </TabItem>

   <TabItem value="Helm" label="Helm">

   ```bash
   helm install mysql ./tutorial-2-backup-restore/oracle-mysql
   ```

   </TabItem>

   </Tabs>

2. View the backup policy of this cluster.

   ```bash
   kbcli cluster list-backup-policy mycluster
   ```

### 4.2 Snapshot backups

```bash
kbcli cluster backup mycluster --type snapshot
```

`type` specifies the backup type, indicating whether it is a snapshot or datafile.

If there are multiple backup policies, specify it with the `--policy` flag.

### 4.3 Datafile backups

KubeBlocks supports backup to local storage and cloud object storage. The following is an example of backing up to your localhost.

1. Modify BackupPolicy and specify the PVC name.

   As shown below in `spec.datafile.persistentVolumeClaim.name`, specify the PVC name.

   ```yaml
     spec:
       datafile:
         backupToolName: oracle-mysql-xtrabackup
         backupsHistoryLimit: 7
         persistentVolumeClaim:
           name: mycluster-backup-pvc
           createPolicy: IfNotPresent
           initCapacity: 20Gi
   ```

2. Set `--type` to `datafile`.

   ```bash
   kbcli cluster backup mycluster  --type datafile
   ```

### 4.4 Create a cluster from backups

1. Check the backups.

   ```bash
   kbcli cluster list-backups
   ```

2. Select a backup and create a cluster.

   ```bash
   kbcli cluster restore <clusterName> --backup <backup-name>
   ```

And a new cluster is created.

:::caution

It should be noted that some databases only create the root account and password during the first initialization.

Therefore, although a new root account and password are created when restoring a cluster from backups, they are not effective. You still need to log in with the root account and password of the original cluster.

:::

## Reference

- For more details on the backup and restore function of KubeBlocks, refer to [Backup and Restore](./../../user_docs/backup-and-restore/introduction.md).

## Appendix

### A.1 Cluster data protection policies

KubeBlocks provides various data protection policies for stateful clusters, each offering various data options. Try the following scenarios:

1. If you delete a cluster using `kbcli cluster delete`, will the backup still be available?
2. If you change the `terminationPolicy` of a cluster to `WipeOut` and then delete it, will the backup still be available?
3. If you change the `terminationPolicy` of a cluster to `DoNotTerminate` and then delete it, what will happen?

:::note

Refer to the data protection policies of KubeBlocks via [Termination Policy](./../../user_docs/kubeblocks-for-apecloud-mysql/cluster-management/delete-mysql-cluster.md#termination-policy).

:::

### A.2 Monitor backup progress

In [Step 4](#step-4-back-up-and-restore-a-cluster), you have created a backup using the backup subcommand.

```bash
kbcli cluster backup mycluster  --type snapshot
```

A new backup object is generated and you can view the progress by running the `describe-backup` subcommand.

```bash
kbcli cluster describe-backup <your-back-up-name>
```
