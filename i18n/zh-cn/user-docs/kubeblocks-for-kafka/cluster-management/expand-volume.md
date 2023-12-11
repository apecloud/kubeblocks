---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [kafka, 磁盘扩容]
sidebar_position: 4
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 扩缩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list kafka  
```

## 选项 1. 使用 kbcli

使用 `kbcli cluster volume-expand` 命令配置所需资源，然后再次输入集群名称进行磁盘扩容。

```bash
kbcli cluster volume-expand --storage=30G --component-names=kafka --volume-claim-templates=data kafka
```

- `--component-names` 表示需扩容的组件名称。
- `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
- `--storage` 表示磁盘需扩容至的大小。

## 选项 2. 更改集群的 YAML 文件

在集群的 YAML 文件中更改 `spec.components.volumeClaimTemplates.spec.resources` 的值。

`spec.components.volumeClaimTemplates.spec.resources` 是 Pod 的存储资源信息，更改此值会触发磁盘扩容。

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
