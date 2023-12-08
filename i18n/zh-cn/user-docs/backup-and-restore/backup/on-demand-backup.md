---
title: 按需备份
description: 如何进行按需备份
keywords: [备份, 按需备份, 快照备份, 备份工具]
sidebar_position: 4
sidebar_label: 按需备份
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 按需备份

KubeBlocks 支持按需备份。你可以通过指定 `--method` 来自定义备份方法。本文档以使用备份工具和卷快照为例。

## 备份工具

下面使用 `xtrabackup` 备份方法，创建名为 `mybackup` 的备份。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# 创建备份
kbcli cluster backup mysql-cluster --name mybackup --method xtrabackup
>
Backup mybackup created successfully, you can view the progress:
        kbcli cluster list-backups --name=mybackup -n default
        
# 查看备份
kbcli cluster list-backups --name=mybackup -n default
>
NAME       NAMESPACE   SOURCE-CLUSTER   METHOD       STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION
mybackup   default     mysql-cluster    xtrabackup   Completed   4426858      2m8s       Oct 30,2023 15:19 UTC+0800   Oct 30,2023 15:21 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
# 创建备份
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
spec:
  backupMethod: xtrabackup
  backupPolicyName: mysql-cluster-mysql-backup-policy
EOF

# 查看备份
kubectl get backup mybackup
>
NAME       POLICY                              METHOD       REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mysql-cluster-mysql-backup-policy   xtrabackup   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

</TabItem>

</Tabs>

## 卷快照备份

使用云盘快照创建备份的方式与以上命令类似，只需将对应 YAML 中的 `backupMethod` 或者 kbcli 命令中的 `--method` 参数设置为 `volume-snapshot` 即可。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# 创建备份
kbcli cluster backup mysql-cluster --name mybackup --method volume-snapshot
>
Backup mybackup created successfully, you can view the progress:
        kbcli cluster list-backups --name=mybackup -n default
        
# 查看备份
kbcli cluster list-backups --name=mybackup -n default
>
NAME       NAMESPACE   SOURCE-CLUSTER   METHOD            STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION
mybackup   default     mysql-cluster    volume-snapshot   Completed   4426858      2m8s       Oct 30,2023 15:19 UTC+0800   Oct 30,2023 15:21 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
  backupPolicyName: mysql-cluster-mysql-backup-policy
EOF

# 查看备份
kubectl get backup mybackup
>
NAME       POLICY                              METHOD            REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mysql-cluster-mysql-backup-policy   volume-snapshot   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

</TabItem>

</Tabs>

:::caution

1. 使用云盘快照创建备份时，请确保使用的存储支持快照功能，否则会导致备份失败。

2. 通过 kubectl 或者 kbcli 手动创建的备份，不会自动删除，需要用户手动删除。

:::
