---
title: 按需备份
description: 如何借助快照和备份工作按需备份
keywords: [备份, 按需备份, 快照备份, 备份工具]
sidebar_position: 4
sidebar_label: 按需备份
---

# 按需备份

KubeBlocks 支持按需备份。你可以通过指定 `--method` 来自定义备份方法。本文档以使用备份工具和卷快照为例。

## 备份工具

The following command uses the `xtrabackup` backup method to create a backup named `mybackup`.
下面使用 `xtrabackup` 备份方法，创建名为 `mybackup` 的备份。

```bash
# 创建备份
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
  annotations:
    dataprotection.kubeblocks.io/connection-password: Bw1cR15mzfldc9hzGuK4m1BZQOzha6aBb1i9nlvoBdoE9to4
spec:
  backupMethod: xtrabackup
  backupPolicyName: mycluster-mysql-backup-policy
EOF

# 查看备份
kubectl get backup mybackup
>
NAME       POLICY                              METHOD       REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mycluster-mysql-backup-policy       xtrabackup   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

:::note

annotations 中的 `dataprotection.kubeblocks.io/connection-password` 使用原集群的密码。

:::

## 卷快照备份

将对应 YAML 中的 `backupMethod` 参数设置为 `volume-snapshot`。

```bash
# 创建备份
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
spec:
  backupMethod: volume-snapshot
  backupPolicyName: mycluster-mysql-backup-policy
EOF

# 查看备份
kubectl get backup mybackup
>
NAME       POLICY                              METHOD            REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mycluster-mysql-backup-policy       volume-snapshot   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

:::warning

1. 使用快照创建备份时，请确保使用的存储支持快照功能，否则会导致备份失败。

2. 通过 kubectl 手动创建的备份，不会自动删除，需要用户手动删除。

:::
