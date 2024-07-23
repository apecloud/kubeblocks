---
title: 从备份集中恢复数据
description: 如何从备份集中恢复数据
keywords: [备份恢复, 恢复, 备份集]
sidebar_position: 1
sidebar_label: 从备份集中恢复数据
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 从备份集中恢复数据

KubeBlocks 支持从备份集中恢复数据。

1. 查看备份。

    ```shell
    kubectl get backups
    ```

2. 从特定的备份中恢复集群。

    可使用原集群的 connection password 作为恢复的集群的 `connectionPassword.annotations` 值。可从备份 YAML 文件中的 `dataprotection.kubeblocks.io/connection-password` annotation 中获取原集群的 connection password。

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

3. 连接被恢复集群，进行验证。

    集群状态为 `Running` 后，[连接集群](./../../../kubeblocks-for-apecloud-mysql/cluster-management/create-and-connect-an-apecloud-mysql-cluster.md#connect-to-a-cluster)进行验证。
