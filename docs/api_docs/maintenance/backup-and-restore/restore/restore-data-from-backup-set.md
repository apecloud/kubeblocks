---
title: Restore data from backup set
description: How to restore data from backup set
keywords: [backup and restore, restore, backup set]
sidebar_position: 1
sidebar_label: Restore from backup set
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Restore data from backup set

KubeBlocks supports restoring clusters from backups with the following instructions.

1. View backups.

    ```shell
    kubectl get backups
    ```

2. Restore clusters from a specific backup.

    You can set the `connectionPassword.annotations` of the restored cluster as that of the original cluster. The password of the original cluster can be accessed by viewing the annotation of `dataprotection.kubeblocks.io/connection-password` in the backup YAML file.

    ```bash
    kubectl apply -f - <<-'EOF'
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: myrestore
      namespace: default
      annotations:
        kubeblocks.io/restore-from-backup: '{"mysql":{"name":"mybackup","namespace":"default","connectionPassword": "Bw1cR15mzfldc9hzGuK4m1BZQOzha6aBb1i9nlvoBdoE9to4"}}'
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      terminationPolicy: WipeOut
      componentSpecs:
        - name: mysql
          componentDefRef: mysql
          replicas: 1
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

3. Connect to the restored cluster for verification.

    Once the cluster status is `Running`, [connect to the cluster](./../../../kubeblocks-for-apecloud-mysql/cluster-management/create-and-connect-an-apecloud-mysql-cluster.md#connect-to-a-cluster) for verification.
