---
title: Data file backup and restore for PostgreSQL
description: How to back up and restore PostgreSQL by data files
keywords: [backup and restore, postgresql, data file]
sidebar_position: 2
sidebar_label: Data file
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Data file backup and restore

For KubeBlocks, configuring backup and restoring data is simple with 3 steps. Configure storage path and backup policy, create a backup (manually or scheduled), and then you can restore data backed up.

## Configure target storage path

Currently, KubeBlocks backs up and restores data on the storage path predefined.

<Tabs>
<TabItem value="S3" label="AWS S3" default>

Enable CSI-S3 and fill in the values based on your actual environment.

```bash
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

helm install csi-s3  kubeblocks/csi-s3 --version=0.5.0 \
--set secret.accessKey=<your_accessKey> \
--set secret.secretKey=<your_secretKey> \
--set storageClass.singleBucket=<s3_bucket>  \
--set secret.endpoint=https://s3.<region>.amazonaws.com.cn \
--set secret.region=<region> -n kb-system

# CSI-S3 installs a daemonSet pod on all nodes and you can set tolerations to install daemonSet pods on the specified nodes
--set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
```

:::note

Endpoint format:

* China: `https://s3.<region>.amazonaws.com.cn`
* Other countries/regions: `https://s3.<region>.amazonaws.com`

:::

</TabItem>

<TabItem value="OSS" label="OSS">

```bash
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

helm install csi-s3 kubeblocks/csi-s3 --version=0.5.0 \
--set secret.accessKey=<your_access_id> \
--set secret.secretKey=<your_access_secret> \
--set storageClass.singleBucket=<bucket_name>  \
--set secret.endpoint=https://oss-<region>.aliyuncs.com \
 -n kb-system

# CSI-S3 installs a daemonSet pod on all nodes and you can set tolerations to install daemonSet pods on the specified nodes
--set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
```

</TabItem>

<TabItem value="minIO" label="MinIO">

1. Install minIO.

   ```bash
   helm upgrade --install minio oci://registry-1.docker.io/bitnamicharts/minio --set persistence.enabled=true,persistence.storageClass=csi-hostpath-sc,persistence.size=100Gi,defaultBuckets=backup
   ```

2. Install CSI-S3.

   ```bash
   helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

   helm install csi-s3 kubeblocks/csi-s3 --version=0.5.0 \
   --set secret.accessKey=<ROOT_USER> \
   --set secret.secretKey=<ROOT_PASSWORD> \
   --set storageClass.singleBucket=backup  \
   --set secret.endpoint=http://minio.default.svc.cluster.local:9000 \
    -n kb-system

   # CSI-S3 installs a daemonSet pod on all nodes and you can set tolerations to install daemonSet pods on the specified nodes
   --set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
   ```

</TabItem>
</Tabs>

You can configure a global backup storage to make this storage the default backup destination path of all new clusters. But currently, the global backup storage cannot be synchronized as the backup destination path of created clusters.

Set the backup policy with the following command.

```bash
kbcli kubeblocks config --set dataProtection.backupPVCName=kubeblocks-backup-data \
--set dataProtection.backupPVCStorageClassName=csi-s3 -n kb-system

# dataProtection.backupPVCName: PersistentVolumeClaim Name for backup storage
# dataProtection.backupPVCStorageClassName: StorageClass Name
# -n kb-system: namespace where KubeBlocks is installed
```

:::note

* If there is no PVC, the system creates one automatically based on the configuration.
* It takes about 1 minute to make the configuration effective.

:::

## Create backup

**Option 1. Manually Backup**

1. Check whether the cluster is running.

   ```bash
   kbcli cluster list pg-cluster
   ```

2. Create a backup for this cluster.

   ```bash
   kbcli cluster backup pg-cluster --type=datafile
   ```

3. View the backup set.

   ```bash
   kbcli cluster list-backups pg-cluster 
   ```

**Option 2. Enable scheduled backup**

```bash
kbcli cluster edit-backup-policy pg-cluster-postgresql-backup-policy
>
spec:
  ...
  schedule:
    baseBackup:
      # UTC time zone, the example below means 2 a.m. every Monday
      cronExpression: "0 18 * * 0"
      # Enable this function
      enable: true
      # Select the basic backup type, available options: snapshot and snapshot
      # This example selects datafile as the basic backup type
      type: datafile
```

## Restore data from backup

1. Restore data from the backup.

   ```bash
   kbcli cluster restore new-pg-cluster --backup backup-default-pg-cluster-20230418124113
   ```

2. View this new cluster.

   ```bash
   kbcli cluster list new-pg-cluster
   ```
