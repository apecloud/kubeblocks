---
title: 在测试环境中连接数据库
description: 如何在测试环境中连接数据库
keywords: [连接数据库, 测试环境]
sidebar_position: 2
sidebar_label: 测试环境
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 在测试环境中连接数据库

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## 方案 1. 使用 kbcli cluster connect 命令

您可以使用 `kbcli cluster connect` 命令并指定要连接的集群名称。

```bash
kbcli cluster connect ${cluster-name}
```

它的底层命令是 `kubectl exec`。只要能够访问 K8s API 服务器，就可以使用该命令。

## 方案 2. 使用 CLI 或者 SDK 客户端连接数据库

执行以下命令以获取目标数据库的网络信息，并使用其打印输出的 IP 地址进行连接。

```bash
kbcli cluster connect --show-example --show-password ${cluster-name}
```

其打印输出的信息包括数据库地址、端口号、用户名和密码。下图以 MySQL 数据库为例。

- 地址：-h 表示服务器地址。在下面的示例中为 127.0.0.1。
- 端口：-P 表示端口号。在下面的示例中为 3306。
- 用户名：-u 表示用户名。
- 密码：-p 表示密码。在下面的示例中为 hQBCKZLI。

:::note

密码不包括 -p 本身。

:::
![testing env](../img/../../img/connect-to-database-in-testing-env.png)

</TabItem>

<TabItem value="kubectl" label="kubeclt">

## 步骤 1. 获取数据库凭证

在连接运行在 Kubernetes 集群内的 MySQL 数据库之前，您需要从 Kubernetes Secret 中获取用户名和密码。Kubernetes 中的 Secret 通常是经过 base64 编码的，因此您需要将其解码以获得实际的凭据。以下是使用 kubectl 获取凭证的方法。

1. 获取 `username`。

   使用 `kubectl get secrets` 命令，从 demo 命名空间中名为 `mycluster-conn-credential` 的 secret 中提取用户名。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   root
   ```

   - 您可使用实际的集群名称替换命令中的 "mycluster"。
   - 您可使用实际的命名空间名称替换命令中的 "demo"。

2. 获取 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   2gvztbvz
   ```

   - 您可使用实际的集群名称替换命令中的 "mycluster"。
   - 您可使用实际的命名空间名称替换命令中的 "demo"。

:::note

大多数 KubeBlocks v0.9 Addon 中，包含连接凭据的 secret 命名格式为 `{cluster.name}-conn-credential`。但是在新版本中，部分 Addon secret 的命名可能已更改为 `{cluster.name}-{component.name}-account-{account.name}`。为确保使用正确的 secret 名称，可执行以下命令罗列命名空间中的所有 secret，并查找与数据库集群相关的项。

```bash
kubectl get secrets -n demo | grep mycluster
```

:::

## 步骤 2. 连接集群

获取凭证后，您可以通过以下两种方式连接至在 K8s 集群中运行的 MySQL 数据库。

- 使用 `kubectl exec` 直连 Pod。
- 使用 `kubectl port-forward` 从本地连接数据库。

### 方式 1. 使用 kubectl exec 直连 Pod

在某些情况下，您可能需要直接连接到 Kubernetes Pod 内运行的 MySQL 数据库，而无需依赖外部访问。这种情况下，您可以使用 `kubectl exec` 进入 Pod，直接在集群内与 MySQL 实例进行交互，避免使用外部数据库地址。

1. 指定需要连接的 Pod，并执行以下命令。

   `kubectl exec` 命令在 MySQL Pod 中创建了交互式 shell 对话，您可以直接在 Pod 环境中使用以下命令。

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash
   ```

   - `-ti`：打开交互式终端会话（`-t` 分配一个 TTY，`-i` 将伪 TTY 传输给容器）。
   - `-n demo`：指定 Pod 所在的命名空间 demo。
   - `mycluster-mysql-0`：MySQL Pod 的名称。如果名称与您的实际情况不同，请确保替换为实际的 Pod 名称。
   - `-- bash`：在 Pod 内打开一个 Bash shell。如果容器中没有 Bash，也可以使用 sh。

2. 连接集群。

   进入 Pod 后，您可使用 MySQL 客户端连接到同一 Pod 或集群内运行的数据库服务。由于您已经在 Pod 内，因此无需指定外部主机或地址。

   ```bash
   mysql -u root -p2gvztbvz
   ```

### 方式 2. 使用 kubectl port-forward 连接

在 Kubernetes 集群中管理已部署的数据库时，可使用 `kubectl port-forward` 从本地安全地连接到数据库。该命令将本地端口的流量转发到 Kubernetes 集群中的端口，您可以像在本地运行数据库服务一样访问数据库集群。

以下为在本地使用 CLI 工具连接集群的示例。

1. 使用 `kubectl port-forward` 转发端口。

   首先，您需要从本地将端口转发至在 K8s 中运行的 MySQL 服务。如下命令将您本地的 3306 端口转发至集群中 MySQL 服务的同一端口。

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo
   ```

   - `svc/mycluster-mysql`：代指您的 K8s 集群中的 MySQL 服务。
   - `3306:3306`：将本地 3306 端口与服务的 3306 端口绑定。
   - `-n demo`：指定 MySQL 服务所在的命名空间 demo。

2. 从本地连接数据库。

   端口转发完成后，您可以使用标准的 MySQL 客户端连接至 MySQL 数据库，这一操作与通过 127.0.0.1 连接客户端的体验一致。该连接将安全地通过这一通道转发到集群内部的服务。

   ```bash
   mysql -h 127.0.0.1 -P 3306 -u root -p2gvztbvz
   ```

</TabItem>

</Tabs>
