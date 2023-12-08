---
title: 卷扩展
description: 如何对 Kafka 集群进行卷扩展
keywords: [kafka, 卷扩展, 扩容]
sidebar_position: 4
sidebar_label: 卷扩展
---

# 卷扩展

KubeBlocks 支持对 Pod 进行卷扩展。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list kafka  
```

## 选项 1. 使用 kbcli

使用 `kbcli cluster volume-expand` 命令配置所需资源，然后再次输入集群名称进行卷扩展。

```bash
kbcli cluster volume-expand --storage=30G --component-names=kafka --volume-claim-templates=data kafka
```

- `--component-names` 表示用于卷扩展的组件名称。
- `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
- `--storage` 表示卷的存储容量大小。

## 选项 2. 更改集群的 YAML 文件

在集群的 YAML 文件中更改 `spec.components.volumeClaimTemplates.spec.resources` 的值。`spec.components.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发卷扩展。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: kafka
  namespace: default
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  componentSpecs:
  - name: kafka 
    componentDefRef: kafka
    replicas: 1
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi # 修改磁盘容量
  terminationPolicy: Halt
```
