---
title: 配置 BackupPolicy
description: 如何配置 BackupPolicy
keywords: [备份, backup policy]
sidebar_position: 2
sidebar_label: 配置 BackupPolicy
---

# 配置 BackupPolicy

## 配置加密密钥

为了确保恢复的集群能够正常访问，KubeBlocks 在备份集群时会将集群的连接密码加密，并将其安全地存储在 Backup 对象的 Annotation 中。因此，为了保障您的数据安全，强烈建议谨慎分配备份对象的 Get/List 权限，并在安装或升级 KubeBlocks 时，务必指定加密密钥。这些措施将有助于确保你的数据得到妥善保护。

KubeBlocks v0.9.0 版本集成了 datasafed 的数据加密功能，目前已支持 `AES-128-CFB`、`AES-192-CFB` 和 `AES-256-CFB` 等加密算法。在写入存储之前，备份数据会被加密。您的加密密钥不仅可以用于加密连接密码，还可以备份数据。用户可以根据实际需要，直接引用已有的 secret 对象，或给数据库集群创建不同的密钥，从而进行备份加密。

### 引用已有的 key

如果 Secret 对象已存在，您可以选择直接引用，而无需设置 `dataProtection.encryptionKey`。KubeBlocks 提供了快速引用已有 encryptionKey 进行加密的方式。

假设已经有一个名为 `dp-encryption-key` 的 Secret，其中包含一个名为 `encryptionKey` 的密钥。例如，通过以下命令创建了一个密钥：

```bash
kubectl create secret generic dp-encryption-key \
    --from-literal=encryptionKey='S!B\*d$zDsb='
```

您可以在安装或升级 KubeBlocks 时引用该密钥。

```bash
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace 
    --set dataProtection.encryptionKeySecretKeyRef.name="dp-encryption-key" \
    --set dataProtection.encryptionKeySecretKeyRef.key="encryptionKey"
```

### 创建新的 key

如果无需默认开启备份加密，或者需要使用独立的 `encryptionKey`，您可以按照以下步骤创建一个 Secret 对象，手动开启备份加密。

1. 创建 Secret，用于存储加密密钥。

    ```bash
    kubectl create secret generic backup-encryption \
    --from-literal=secretKey='your secret key'
    ```

2. 修改 BackupPolicy，开启备份加密。此处需要引用之前创建的密钥：

    ```bash
    kubectl --type merge patch backuppolicy mysqlcluster-mysql-backup-policy \
    -p '{"spec":{"encryptionConfig":{"algorithm":"AES-256-CFB","passPhraseSecretKeyRef":{"name":"backup-encryption","key":"secretKey"}}}}'
    ```

配置完成，可照常执行备份和恢复操作。

:::warning

请不要修改或删除在第一步创建的 Secret，否则将来可能无法解密备份。

:::

默认情况下，`encryptionKey` 仅用于加密连接密码。如果您想用它加密备份数据，请在上述命令中添加 `--set dataProtection.enableBackupEncryption=true`。然后，所有新建的数据库集群都会默认开启备份加密。

## 创建集群

准备一个集群，用于后续测试备份恢复功能。本文创建了一个名为 `mycluster` 的 ApeCloud MySQL 集群，具体操作可参考[创建 ApeCloud MySQL 集群](./../../../kubeblocks-for-apecloud-mysql/cluster-management/create-and-connect-an-apecloud-mysql-cluster.md)。

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: default
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: true
    replicas: 3
    serviceAccountName: kb-acmysql-cluster
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
EOF
```

如无特殊设置，所有备份都会保存在全局默认仓库中，可以执行如下命令查看所有 BackupRepo，其中 `DEFAULT` 字段为 `true`，表示该 BackupRepo 为默认 BackupRepo。

```bash
# 查看 BackupRepo
kubectl get backuprepo
```

## 查看 BackupPolicy

使用 KubeBlocks 创建数据库集群后，对于支持备份的数据库，会自动为其创建一个备份策略（BackupPolicy），可以执行如下命令查看集群的备份策略：

```bash
kubectl get backuppolicy | grep mycluster
>
mycluster-mysql-backup-policy                            Available   35m
mycluster-mysql-backup-policy-hscale                     Available   35m
```

备份策略中包含了该集群支持的备份方法，执行以下命令进行查看备份方法：

```bash
kubectl get backuppolicy mycluster-mysql-backup-policy -o yaml
```

对于 MySQL 集群而言，默认支持两种备份方法：`xtrabackup` 和 `volume-snapshot`，前者使用备份工具 `xtrabackup` 将 MySQL 数据备份至对象存储中；后者则使用云存储的卷快照能力，通过快照方式对数据进行备份。创建备份时，可以指定要使用哪种备份方法进行备份。
