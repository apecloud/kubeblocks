---
title: 创建并连接到 Redis 集群
description: 如何创建并连接到 Redis 集群
keywords: [redis, 创建 Redis 集群, 连接到 Redis 集群, 集群, redis sentinel]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接到 Redis 集群

本文档展示如何创建并连接到一个 Redis 集群。

## 创建 Redis 集群

### 开始之前

* [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
* 确保 Redis 引擎已启用。如果未启用，可参考[该文档](./../../overview/database-engines-supported.md#使用引擎)启用引擎。
  
  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  redis                     Helm   Enabled                   true
  ...
  ```

* 查看可用于创建集群的数据库类型和版本。

  ```bash
  kbcli clusterdefinition list

  kbcli clusterversion list
  ```

* 为了保持隔离，本文档中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### 创建集群

KubeBlocks 支持创建两种 Redis 集群：单机版（Standalone）和主备版（Replication）。Redis 单机版仅支持一个副本，适用于对可用性要求较低的场景。 对于高可用性要求较高的场景，建议创建主备版集群，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。

创建 Redis 单机版。

```bash
kbcli cluster create redis --mode standalone <clustername>
```

创建 Redis 主备版。

```bash
kbcli cluster create redis --mode replication <clustername>
```

如果只有一个节点用于部署主备版集群，请在创建集群时将 `topology-keys` 设置为 `null`。

```bash
kbcli cluster create redis --mode replication --topology-keys null <clustername>
```

:::note

* 在生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。
* 执行以下命令，查看创建 Redis 集群的选项和默认值。
  
  ```bash
  kbcli cluster create redis -h
  ```

:::

## 连接到 Redis 集群

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
