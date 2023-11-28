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