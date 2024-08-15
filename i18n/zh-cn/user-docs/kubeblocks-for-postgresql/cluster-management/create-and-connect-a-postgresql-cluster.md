---
title: 创建并连接到 PostgreSQL 集群
description: 如何创建并连接到 PostgreSQL 集群
keywords: [postgresql, 创建 PostgreSQL 集群, 连接到 PostgreSQL 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接 PostgreSQL 集群

本文档展示如何创建并连接到一个 PostgreSQL 集群。

## 创建 PostgreSQL 集群

### 开始之前

* [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
* 确保 PostgreSQL 引擎已启用。
  
  ```bash
  kbcli addon list
  >
  NAME                       TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  postgresql                 Helm   Enabled                   true
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
  ```

### 创建集群

KubeBlocks 支持创建两种 PostgreSQL 集群：单机版（Standalone）和主备版（Replication）。单机版仅支持一个副本，适用于对可用性要求较低的场景。 对于高可用性要求较高的场景，建议创建集群版，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。

创建 PostgreSQL 单机版。

```bash
kbcli cluster create postgresql <clustername>
```

创建 PostgreSQL 主备版。

```bash
kbcli cluster create postgresql --mode replication <clustername>
```

如果只有一个节点用于部署集群版，请在创建集群时将 `topology-keys` 设置为 `null`。

```bash
kbcli cluster create postgresql --mode replication --topology-keys null <clustername>
```

如果您想要指定集群版本，可以先查看支持的版本，并通过 `--cluster-version` 指定。

```bash
kbcli clusterversion list

kbcli cluster create --cluster-definition posrgresql --cluster-version postgresql-14.8.0
```

:::note

* 在生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。
* 执行以下命令，查看创建 PostgreSQL 集群的选项和默认值。
  
  ```bash
  kbcli cluster create postgresql -h
  ```

:::

## 连接到 PostgreSQL 集群

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
