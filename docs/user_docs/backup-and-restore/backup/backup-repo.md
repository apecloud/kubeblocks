---
title: Configure BackupRepo
description: How to configure BackupRepo
keywords: [introduction, backup, restore]
sidebar_position: 2
sidebar_label: Configure BackupRepo
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Introduction

BackupRepo is the storage repository for backup data, which uses CSI driver to upload backup data to various storage systems, such as object storage systems like S3 and GCS, or storage servers like FTP and NFS. Currently, BackupRepo of Kubeblocks only supports S3 and other S3-compatible storage systems, and more types of storage will be supported in the future.

Users can create multiple BackupRepos to suit different scenarios. For example, based on different businesses, the data of business A is stored in repository A, and the data of business B is stored in repository B. Or you can configure multiple repositories by region to realize geo-disaster recovery. But it is required to specify backup repositories when creating a backup. You can also create a default backup repository and KubeBlocks uses this default repository to store backup data if no specific repository is specified when creating a backup.

The following instructions take AWS S3 as an example to demonstrate how to configure BackupRepo. There are two options for configuring BackupRepo.

* Automatic BackupRepo configuration: you can provide the necessary configuration information (for example, the AccessKey of object storage, etc.) when installing KubeBlocks, and the installer automatically creates a default BackupRepo and the CSI driver add-on required for automatic installation.
* Manual BackupRepo configuration: you can install S3 CSI driver and then create BackupRepo manually.

## Automatic BackupRepo configuration

You can specify the BackupRepo information in a YAML configuration file when installing KubeBlocks, and KubeBlocks creates a BackupRepo automatically.

1. Write the following configuration to the `backuprepo.yaml` file.

    ```yaml
    backupRepo:
      create: true
      storageProvider: s3
      config:
        region: cn-northwest-1
        bucket: test-kb-backup
      secrets:
        accessKeyId: <ACCESS KEY>
        secretAccessKey: <SECRET KEY>
    ```

    * `region`: specifies the region where S3 is located.
    * `bucket`: specifies the bucket name of S3.
    * `accessKeyId`: specifies the Access Key of AWS.
    * `secretAccessKey`: specifies the Secret Key of AWS.

    :::note

    1. For KubeBlocks v0.6.0, the available `storageProvider` are `s3`, `oss`, and `minio`.
    2. For different storage providers, the configuration may differ. `config` and `secrets` in the above example are applied to S3.
    3. You can run the command below to view the supported storage provider.

       ```bash
       kubectl get storageproviders.storage.kubeblocks.io
       ```

    :::

2. Install KubeBlocks.

   ```bash
   kbcli kubeblocks install -f backuprepo.yaml
   ```

## Manual BackupRepo configuration

If you do not configure the BackupRepo information when installing KubeBlocks, you can manually configure it by the following instructions.

### Before you start

There are various ways to create a BackupRepo. Make sure you have done all the necessary preparations before creating it. If you want to use MinIO, you need to make the following configurations in advance.

1. Install MinIO.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   # Install via add-on
   kbcli addon enable minio

   # Access MinIO dashboard
   kbcli dashboard open minio
   ```

   </TabItem>

   <TabItem value="Helm" label="Helm">

   ```bash
   helm install minio oci://registry-1.docker.io/   bitnamicharts/minio
   ```

   </TabItem>

   </Tabs>

   To get initial username and password, try the following.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ``` bash
   Initial username: kubeblocks
   Initial password: kubeblocks
   ```

   </TabItem>

   <TabItem value="Helm" label="Helm">

   ```bash
   # Initial username
   echo $(kubectl get secret --namespace default minio -o jsonpath="{.data.root-user}" | base64 -d)

   # Initial password
   echo $(kubectl get secret --namespace default minio -o jsonpath="{.data.root-password}" | base64 -d)       
   ```

   </TabItem>

   </Tabs>

:::note

If you encounter connection issues while logging in to MinIO with `kbcli dashboard open minio`, try running the command `kubectl port-forward --namespace kb-system svc/kb-addon-minio 9001:9001` (You may need to retry a few times) and then access the dashboard.

:::


2. Generate credentials.

   Access the login page by running `kubectl port-forward --namespace default svc/minio 9001:9001` and then accessing `127.0.0.1:9001`.

   Once you are logged in to the dashboard, you can generate an `access key` and `secret key`.

   ![MinIO dashboard](./../../../img/backup-and-restore-configure-backuprepo-minio.png)

### Install S3 CSI driver

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# Enable the CSI-S3 add-on
kbcli addon enable csi-s3

# You can add flags to customize the installation of this add-on
# CSI-S3 install a daemonSet Pod on all nodes by default and you can set tolerations to install it on the specified node
kbcli addon enable csi-s3 \
  --tolerations '[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]' \
  --tolerations 'daemonset:[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]'

# View the status of CSI-S3 driver and make sure it is Enabled
kbcli addon list csi-s3
```

</TabItem>

<TabItem value="Helm" label="Helm">

```bash
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable
helm install csi-s3 kubeblocks/csi-s3 --version=0.6.0 -n kb-system

# You can add flags to customize the installation of this add-on
# CSI-S3 install a daemonSet Pod on all nodes by default and you can set tolerations to install it on the specified node
--set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
--set-json daemonsetTolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
```

</TabItem>

</Tabs>

### Create BackupRepo

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

   <Tabs>

   <TabItem value="S3" label="S3" default>

   ```bash
   kbcli backuprepo create my-repo \
     --provider s3 \
     --region cn-northwest-1 \
     --bucket test-kb-backup \
     --access-key-id <ACCESS KEY> \
     --secret-access-key <SECRET KEY> \
     --default
   ```

   </TabItem>

   <TabItem value="OSS" label="OSS">

   ```bash
   kbcli backuprepo create my-repo \
     --provider oss \
     --region cn-zhangjiakou \
     --bucket  test-kb-backup \
     # --endpoint https://oss-cn-zhangjiakou-internal.aliyuncs.com \ To display the specified oss endpoint
     --access-key-id <ACCESS KEY> \
     --secret-access-key <SECRET KEY> \
     --default
   ```

   </TabItem>

   <TabItem value="MinIO" label="MinIO">

   ```bash
   kbcli backuprepo create my-repo \
     --provider minio \
     --endpoint <ip:port> \
     --bucket test-minio \
     --access-key-id <ACCESS KEY> \
     --secret-access-key <SECRET KEY> \
     --default
   ```

   </TabItem>

   </Tabs>

   The above command creates a default backup repository `my-repo`.

   * `my-repo` is the name of the created backup repository. If you do not specify a name, the system creates a random name, following the format `backuprepo-xxxxx`.
   * `--default` means that this repository is set as the default repository. Note that there can only be one default global repository. If there exist multiple default repositories, KubeBlocks cannot decide which one to use (similar to the default StorageClass of Kubernetes), which further results in backup failure. Using `kbcli` to create BackupRepo can avoid such problems because `kbcli` checks whether there is another default repository before creating a new one.
   * `--provider` specifies the storage type, i.e. `storageProvider`, and is required for creating a BakcupRepo. The available values are `s3`, `oss`, and `minio`. Parameters for different storage providers vary and you can run `kbcli backuprepo create --provider STORAGE-PROVIDER-NAME -h` to view the flags for different storage providers.

   After `kbcli backuprepo create` is executed successfully, the system creates the K8s resource whose type is `BackupRepo`. You can modify the annotation of this resource to adjust the default repository.

   ```bash
   # Canel the default repository
   kubectl annotate backuprepo old-default-repo \
     --overwrite=true \
     dataprotection.kubeblocks.io/is-default-repo=false
   ```

   ```bash
   # Set a new default repository
   kubectl annotate backuprepo backuprepo-4qms6 \
     --overwrite=true \
     dataprotection.kubeblocks.io/is-default-repo=true
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

   `kubectl` is another option to create a BackupRepo, but the commands do not include parameter and default repository verification compared with kbcli, which is not convenient.

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
     storageProviderRef: s3
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

</TabItem>

</Tabs>

## (Optional) Change the backup repository for a cluster

By default, all backups are stored in the global default repository when creating a cluster. You can specify another backup repository for this cluster by editing `BackupPolicy`.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster edit-backup-policy mysql-cluster --set="datafile.backupRepoName=my-repo"
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl edit backuppolicy mysql-cluster-mysql-backup-policy
...
spec:
  datafile:
    ... 
    # Edit the backup repository name
    backupRepoName: my-repo
```

</TabItem>

</Tabs>
