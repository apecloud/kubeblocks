---
title: On-demand backup
description: How to back up databases on-demand by snapshot and backup tool
keywords: [backup, on-demand backup, snapshot backup, backup tool]
sidebar_position: 4
sidebar_label: On-demand backup
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# On-demand backup

KubeBlocks supports on-demand backups. You can customize your backup method by specifying `--method`. The instructions below take using a backup tool and volume snapshot as examples.

## Backup tool

The following command uses the `xtrabackup` backup method to create a backup named `mybackup`.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# Create a backup
kbcli cluster backup mysql-cluster --name mybackup --method xtrabackup
>
Backup mybackup created successfully, you can view the progress:
        kbcli cluster list-backups --name=mybackup -n default
        
# View the backup
kbcli cluster list-backups --name=mybackup -n default
>
NAME       NAMESPACE   SOURCE-CLUSTER   METHOD       STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION
mybackup   default     mysql-cluster    xtrabackup   Completed   4426858      2m8s       Oct 30,2023 15:19 UTC+0800   Oct 30,2023 15:21 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
# Create a backup
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

# View the backup
kubectl get backup mybackup
>
NAME       POLICY                              METHOD       REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mysql-cluster-mysql-backup-policy   xtrabackup   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

</TabItem>

</Tabs>

## Volume snapshot backup

To create a backup using the snapshot, the `backupMethod` in the YAML configuration file or the `--method` field in the kbcli command should be set to `volume-snapshot`.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
# Create a backup
kbcli cluster backup mysql-cluster --name mybackup --method volume-snapshot
>
Backup mybackup created successfully, you can view the progress:
        kbcli cluster list-backups --name=mybackup -n default
        
# View the backup
kbcli cluster list-backups --name=mybackup -n default
>
NAME       NAMESPACE   SOURCE-CLUSTER   METHOD            STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION
mybackup   default     mysql-cluster    volume-snapshot   Completed   4426858      2m8s       Oct 30,2023 15:19 UTC+0800   Oct 30,2023 15:21 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
# Create a backup
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

# View the backup
kubectl get backup mybackup
>
NAME       POLICY                              METHOD            REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mysql-cluster-mysql-backup-policy   volume-snapshot   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

</TabItem>

</Tabs>

:::caution

1. When creating backups using snapshots, ensure that the storage used supports the snapshot feature; otherwise, the backup may fail.

2. Backups created manually using kubectl or kbcli will not be automatically deleted. You need to manually delete them.

:::
