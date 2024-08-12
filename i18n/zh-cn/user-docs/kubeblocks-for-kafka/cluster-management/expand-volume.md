---
title: 磁盘扩容
description: 如何调整集群所使用的磁盘大小
keywords: [kafka, 磁盘扩容]
sidebar_position: 4
sidebar_label: 磁盘扩容
---

# 磁盘扩容

KubeBlocks 支 支持 Pod 磁盘存储扩容。

## 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list kafka  
```

## 步骤

1. 使用 `kbcli cluster volume-expand` 命令配置所需资源，然后再次输入集群名称进行磁盘扩容。

   ```bash
   kbcli cluster volume-expand --storage=30G --components=kafka --volume-claim-templates=data kafka
   ```

   - `--components` 表示需扩容的组件名称。
   - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
   - `--storage` 表示磁盘需扩容至的大小。

2. 验证磁盘扩容操作是否成功。

   ```bash
   kbcli cluster list kafka-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   kafka-cluster        default          redis                     kafka-3.3.2              Delete                    Running        May 11,2023 15:27 UTC+0800
   ```
