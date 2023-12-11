---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [mongodb, 磁盘扩容]
sidebar_position: 3
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支持 Pod 扩缩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list mongodb-cluster
```

## 选项 1. 使用 kbcli

使用 `kbcli cluster volume-expand` 命令配置所需资源，然后再次输入集群名称进行磁盘扩容。

```bash
kbcli cluster volume-expand --storage=30G --component-names=mongodb --volume-claim-templates=data mongodb-cluster
>
OpsRequest mongodb-cluster-volumeexpansion-gcfzp created successfully, you can view the progress:
        kbcli cluster describe-ops mongodb-cluster-volumeexpansion-gcfzp -n default
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
  name: mongodb-cluster
  namespace: default
spec:
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-5.0
  componentSpecs:
  - name: mongodb 
    componentDefRef: mongodb
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
