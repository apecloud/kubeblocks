---
title: 配置 BackupRepo
description: 如何配置 BackupRepo
keywords: [简介, 备份, 恢复, backuprepo]
sidebar_position: 1
sidebar_label: 配置 BackupRepo
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 概述

BackupRepo 是备份数据的存储仓库，支持配置 OSS（阿里云对象存储），S3（亚马逊对象存储），COS（腾讯云对象存储），GCS（谷歌云对象存储），OBS（华为云对象存储），MinIO 等兼容 S3 协议的对象存储作为备份仓库，同时支持 K8s 原生的 PVC 作为备份仓库。

用户可以创建多个 BackupRepo 以适应不同的场景。例如，根据不同的业务需求，可以把业务 A 的数据存储在 A 仓库，把业务 B 的数据存储在 B 仓库，或者可以按地区配置多个仓库以实现异地容灾。在创建备份时，你需要指定备份仓库。你也可以创建一个默认的备份仓库，如果在创建备份时未指定具体的仓库，KubeBlocks 将使用此默认仓库来存储备份数据。

## 开始之前

请确保你已经：

* [安装 kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [安装 Helm](https://helm.sh/docs/intro/install/)
* [安装 KubeBlocks](./../../../installation/install-kubeblocks.md)

## 安装 MinIO

如果你没有使用云厂商的对象存储，可在 Kubernetes 中部署开源服务 MinIO，用它来配置 BackupRepo。如果你正在使用云厂商提供的对象存储服务，可以直接跳转至[配置 BackupRepo](#配置-backuprepo)。

***步骤***

1. 在 `kb-system` 命名空间中安装 MinIO。

   ```bash
   helm repo add kubeblocks-apps https://jihulab.com/api/v4/projects/152630/packages/helm/stable
   helm install minio kubeblocks-apps/minio --namespace kb-system --create-namespace --set "extraEnvVars[0].name=MINIO_BROWSER_LOGIN_ANIMATION" --set "extraEnvVars[0].value=off"
   ```

   获取初始用户名和密码：

   ```bash
   # Initial username
   echo $(kubectl get secret --namespace kb-system minio -o jsonpath="{.data.root-user}" | base64 -d)

   # Initial password
   echo $(kubectl get secret --namespace kb-system minio -o jsonpath="{.data.root-password}" | base64 -d)
   ```

2. 生成连接凭证。

   执行 `kubectl port-forward --namespace kb-system svc/minio 9001:9001`，然后访问 `127.0.0.1:9001` 进入登录页面。

   登录到仪表盘后，生成 `access key` 和 `secret key`。

   ![backup-and-restore-backup-repo-1](./../../../../img/backup-and-restore-backup-repo-1.png)

3. 创建 bucket。

   在 MinIO 仪表盘上创建一个名为 `test-minio` 的存储桶。

   ![backup-and-restore-backup-repo-2](./../../../../img/backup-and-restore-backup-repo-2.png)
   ![backup-and-restore-backup-repo3](./../../../../img/backup-and-restore-backup-repo-3.png)

  :::note

  安装的 MinIO 的访问地址（端口）为 `http://minio.kb-system.svc.cluster.local:9000`，用于配置 BackupRepo。在本例中，`kb-system` 是安装 MinIO 的命名空间的名称。

  :::

## 配置 BackupRepo

准备好对象存储服务后，就可以配置 BackupRepo 了。KubeBlocks 支持通过如下两种方式：

* 安装 KubeBlocks 时自动配置 BackupRepo；
* 按需手动配置 BackupRepo。
  
### 访问 BackupRepo

备份和恢复任务在运行时，有两种访问远端对象存储的方式：

* 使用命令行工具，通过网络直接访问远端存储。
* 通过 CSI Driver 将远端存储映射到本地，工作进程可以像访问本地文件一样访问远端存储。

我们将这两种访问方式分别命名为 “Tool” 和 “Mount” 。用户在创建 BackupRepo 时可以通过 `accessMethod` 字段指定其访问方式，创建之后不能修改。

一般来说，推荐使用 “Tool”，因为相比 “Mount”，它不必安装额外的 CSI Driver，减少了一层依赖。

不过，由于备份和恢复任务需要运行在数据库集群所在的 namespace 下，在 “Tool” 方式下，我们会自动将访问远端存储所需的密钥以 secret 资源的形式同步到这些 namespace 中，以供我们的数据传输工具使用。在多租户隔离的情况下，如果你认为这种同步 secret 的做法会带来安全隐患，可以选择使用 “Mount”。

### 自动配置 BackupRepo

安装 KubeBlocks 时，可以通过配置文件指定 BackupRepo 相关信息，KubeBlocks 会根据配置信息创建 BackupRepo 并自动安装必要的 CSI Driver。

1. 准备配置文件。

   以 MinIO 为例，配置文件 `backuprepo.yaml` 如下：

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

    * `accessMethod`：指定备份仓库的连接方式。
    * `config`：指定 `StorageProvider` 的非密码配置参数。
    * `credential`：引用保存 `StorageProvider` 凭证的密钥。
    * `pvReclaimPolicy`：指定此备份库创建的 PV 的回收策略。
    * `storageProviderRef`：指定此备份库使用的 `StorageProvider` 的名称，在此情况下为 `minio`。

:::note

* 从 KubeBlocks v0.8.0 开始，`storageProvider` 目前可选 `s3`、`cos`、`gcs-s3comp`、`obs`、`oss`、`minio`、`pvc`、`ftp`、`nfs`。
* 不同 `storageProvider` 所需的配置信息并不统一，上面展示的 `config` 和 `credential` 适用于 MinIO。
* 执行 `kubectl get storageproviders.dataprotection.kubeblocks.io` 命令可以查看支持的 `storageProvider`。

:::

2. 安装 KubeBlocks 时指定配置文件。

   ```bash
   kubectl create -f backuprepo.yaml
   ```

   安装完成后，可以执行命令查看 BackupRepo。

   ```bash
   kubectl get backuprepo
   ```

### 手动配置 BackupRepo

如果在安装 KubeBlocks 时没有配置 BackupRepo 信息，你可以按照以下说明进行手动配置。

1. 安装 S3 CSI driver （仅访问方式为 “Mount” 时需要安装）。

    ```bash
    helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable
    helm install csi-s3 kubeblocks/csi-s3 --version=0.7.0 -n kb-system

    # You can add flags to customize the installation of this addon
    # CSI-S3 installs a daemonSet Pod on all nodes by default and you can set tolerations to install it on the specified node
    --set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
    --set-json daemonsetTolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
    ```

2. 创建 BackupRepo。

      <Tabs>

      <TabItem value="S3" label="S3" default>

      ```bash
      # 创建密码，用于存储 S3 连接密钥
      kubectl create secret generic s3-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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
      # 创建密码，用于存储 OSS 连接密钥
      kubectl create secret generic oss-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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
      # 创建密码，用于存储 OBS 连接密钥
      kubectl create secret generic obs-credential-for-backuprepo \
      -n kb-system \
      --from-literal=accessKeyId=<ACCESS KEY> \
      --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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
      # 创建密码，用于存储 COS 连接密钥
      kubectl create secret generic cos-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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
      # 创建密码，用于存储 GCS 连接密钥
      kubectl create secret generic gcs-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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
      # 创建密码，用于存储 MinIO 连接密钥
      kubectl create secret generic minio-credential-for-backuprepo \
        -n kb-system \
        --from-literal=accessKeyId=<ACCESS KEY> \
        --from-literal=secretAccessKey=<SECRET KEY>

      # 创建 BackupRepo 资源
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

3. 查看 BackupRepo 及其状态。 如果 STATUS 为 `Ready`，说明 BackupRepo 已经准备就绪。

   ```bash
   kubectl get backuprepo
   ```
