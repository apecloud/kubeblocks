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

:::caution

1. The persistent volume requires `accessMode` to be ReadWriteMany.

   If you use other access modes, these modes may not be applicable in many scenarios. For example,
   a. ReadWriteOnce: If the pods of a cluster need to be scheduled to different nodes, the restore may fail.
   b. ReadOnlyMany: Backups of multiple clusters to this PV may fail.
2. The current backup strategy uses `nodeName` to specify the node affinity of the Pod running the backup task. If the `volumeBindingMode` of the StorageClass is `WaitForFirstConsumer`,  manually create a PV and PVC and wait for the PVC to be bound successfully.

:::

### Use S3 as the backup storage source

1. Enable CSI-S3 and fill in the values based on your actual environment.

   ```bash
   kbcli addon enable csi-s3 \
   --set secret.accessKey=<your_accessKey> \
   --set secret.secretKey=<your_secretKey> \
   --set storageClass.singleBucket=<s3_bucket>  \
   --set secret.endpoint=https://s3.<region>.amazonaws.com.cn \
   --set secret.region=<region> -n kb-system
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

:::caution

CSI-OSS does not support the dynamic volume provisioning of persistent volumes. The following systems are supported:

* Linux:
  * CentOS 7.0 and above
  * Ubuntu 18.04
  * Anolis 7 and above
* FUSE 2.8.4 and above

:::

1. Add a CSI driver.
     * For the Kubernetes clusters that do not install any Alibaba Cloud CSI driver

        ```bash
        kbcli addon enable csi-oss \
        --set secret.akId=<your_akId> \
        --set secret.akSecret=<your_akSecret> \
        --set storageConfig.bucket=<your_bucket>  \
        --set storageConfig.endpoint=oss-cn-hangzhou.aliyuncs.com
        ```

     * For the Kubernetes cluster that already installed Alibaba Cloud CSI driver, choose one of the following two methods to add OSS CSI.

       * Add the OSS CSI driver.

         ```yaml
         kubectl apply -f -<< EOF
         apiVersion: storage.k8s.io/v1
         kind: CSIDriver
         metadata:
           name: ossplugin.csi.alibabacloud.com
         spec:
           attachRequired: false
           podInfoOnMount: true
         EOF
         ```

       * Modify the original DaemonSet.

         1. Add an OSS registrar container.

            ```yaml
            - name: oss-driver-registrar
              image: registry.cn-hangzhou.aliyuncs.com/acs/csi-node-driver-registrar:v1.2.0
              imagePullPolicy: Always
              args:
                - "--v=5"
                - "--csi-address=/var/lib/kubelet/csi-plugins/ossplugin.csi.alibabacloud.com/csi.sock"
                - "--kubelet-registration-path=/var/lib/kubelet/csi-plugins/ossplugin.csi.alibabacloud.com/csi.sock"
              volumeMounts:
              - name: kubelet-dir
                mountPath: /var/lib/kubelet/
              - name: registration-dir
                mountPath: /registration
             ```

         2. Modify the `args` parameter of the CSI plug-in container.

             ```yaml
              --driver=disk,oss  // add oss driver
              --nodeid=$(KUBE_NODE_NAME) // add this arg (this option is required for services other than alibaba esc)
              ```

             Add the following parameters to `volumeMounts`

             ```yaml
             - name: ossconnectordir
               mountPath: /host/usr/

         3. Add the following parameters to `volumes`.

              ```yaml
              - name: ossconnectordir
                hostPath:
                  path: /usr/
              ```

2. Create PV and PVC.

    ```yaml
    kubectl apply -f -<< EOF
    apiVersion: v1
    kind: PersistentVolume
    metadata:
      name: backup-data-pv
      labels:
        alicloud-pvname: backup-data-pv
    spec:
      capacity:
        storage: 100Gi
      accessModes:
        - ReadWriteMany
      persistentVolumeReclaimPolicy: Retain
      csi:
        driver: ossplugin.csi.alibabacloud.com
        # This value should be consistent with the PV name
        volumeHandle: backup-data-pv
        nodePublishSecretRef:
          name: csi-oss-secret
          # The same namespace as kubeblocks
          namespace: kb-system
        volumeAttributes:
          # Fill in this value based on your actual environment
          bucket: "<your_bucket>"
          url: "oss-cn-hangzhou.aliyuncs.com"
          otherOpts: "-o max_stat_cache_size=0 -o allow_other"
          # Fill in this value based on your actual environment
          path: "/"

    ---
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: backup-data
    spec:
      accessModes:
      - ReadWriteMany
      resources:
        requests:
          storage: 100Gi
      # Fill in storageClassName with ""
      storageClassName: ""
      selector:
        matchLabels:
          alicloud-pvname: backup-data-pv
    EOF
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

## Specify the PVC for storing backup

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

## Configure full file backup storage source by KubeBlocks

You can configure the full file backup storage source by using the commands below to make this source the default backup policy of all new clusters. Currently, the backup policy of created clusters cannot be synchronized.

If there is no PVC, the system creates one automatically based on the configuration.

:::note

`-n kb-system` specifies the namespace in which KubeBlocks is installed. If you install KubeBlocks in another namespace, specify your namespace instead.

:::

### Dynamic volume provisioning

This configuration applies to the CSI-driver which supports dynamic volume provisioning, such as CSI-S3.

```bash
kbcli kubeblocks config --set dataProtection.backupPVCName=backup-data \
--set dataProtection.backupPVCInitCapacity=100Gi \
--set dataProtection.backupPVCStorageClassName=csi-s3 -n kb-system
```

### Static volume provisioning

This configuration applies to the CSI-driver which supports static volume provisioning, such as CSI-OSS.

```bash
kbcli kubeblocks config --set dataProtection.backupPVCName=backup-data \
--set dataProtection.backupPVCInitCapacity=100Gi \
--set dataProtection.backupPVConfigMapName=oss-persistent-volume-template \
--set dataProtection.backupPVConfigMapNamespace=kb-system -n kb-system
```

:::note

`backupPVConfigMapName`: there exits a configMap whose key is "persistentVolume" and value is "PersistentVolume struct".

:::

### Self-installed OSS CSI driver

If the OSS CSI driver is installed by yourself, create the persistent-volume-template configMap of OSS by running the commands below.

```yaml
kubectl apply -f -<< EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: oss-persistent-volume-template
  namespace: default
data:
  persistentVolume: |
    apiVersion: v1
    kind: PersistentVolume
    metadata:
      # Self-generated name, the format is "pvcName-pvcNamespace"
      # $(GENERATE_NAME) kubeblocks  
      name: $(GENERATE_NAME)
      labels:
        alicloud-pvname: $(GENERATE_NAME)
    spec:
      capacity:
        storage: 100Gi
      accessModes:
        - ReadWriteMany
      persistentVolumeReclaimPolicy: Retain
      csi:
        driver: ossplugin.csi.alibabacloud.com
        volumeHandle: $(GENERATE_NAME)
        # Fill in the following parameters based on your actual environment
        nodePublishSecretRef:
          name: <your ak/sk secret name>
          namespace: <secret namespace>
        volumeAttributes:
          bucket: <bucket-name>
          url: "oss-cn-hangzhou.aliyuncs.com"
          otherOpts: "-o max_stat_cache_size=0 -o allow_other"
          path: "/"
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
