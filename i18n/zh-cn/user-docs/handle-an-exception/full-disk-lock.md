---
title: 磁盘满锁
description: 磁盘空间将满时，如何设置磁盘锁定
sidebar_position: 2
sidebar_label: 磁盘满锁
---

# 磁盘满锁

KubeBlocks 的磁盘满锁功能确保了数据库的稳定性和可用性。该功能在磁盘使用量达到设定的阈值时触发磁盘锁定，从而暂停写操作，只允许读操作。这样的机制可以防止数据库受到磁盘空间耗尽的影响。

## 锁定/解锁机制

只要有某些卷的空间水位超过定义的阈值，实例就会被锁定（只读）。同时，系统会发送相关警报，包括每个卷的具体阈值和空间使用信息。
当所有卷的空间水位都低于定义的阈值时，实例将被解锁（读写）。同时，系统也会发送相关警报，包括每个卷的具体阈值和空间使用信息。

:::note

1. 磁盘满锁功能目前支持全局（ClusterDefinition）启用或禁用，暂不支持集群维度的控制。动态启用或禁用此功能可能会影响使用此 ClusterDefinition 的现有集群实例，并导致它们重新启动。请谨慎操作。
2. 磁盘满锁功能依赖于两个系统资源节点和 nodes/stats 的读取权限（get 和 list）。如果你通过 kbcli 创建实例，请确保为控制器授予 ClusterRoleBinding 的管理权限。
3. KubeBlocks v0.6.0 支持的引擎：ApeCloud MySQL、PostgreSQL、MongoDB。

:::

## 启用磁盘满锁

- 对于 MySQL 数据库，当磁盘使用量达到 `highwatermark` 值时，读写用户无法写入磁盘，而超级用户仍然可以写入。
- 对于 PostgreSQL 和 MongoDB 数据库，当磁盘使用量达到 `highwatermark` 时，无论是读写用户还是超级用户都无法写入。
- 组件级别的高水位的默认阈值为 `90`，当磁盘使用量达到 90% 时将锁定磁盘。而卷级别的设置为 `85`，会覆盖组件级别的阈值。

在集群定义中，添加以下内容以启用磁盘满锁功能。你可以根据需要进行设置。

```yaml
volumeProtectionSpec:
  highWatermark: 90
  volumes:
  - highWatermark: 85
    name: data
```

:::note

推荐将 `highWatermark` 设置为 90。

:::

## 禁用磁盘满锁

从 ClusterDefinition 文件中删除 `volumeProtectionSpec`。
