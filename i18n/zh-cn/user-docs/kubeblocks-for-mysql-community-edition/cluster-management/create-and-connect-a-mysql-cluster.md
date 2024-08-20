---
title: 创建并连接 MySQL 集群
description: 如何创建并连接到 MySQL 集群
keywords: [mysql, 创建 mysql 集群, 连接 mysql 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接 MySQL 集群

本文档展示如何创建并连接到一个 MySQL 集群。

## 创建 MySQL 集群

### 开始之前

* [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)。
* 确保 MySQL 引擎已启用。如果引擎未启用，可参考[该文档](./../../overview/database-engines-supported.md#使用引擎)启用引擎。
  
  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  mysql                          0.9.1           community   Enabled    true
  ...
  ```

* 查看可用于创建集群的数据库类型和版本。

  ```bash
  kbcli clusterdefinition list

  kbcli clusterversion list
  ```

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种类型的 MySQL 集群：单机版（Standalone）和主备版（Replication）。单机版仅支持一个副本，适用于对可用性要求较低的场景。主备版包含两个个副本，适用于对高可用性要求较高的场景。

为了确保高可用性，所有的副本都默认分布在不同的节点上。

创建单机版集群。

```bash
kbcli cluster create mycluster --cluster-definition mysql
```

创建主备版集群。

```bash
kbcli cluster create mycluster --cluster-definition mysql --set replicas=2
```

如果您只有一个节点可用于部署主备版，可将 `topology-keys` 设置为 `null`。

```bash
kbcli cluster create mycluster --cluster-definition mysql --set replicas=2 --topology-keys null
```

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

如果您想要指定集群版本，可以先查看支持的版本，并通过 `--cluster-version` 指定。

```bash
kbcli clusterversion list

kbcli cluster create mycluster --cluster-definition mysql --cluster-version mysql-8.0.30
```

:::note

* 生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。
* 执行以下命令，查看更多集群创建的选项和默认值。

  ```bash
  kbcli cluster create --help
  ```

:::

## 连接集群

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
