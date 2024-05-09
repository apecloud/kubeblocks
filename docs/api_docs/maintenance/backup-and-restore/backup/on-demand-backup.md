---
title: On-demand backup
description: How to back up databases on-demand by snapshot and backup tool
keywords: [backup, on-demand backup, snapshot backup, backup tool]
sidebar_position: 4
sidebar_label: On-demand backup
---

# On-demand backup

KubeBlocks supports on-demand backups. You can customize your backup method by specifying `--method`. The instructions below take using a backup tool and volume snapshot as examples.

## Backup tool

The following command uses the `xtrabackup` backup method to create a backup named `mybackup`.

```bash
# Create a backup
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

# View the backup
kubectl get backup mybackup
>
NAME       POLICY                              METHOD       REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mycluster-mysql-backup-policy       xtrabackup   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

:::note

The `dataprotection.kubeblocks.io/connection-password` in annotations uses the password of the original cluster.

:::

## Volume snapshot backup

To create a backup using the snapshot, the `backupMethod` in the YAML configuration file should be set to `volume-snapshot`.

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
  backupPolicyName: mycluster-mysql-backup-policy
EOF

# View the backup
kubectl get backup mybackup
>
NAME       POLICY                              METHOD            REPO      STATUS      TOTAL-SIZE   DURATION   CREATION-TIME          COMPLETION-TIME        EXPIRATION-TIME
mybackup   mycluster-mysql-backup-policy       volume-snapshot   my-repo   Completed   4426858      2m8s       2023-10-30T07:19:21Z   2023-10-30T07:21:28Z
```

:::caution

1. When creating backups using snapshots, ensure that the storage used supports the snapshot feature; otherwise, the backup may fail.

2. Backups created manually using kubectl will not be automatically deleted. You need to manually delete them.

:::
