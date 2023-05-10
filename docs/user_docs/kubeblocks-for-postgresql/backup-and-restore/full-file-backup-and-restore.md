---
title: Full file backup and restore for PostgreSQL
description: How to back up and restore full files for PostgreSQL
keywords: [full file backup and restore, postgresql]
sidebar_position: 3
sidebar_label: Full file backup and restore
---

# Full file backup and restore

Follow the steps below to perform the full file backup and restore of a cluster.

## Back up the storage source

Currently, the KubeBlocks backup and restore rely on the Kubernetes PersistentVolume and PersistentVolumeClaim and store the backup data on the specified PVC. The tables below list whether a PersistentVolume access mode is supported.

:paperclip: Table 1. Backup

| Access Mode      | Same backup source <br />(All clusters share a PVC) | One PVC for one cluster |
| :----------      | :----------------- | :---------------------- |
| ReadWriteMany    | Yes                | Yes                     |
| ReadWriteOnce    | NA                 | Yes (only for test)     |
| ReadOnlyMany     | NA                 | Yes (only for test)     |
| ReadWriteOncePod | NA                 | NA                      |

:paperclip: Table 2. Restore

| Access Mode      | One-replica cluster | Multiple-replicas cluster |
| :----------      | :-----------------  | :----------------------   |
| ReadWriteMany    | Yes                 | Yes                       |
| ReadWriteOnce    | Yes (only for test) | NA                        |
| ReadOnlyMany     | Yes (only for test) | Yes (only for test)       |
| ReadWriteOncePod | NA                  | NA                        |

If you use a local volume, the pods of backup and restore should be on the same node and can only be used for a test. Note that ReadWriteOnce pod is not supported.

If the `volumeBindingMode` of the StorageClass for the local test is `WaitForFirstConsumer`,  manually create a PV and PVC and wait for the PVC to be bound successfully.

### Use S3 as the backup storage source

1. Enable CSI-S3 and fill in the values based on your actual environment.

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

2. Create a PVC.

   ```yaml
   kubectl apply -f -<< EOF
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: backup-data-s3
   spec:
     accessModes:
     - ReadWriteMany
     resources:
       requests:
         storage: 10Gi
     storageClassName: csi-s3
   EOF
   ```

### Use OSS as the backup storage source

```bash
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

helm install csi-s3 kubeblocks/csi-s3 --version=0.5.0 \
--set secret.accessKey=<your_access_id> \
--set secret.secretKey=<your_access_secret> \
--set storageClass.singleBucket=<bucket_name>  \
--set secret.endpoint=https://oss-<region>.aliyuncs.com \
--set storageClass.mounter=s3fs,storageClass.mountOptions="" \
 -n kb-system

# CSI-S3 installs a daemonSet pod on all nodes and you can set tolerations to install daemonSet pods on the specified nodes
--set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
```

### Use minIO as the backup storage source

1. Install minIO.

   ```bash
   helm upgrade --install minio oci://registry-1.docker.io/bitnamicharts/minio --set persistence.enabled=true,persistence.storageClass=csi-hostpath-sc,persistence.size=100Gi,defaultBuckets=backup
   ```

2. Get the minIO access key and secret key.

   ```bash
   export ROOT_USER=$(kubectl get secret --namespace default minio -o jsonpath="{.data.root-user}" | base64 -d)
   export ROOT_PASSWORD=$(kubectl get secret --namespace default minio -o jsonpath="{.data.root-password}" | base64 -d)
   ```

3. Install CSI-S3.

   ```bash
   helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

   helm install csi-s3 kubeblocks/csi-s3 --version=0.5.0-beta.17 \
   --set secret.accessKey=$ROOT_USER \
   --set secret.secretKey=$ROOT_PASSWORD \
   --set storageClass.singleBucket=backup  \
   --set secret.endpoint=http://minio.default.svc.cluster.local:9000 \
    -n kb-system

   # CSI-S3 installs a daemonSet pod on all nodes and you can set tolerations to install daemonSet pods on the specified nodes
   --set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
   ```

## Configure global backup storage source by KubeBlocks

You can configure a global backup storage source to make this source the default backup policy of all new clusters. Currently, the glocal backup storage source cannot be synchronized as the backup policy of created clusters.

If there is no PVC, the system creates one automatically based on the configuration.

It takes about 1 minute to make the configuration effective.

:::note

`-n kb-system` specifies the namespace in which KubeBlocks is installed. If you install KubeBlocks in another namespace, specify your namespace instead.

:::

```bash

# CSI-driver suppoers the dynamic volume provisioning, such as CSI-S3
kbcli kubeblocks config --set dataProtection.backupPVCName=backup-data \
--set dataProtection.backupPVCCreatePolicy=IfNotPresent \
--set dataProtection.backupPVCInitCapacity=100Gi \
--set dataProtection.backupPVCStorageClassName=csi-s3 -n kb-system

```

## Create a cluster

`kbcli` and `kubectl` options are available.

**Option 1.** Use `kbcli`

1. Create a cluster.

   ```bash
   kbcli cluster create pg-cluster --cluster-definition='postgresql'
   ```

2. View the backup policy.

   ```bash
   kbcli cluster list-backup-policy pg-cluster
   >
   NAME                                  DEFAULT   CLUSTER      CREATE-TIME                  
   pg-cluster-postgresql-backup-policy   true      pg-cluster   Apr 18,2023 11:40 UTC+0800
   ```

**Option 2.** Use `kubectl`

```yaml
kubectl apply -f -<< EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: pg-cluster
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-14.7.0
  componentSpecs:
  - classDefRef:
      class: general-1c1g
    componentDefRef: postgresql
    enabledLogs:
    - error
    - general
    - slow
    monitor: true
    name: postgresql
    replicas: 1
    resources:
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
EOF
```

## Check the backup policy synchronization

Check whether the backup policy is automatically synchronized with the global backup storage source.

**Option 1.** Use `kbcli`

```bash
kbcli cluster edit-backup-policy pg-cluster-postgresql-backup-policy
> 
spec:
  full:
    backupToolName: postgres-basebackup
    backupsHistoryLimit: 7
    persistentVolumeClaim:
      # This policy creates a PVC automatically if there is no PVC
      createPolicy: IfNotPresent
      # Fill in with the storage class name and the default one is applied if not specified
      storageClassName: "csi-s3"
      initCapacity: 100Gi
      # Fill in with the PVC name for backup storage
      name: "backup-data"
```

**Option 2.** Use `kubectl`

```bash
kubectl edit backuppolicy pg-cluster-postgresql-backup-policy
> 
spec:
  full:
    backupToolName: postgres-basebackup
    backupsHistoryLimit: 7
    persistentVolumeClaim:
      # This policy creates a PVC automatically if there is no PVC
      createPolicy: IfNotPresent
      # Fill in with the storage class name and the default one is applied if not specified
      storageClassName: "csi-s3"
      initCapacity: 100Gi
      # Fill in with the PVC name for backup storage
      name: "backup-data"
```

## Back up the cluster

**Option 1** Use `kbcli`

1. Check whether the cluster is running.

   ```bash
   kbcli cluster list pg-cluster
   > 
   NAME         NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME                 
   pg-cluster   default     postgresql           postgresql-14.7.0   Delete               Running   Apr 18,2023 11:40 UTC+0800  
   ```

2. Create a backup for this cluster.

   ```bash
   kbcli cluster backup pg-cluster --backup-type=full
   > 
   Backup backup-default-pg-cluster-20230418124113 created successfully, you can view the progress:
           kbcli cluster list-backup --name=backup-default-pg-cluster-20230418124113 -n default
   ```

3. View the backup set.

   ```bash
   kbcli cluster list-backups pg-cluster 
   > 
   NAME                                       CLUSTER         TYPE   STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              
   backup-default-pg-cluster-20230418124113   pg-cluster      full   Completed                21s        Apr 18,2023 12:41 UTC+0800   Apr 18,2023 12:41 UTC+0800
   ```

**Option 2.** Use `kuebctl`

```yaml
kubectl apply -f -<< EOF
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: backup-default-pg-cluster
  namespace: default
spec:
  backupPolicyName: pg-cluster-postgresql-backup-policy
  backupType: full
EOF
```

## Restore data from backup

**Option 1.** Use `kbcli`

1. Restore data from the backup.

   ```bash
   kbcli cluster restore new-pg-cluster --backup backup-default-pg-cluster-20230418124113
   >
   Cluster new-pg-cluster created
   ```

2. View this new cluster.

   ```bash
   kbcli cluster list new-pg-cluster
   >
   NAME             NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS     CREATED-TIME                 
   new-pg-cluster   default     postgresql           postgresql-14.7.0   Delete               Running   Apr 18,2023 12:42 UTC+0800
   ```

**Option 2.** Use `kubectl`

```yaml
kubectl apply -f -<< EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    # Add the restored annotation
    kubeblocks.io/restore-from-backup: '{"postgresql":"backup-default-postgresql-20230418140448"}'
  name: new-pg-cluster
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-14.7.0
  componentSpecs:
  - classDefRef:
      class: general-1c1g
    componentDefRef: postgresql
    enabledLogs:
    - error
    - general
    - slow
    monitor: true
    name: postgresql
    replicas: 1
    resources:
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
EOF
```

## Enable scheduled backup

**Option 1.** Use `kbcli`

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
      # Select the basic backup type, available options: snapshot and full
      # This example selects full as the basic backup type
      type: full
  
```

**Option 2.** Use `kubectl`

```bash
kubectl edit backuppolicy pg-cluster-postgresql-backup-policy
> 
spec:
  ...
  schedule:
    baseBackup:
      # UTC time zone, the example below means 2 a.m. every Monday
      cronExpression: "0 18 * * 0"
      # Enable this function
      enable: true
      # Select the basic backup type, available options: snapshot and full
      # This example selects full as the basic backup type
      type: full
      
```
