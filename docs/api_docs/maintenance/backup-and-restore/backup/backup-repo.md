---
title: Configure BackupRepo
description: How to configure BackupRepo
keywords: [introduction, backup, restore]
sidebar_position: 1
sidebar_label: Configure BackupRepo
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Introduction

BackupRepo is the storage repository for backup data. Currently, KubeBlocks supports configuring various object storage services as backup repositories, including OSS (Alibaba Cloud Object Storage Service), S3 (Amazon Simple Storage Service), COS (Tencent Cloud Object Storage), GCS (Google Cloud Storage), OBS (Huawei Cloud Object Storage), MinIO, and other S3-compatible services. Additionally, it also supports using Kubernetes-native PVCs as backup repositories.

You can create multiple BackupRepos to suit different scenarios. For example, based on different businesses, the data of business A is stored in repository A, and the data of business B is stored in repository B. Or you can configure multiple repositories by region to realize geo-disaster recovery. But it is required to specify backup repositories when you create a backup. You can also create a default backup repository and KubeBlocks uses this default repository to store backup data if no specific repository is specified.

## Before you start

Make sure you have all the following prepared.

* [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [Install Helm](https://helm.sh/docs/intro/install/)
* [Install KubeBlocks](./../../../installation/install-kubeblocks.md)

## Install MinIO

If you don't have an object storage service from a cloud provider, you can deploy the open-source service MinIO in Kubernetes and use it to configure BackupRepo. If you are using an object storage service provided by a cloud provider, directly skip to [Configure BackupRepo](#configure-backuprepo).

***Steps***

1. Install MinIO in the `kb-system` namespace.

   ```bash
   helm repo add kubeblocks-apps https://jihulab.com/api/v4/projects/152630/packages/helm/stable
   helm install minio kubeblocks-apps/minio --namespace kb-system --create-namespace --set "extraEnvVars[0].name=MINIO_BROWSER_LOGIN_ANIMATION" --set "extraEnvVars[0].value=off"
   ```

   Get the initial username and password:

   ```bash
   # Initial username
   echo $(kubectl get secret --namespace kb-system minio -o jsonpath="{.data.root-user}" | base64 -d)

   # Initial password
   echo $(kubectl get secret --namespace kb-system minio -o jsonpath="{.data.root-password}" | base64 -d)
   ```

2. Generate credentials.

   Access the login page by running `kubectl port-forward --namespace kb-system svc/minio 9001:9001` and then accessing `127.0.0.1:9001`.

   Once you are logged in to the dashboard, you can generate an `access key` and `secret key`.

   ![backup-and-restore-backup-repo-1](./../../../../img/backup-and-restore-backup-repo-1.png)

3. Create a bucket.

   Create a bucket named `test-minio` for the test.

   ![backup-and-restore-backup-repo-2](./../../../../img/backup-and-restore-backup-repo-2.png)
   ![backup-and-restore-backup-repo3](./../../../../img/backup-and-restore-backup-repo-3.png)

  :::note

  The access address (endpoint) for the installed MinIO is `http://minio.kb-system.svc.cluster.local:9000`, which will be used to configure BackupRepo. In this case, `kb-system` is the name of the namespace where MinIO is installed.

  :::

## Configure BackupRepo

With object storage services prepared, it's time to configure BackupRepo. KubeBlocks provides two ways for the configuration:

* Automatic BackupRepo configuration during KubeBlocks installation;
* Manual BackupRepo configuration for on-demand scenarios.
  
### Access BackupRepo

There are two methods to access remote object storage:

* Use command-line tools to directly access remote storage.
* Mount the remote storage to the local system with a CSI driver, allowing the work processes to access the remote storage as if it were local files.
  
The two access methods are referred to as "Tool" and "Mount". When creating BackupRepo, you can specify the access method through the `accessMethod` field, which can not be changed after creation.

Generally, it is recommended to use the "Tool" method as it does not require installing an additional CSI driver, thus reducing dependencies.

However, as backup and restore tasks require running in the namespace of the database cluster, using the "Tool" approach automatically synchronizes the necessary credentials for accessing the remote storage as secret resources in those namespaces. These credentials are used by the data transfer tool. If you have concerns about security risks associated with synchronizing secrets in a multi-tenant environment, you can choose to use the "Mount" method.

### Automatic BackupRepo configuration

You can specify the BackupRepo information in a YAML configuration file when installing KubeBlocks, and KubeBlocks will create the BackupRepo and automatically install the necessary CSI Driver based on the provided configuration.

1. Prepare the configuration file.

   Taking MinIO as an example, the configuration file `backuprepo.yaml` is:

    ```yaml
    spec:
      accessMethod: Tool
      config:
        bucket: test-create-backup-repo
        endpoint: http://kb-addon-minio.kb-system.svc.cluster.local:9099
      credential:
        name: kb-backup-repo-7rsbs
        namespace: kubeblocks-cloud-ns
      pvReclaimPolicy: Retain
      storageProviderRef: minio   
    ```

    * `accessMethod`: specifies the access method of the backup repository.
    * `config`: specifies the non-secret configuration parameters for the `StorageProvider`.
    * `credential`：references the secret that holds the credentials for the `StorageProvider`.
    * `pvReclaimPolicy`：specifies reclaim policy of the PV created by this backup repository.
    * `storageProviderRef`：specifies the name of the `StorageProvider` used by this backup repository, which is minio in this case.

:::note

* From KubeBlocks v0.8.0 on, the available `storageProviderRef` options are `s3`, `cos`, `gcs-s3comp`, `obs`, `oss`, `minio`, `pvc`, `ftp`, and `nfs`.
* For different `storageProviderRef`, the configuration may differ. `config` and `credential` in the above example are applied to minio.
* Execute the command `kubectl get storageproviders.storage.kubeblocks.io` to view the supported `storageProvider` options.

:::

2. Specify the configuration file when installing KubeBlocks.

   ```bash
   kubectl create -f backuprepo.yaml
   ```

   Use the command below to check the BackupRepo after installation.

   ```bash
   kubectl get backuprepo
   ```

### Manual BackupRepo configuration

If you do not configure the BackupRepo information when installing KubeBlocks, you can manually configure it by the following instructions.

1. Install the S3 CSI driver (only used in the Mount method).

    ```bash
    helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable
    helm install csi-s3 kubeblocks/csi-s3 --version=0.7.0 -n kb-system

    # You can add flags to customize the installation of this add-on
    # CSI-S3 installs a daemonSet Pod on all nodes by default and you can set tolerations to install it on the specified node
    --set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
    --set-json daemonsetTolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
    ```

2. Create BackupRepo.

      <Tabs>

      <TabItem value="S3" label="S3" default>

      ```bash
      # Create a secret to save the access key for S3
      kubectl create secret generic s3-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: s3
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          endpoint: ""
          mountOptions: --memory-limit 1000 --dir-mode 0777 --file-mode 0666
          region: cn-northwest-1
        credential:
          name: s3-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      <TabItem value="OSS" label="OSS">

      ```bash
      # Create a secret to save the access key for OSS
      kubectl create secret generic oss-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: oss
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          mountOptions: ""
          endpoint: ""
          region: cn-zhangjiakou
        credential:
          name: oss-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      <TabItem value="OBS" label="OBS">

      ```bash
      # Create a secret to save the access key for OBS
      kubectl create secret generic obs-credential-for-backuprepo \
      -n kb-system \
      --from-literal=accessKeyId=<ACCESS KEY> \
      --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: obs
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          mountOptions: ""
          endpoint: ""
          region: cn-north-4
        credential:
          name: obs-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      <TabItem value="COS" label="COS">

      ```bash
      # Create a secret to save the access key for COS
      kubectl create secret generic cos-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: cos
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          mountOptions: ""
          endpoint: ""
          region: ap-guangzhou
        credential:
          name: cos-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      <TabItem value="GCS" label="GCS">

      ```bash
      # Create a secret to save the access key for GCS
      kubectl create secret generic gcs-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: gcs
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          mountOptions: ""
          endpoint: ""
          region: auto
        credential:
          name: gcs-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      <TabItem value="MinIO" label="MinIO">

      ```bash
      # Create a secret to save the access key for MinIO
      kubectl create secret generic minio-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # Create the BackupRepo resource
      kubectl apply -f - <<-'EOF'
      apiVersion: dataprotection.kubeblocks.io/v1alpha1
      kind: BackupRepo
      metadata:
        name: my-repo
        annotations:
          dataprotection.kubeblocks.io/is-default-repo: "true"
      spec:
        storageProviderRef: minio
        accessMethod: Tool
        pvReclaimPolicy: Retain
        volumeCapacity: 100Gi
        config:
          bucket: test-kb-backup
          mountOptions: ""
          endpoint: <ip:port>
        credential:
          name: minio-credential-for-backuprepo
          namespace: kb-system
      EOF
      ```

      </TabItem>

      </Tabs>

3. View the BackupRepo and its status. If the status is `Ready`, the BackupRepo is ready.

   ```bash
   kubectl get backuprepo
   ```