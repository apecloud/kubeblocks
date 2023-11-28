---
title: 在云上使用KubeBlocks
description: KubeBlocks, kbcli, multicloud
keywords: [kubeblocks, 简介]
sidebar_position: 2
---

# 在云上使用 KubeBlocks

本指南将引导你快速入门 KubeBlocks，并演示如何通过命令创建演示环境（Playground）。
## 准备工作
在云上部署时，通常使用 Terraform 脚本 来初始化云资源。kbcli 会自动下载并保存该脚本，然后使用 Terraform 命令来创建一个完全托管的 Kubernetes 集群，并在该集群上部署 KubeBlocks。

<Tabs>
<TabItem value="AWS" label="AWS" default>

### 在 AWS 上使用 KubeBlocks 之前
请确保已经：
- 安装 AWS CLI。
- 安装 kubectl。
- 安装 kbcli。
### 配置访问密钥

- 选项1. 使用 aws configure 命令
填写访问密钥，并执行以下命令进行身份验证。

```aws configure
AWS Access Key ID [None]: YOUR_ACCESS_KEY_ID
AWS Secret Access Key [None]: YOUR_SECRET_ACCESS_KEY
```

你可以参考 配置 AWS CLI 获取详细信息。

- 选项2. 使用环境变量

```
export AWS_ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
```
### 初始化 Playground

```
kbcli playground init --cloud-provider aws --region us-west-2
```
- `cloud-provider` 用于指定云服务商。
- `region` 用于指定部署 Kubernetes 集群的地区。你可以在官方网站上找到地区列表。
在初始化过程中，kbcli 将 GitHub 仓库 克隆到 `~/.kbcli/playground` 目录，安装 KubeBlocks，并创建一个 MySQL 集群。执行 `kbcli playground init` 命令后，kbcli 会自动将 kubeconfig 的 context 切换到新的Kubernetes 集群。执行以下命令以查看创建的集群。

```
# 查看 kbcli 版本
kbcli version

# 查看集群列表
kbcli cluster list
```
:::note

整个初始化过程大约需要 20 分钟。如果长时间未安装成功，建议检查网络环境。

:::
</TabItem>
<TabItem value="GCP" label="GCP">

###  在 GCP 上使用 KubeBlocks 之前
请确保已经：
- 拥有 Google Cloud 账户。
- 安装 kubectl。
- 安装 kbcli。

### 配置 GCP 环境

*步骤：*

1. 安装 Google Cloud SDK。
```
# macOS 安装 brew
   brew install --cask google-cloud-sdk

# windows
   choco install gcloudsdk
```
2. 初始化 GCP。
   
   ```gcloud init```

3. 登录 GCP。
   ```
   gcloud auth application-default login
   ```

4. GOOGLE_PROJECT 环境变量，kbcli playground 会在该项目中创建 GKE 集群。
   
   ```
   export GOOGLE_PROJECT=<project-name>
   ```

### 初始化 Playground

执行以下命令，在 GCP 的 us-central1 地区部署一个 GKE 服务，并安装 KubeBlocks。
```
kbcli playground init --cloud-provider gcp --region us-central1
```
* cloud-provider 用于指定云服务商。
* region 用于指定部署 Kubernetes 集群的地区。

在初始化过程中，kbcli 将 GitHub 仓库 克隆到目录~/.kbcli/playground，安装 KubeBlocks，并创建一个 MySQL 集群。执行 kbcli playground init 命令后，kbcli 会自动将 kubeconfig 的 context 切换到新的 Kubernetes 集群。执行以下命令以查看创建的集群。
```
# 查看 kbcli 版本
kbcli version

# 查看集群列表 
kbcli cluster list
```

:::note

整个初始化过程大约需要 20 分钟。如果长时间未安装成功，建议检查网络环境。

:::

</TabItem> 
<TabItem value="Tencent Cloud" label="Tencent Cloud">

### 在腾讯云上使用 KubeBlocks 之前

请确保已经：
- 拥有腾讯云账户。
- 安装 kubectl。
- 安装 kbcli。

### 配置 TKE 环境

*步骤：*
1. 登录腾讯云。
2. 前往容器服务控制台授权资源操作权限。
3. 前往访问管理控制台-> 访问密钥 -> API 密钥管理。点击创建密钥来生成一对 Secret ID 和 Secret Key。
4. 将 Secret ID 和 Secret Key 添加到环境变量中。
```
export TENCENTCLOUD_SECRET_ID=YOUR_SECRET_ID
export TENCENTCLOUD_SECRET_KEY=YOUR_SECRET_KEY
```
### 初始化 Playground
在腾讯云的 `ap-chengdu` 可用区部署一个 Kubernetes 服务，并安装 KubeBlocks。

```
kbcli playground init --cloud-provider tencentcloud --region ap-chengdu
```
- `cloud-provider` 用于指定云服务商。
- `region` 用于指定部署 Kubernetes 集群的地区。
在初始化过程中，kbcli 将 GitHub 仓库 克隆到目录`~/.kbcli/playground`，安装 KubeBlocks，并创建一个 MySQL 集群。执行 `kbcli playground init` 命令后，kbcli 会自动将 kubeconfig 的 context 切换到新的 Kubernetes 集群。执行以下命令以查看创建的集群。

```
# 查看 kbcli 版本
kbcli version

# 查看集群列表
kbcli cluster list
```

:::note

整个初始化过程大约需要 20 分钟。如果长时间未安装成功，建议检查网络环境。

:::

</TabItem>
<TabItem value="Alibaba Cloud" label="Alibaba Cloud">

### 在阿里云上使用 KubeBlocks 之前
请确保已经：
- 拥有阿里云账户。
- 安装 kubectl。
- 安装 kbcli。

### 配置 ACK 环境

*步骤：*
1. 登录阿里云。
2. 按照首次使用容器服务 Kubernetes 版，检查是否已激活 ACK 并分配角色。
 :::note
   在中国大陆地区部署阿里云的用户，请参考相应指南。
   :::
3. 点击 AliyunOOSLifecycleHook4CSRole，点击同意授权以创建一个 AliyunOOSLifecycleHook4CSRole 角色。
创建 ACK 集群时，需要创建和管理节点池，因此需要创建 AliyunOOSLifecycleHook4CSRole 角色，为 OOS 运维编排服务授权以访问其他云产品中的资源，步骤如下（详情请参考官方文档）。
4. 创建 AccessKey ID 和对应的 AccessKey 密钥。
  i. 进入阿里云管理控制台。将鼠标悬停在账户中心处，点击 AccessKey 管理。
  ii. 点击创建 AccessKey，创建 AccessKey ID 和对应的 AccessKey 密钥。 
  iii. 将 AccessKey ID 和 AccessKey 密钥添加到环境变量中，以配置身份授权信息。
  ```
  export ALICLOUD_ACCESS_KEY="YOUR_ACCESS_KEY"
  export ALICLOUD_SECRET_KEY="YOUR_SECRET_KEY"
  ```
  :::note

详情请参考 创建 AccessKey。

   :::

### 初始化 Playground
执行以下命令，在阿里云的 cn-hangzhou 地区部署一个 ACK 集群，并安装 KubeBlocks。
```kbcli playground init --cloud-provider alicloud --region cn-hangzhou```
- `cloud-provider` 用于指定云服务商。
- `region` 用于指定部署 Kubernetes 集群的地区。
在初始化过程中，kbcli 将 GitHub 仓库 克隆到目录`~/.kbcli/playground`，安装 KubeBlocks，并创建一个 MySQL 集群。执行 `kbcli playground init` 命令后，kbcli 会自动将 kubeconfig 的当前 context 切换到新的 Kubernetes 集群。执行以下命令以查看创建的集群。
```
# 查看 kbcli 版本
kbcli version

# 查看集群列表
kbcli cluster list
```
:::note

整个初始化过程大约需要 20 分钟。如果长时间未安装成功，建议检查网络环境。

:::

</TabItem>
</Tabs>

## 在 Playground 中使用 KubeBlocks
你可以根据以下说明，体验 KubeBlocks 的基本功能。
### 查看 MySQL 集群
*步骤：*
1. 查看数据库集群列表。
```
kbcli cluster list
```
2. 查看数据库集群的详细信息，比如 STATUS，Endpoints，Topology，Images 和 Events。
```
kbcli cluster describe mycluster
```
### 访问 MySQL 集群

选项 1.  通过容器网络连接数据库
等待该集群的状态变为 Running，然后执行 kbcli cluster connect 来访问指定的数据库集群。例如:
`kbcli cluster connect mycluster`

选项 2. 远程连接数据库

*步骤：*

1. 获取 Credentials。
```
kbcli cluster connect --show-example --client=cli mycluster
```
2. 执行 port-forward。
```
kubectl port-forward service/mycluster-mysql 3306:3306
>
Forwarding from 127.0.0.1:3306 -> 3306
Forwarding from [::1]:3306 -> 3306
```
3. 打开一个新的终端，连接数据库集群。
```
mysql -h 127.0.0.1 -P 3306 -u root -p"******"
>
...
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> show databases;
>
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mydb               |
| mysql              |
| performance_schema |
| sys                |
+--------------------+
5 rows in set (0.02 sec)
```
### 观测 MySQL 集群
KubeBlocks 具备完整的可观测性能力，下面主要演示其中的监控功能。

*步骤：*

1. 打开 Grafana 仪表盘。
```
kbcli dashboard open kubeblocks-grafana
```

   *结果*

  命令执行后，将自动加载出 Grafana 网站的监控页面。


2. 点击左侧栏的仪表盘图标，页面上会显示两个监控面板。
[图片]

3. 点击 General -> MySQL，监控 Playground 创建的 MySQL 集群的状态。
[图片]

### MySQL 的高可用性
下面通过简单的故障模拟，展示 MySQL 的故障恢复能力。

#### 删除 MySQL 单节点集群
首先删除 MySQL 单节点集群。
```
kbcli cluster delete mycluster
```

#### 创建 MySQL 三节点集群
使用 kbcli 创建一个三节点集群。使用默认配置创建的示例如下。
```
kbcli cluster create --cluster-definition='apecloud-mysql' --set replicas=3
```
#### 模拟 Leader Pod 故障恢复
下面通过删除 Leader Pod 来模拟故障。

*步骤:*

1. 确保新创建的集群状态为Running。
```kbcli cluster list
```
2. 在 Topology 中找到 Leader Pod 的名称。在这个示例中，Leader Pod 的名称是 maple05-mysql-1。

```
kbcli cluster describe maple05
>
Name: maple05         Created Time: Jan 27,2023 17:33 UTC+0800
NAMESPACE        CLUSTER-DEFINITION        VERSION                STATUS         TERMINATION-POLICY
default          apecloud-mysql            ac-mysql-8.0.30        Running        WipeOut

Endpoints:
COMPONENT        MODE             INTERNAL                EXTERNAL
mysql            ReadWrite        10.43.29.51:3306        <none>

Topology:
COMPONENT        INSTANCE               ROLE            STATUS         AZ            NODE                                                 CREATED-TIME
mysql            maple05-mysql-1        leader          Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
mysql            maple05-mysql-2        follower        Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
mysql            maple05-mysql-0        follower        Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800

Resources Allocation:
COMPONENT        DEDICATED        CPU(REQUEST/LIMIT)        MEMORY(REQUEST/LIMIT)        STORAGE-SIZE        STORAGE-CLASS
mysql            false            <none>                    <none>                       <none>              <none>

Images:
COMPONENT        TYPE         IMAGE
mysql            mysql        docker.io/apecloud/wesql-server:8.0.30-5.alpha2.20230105.gd6b8719

Events(last 5 warnings, see more:kbcli cluster list-events -n default mycluster):
TIME        TYPE        REASON        OBJECT        MESSAGE
```
3. 删除 Leader Pod。
 ```
 kubectl delete pod maple05-mysql-1
 >
 pod "maple05-mysql-1" deleted
```
4. 连接三节点集群，只需几秒就可成功。
```
kbcli cluster connect maple05
>
Connect to instance maple05-mysql-2: out of maple05-mysql-2(leader), maple05-mysql-1(follower), maple05-mysql-0(follower)
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 33
Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision d6b8719

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>
```
## 销毁 Playground

1. 在销毁 Playground 之前，建议删除 KubeBlocks 创建的数据库集群。
# 查看所有集群
```
kbcli cluster list -A
```
# 删除一个集群
# 需要进行二次确认，或者你可以添加 --auto-approve 自动确认
```
kbcli cluster delete <name>
```
# 卸载 KubeBlocks
# 需要进行二次确认，或者你可以添加 --auto-approve 自动确认
```
kbcli kubeblocks uninstall --remove-pvcs --remove-pvs
```
2.  销毁 Playground。
```
kbcli playground destroy 
```

:::caution

`kbcli playground destroy` 会直接销毁云上的 Kubernetes 集群。但是云上可能还有一些残留资源，例如卷和快照等。请及时删除，避免不必要的费用。
:::
