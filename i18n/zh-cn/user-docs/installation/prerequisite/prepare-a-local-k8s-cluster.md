---
title: 创建本地 Kubernetes 测试集群
description: 创建本地 Kubernetes 测试集群
keywords: [kbcli, kubeblocks, addons, 安装，引擎]
sidebar_position: 3
sidebar_label: 本地测试环境准备
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建本地 Kubernetes 测试集群

本文将说明如何使用三种流行的轻量级工具：Minikube、K3d 和 Kind，在本地创建 Kubernetes 测试集群，可用于在本地环境试用 KubeBlocks。这些工具非常适合开发、测试和实验，而无需搭建完整的生产级 Kubernetes 集群。

## 前置条件

在开始之前，请确保您已经在本地安装了以下工具：

- Docker：所有三个工具都使用 Docker 来创建容器化的 Kubernetes 集群。
- kubectl：Kubernetes 的命令行工具，用于与集群交互。参考 [kubectl 安装文档](https://kubernetes.io/docs/tasks/tools/)。

## 使用 Kind 创建 Kubernetes 集群

Kind 是 Kubernetes IN Docker 的缩写。它在 Docker 容器中运行 Kubernetes 集群，非常适合在本地进行 Kubernetes 测试。

1. 安装 Kind。详情可参考 [Kind Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/)。

   <Tabs>

   <TabItem value="macOS" label="macOS" default>

   ```bash
   brew install kind
   ```

   </TabItem>

   <TabItem value="Linux" label="Linux">

   ```bash
   # For AMD64 / x86_64
   [ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
   # For ARM64
   [ $(uname -m) = aarch64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-arm64
   chmod +x ./kind
   sudo cp ./kind /usr/local/bin/kind
   rm -rf kind
   ```

   </TabItem>

   <TabItem value="Windows" label="Windows">

   可使用 chocolatey 安装 Kind。

   ```bash
   choco install kind
   ```

   </TabItem>

   </Tabs>

2. 通过 Kind 创建 Kubernetes 集群：

   ```bash
   kind create cluster --name mykindcluster
   ```

   该命令将创建一个单节点的 Kubernetes 集群，运行在 Docker 容器中。

3. 检查集群是否已启动并运行。

   ```bash
   kubectl get nodes
   ```

   从输出结果中可以看到一个名为 `mykindcluster-control-plane` 的节点。

4. （可选）配置多节点集群。

   Kind 也支持创建多节点的集群。可通过配置文件来创建多节点集群：

   ```yaml
   kind: Cluster
   apiVersion: kind.x-k8s.io/v1alpha4
   nodes:
     role: control-plane
     role: worker
     role: worker
   ```

   使用该配置文件创建集群：

   ```bash
   kind create cluster --name multinode-cluster --config kind-config.yaml
   ```

5. 如需删除 Kind 集群，可以使用以下命令。

   ```bash
   kind delete cluster --name mykindcluster
   ```

## 使用 Minikube 创建 Kubernetes 集群

Minikube 支持在本地机器的虚拟机或容器中运行单节点的 Kubernetes 集群。

1. 安装 Minikube。详情可参考 [Minikube Quck Start](https://minikube.sigs.k8s.io/docs/start/)。

   <Tabs>

   <TabItem value="macOS" label="macOS" default>

   ```bash
   brew install minikube
   ```

   </TabItem>

   <TabItem value="Linux" label="Linux">

   ```bash
   curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-latest.x86_64.rpm
   sudo rpm -Uvh minikube-latest.x86_64.rpm
   ```

   </TabItem>

   <TabItem value="Windows" label="Windows">

   可使用 chocolatey 安装 Minikube。

   ```bash
   choco install minikube
   ```

   </TabItem>

   </Tabs>

2. 启动 Minikube。该命令将创建一个本地的 Kubernetes 集群：

   ```bash
   minikube start
   ```

   您也可以指定其他驱动（例如 Docker、Hyperkit、KVM 等）启动。

   ```bash
   minikube start --driver=docker
   ```

3. 验证安装。

   检查 Minikube 是否正在运行：

   ```bash
   minikube status
   ```

   检查 Kubernetes 集群是否已启动：

   ```bash
   kubectl get nodes
   >
   NAME       STATUS   ROLES           AGE    VERSION
   minikube   Ready    control-plane   197d   v1.26.3
   ```

   从输出结果中可以看到 Minikube 节点处于 Ready 状态。

## 使用 k3d 创建 Kubernetes 集群

k3d 是一个轻量级工具，它在 Docker 容器中运行 k3s（一个轻量的 Kubernetes 发行版）。

1. 安装 k3d。详情可参考 [k3d Quick Start](https://k3d.io/v5.7.4/#releases)。

   <Tabs>

   <TabItem value="macOS" label="macOS" default>

   ```bash
   brew install k3d
   ```

   </TabItem>

   <TabItem value="Linux" label="Linux">

   ```bash
   curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
   ```

   </TabItem>

   <TabItem value="Windows" label="Windows">

   可使用 chocolatey 安装 k3d。

   ```bash
   choco install k3d
   ```

   </TabItem>

   </Tabs>

2. 创建 k3s 集群。

   ```bash
   k3d cluster create myk3s
   ```

   执行该命令后，将创建一个名为 `myk3s` 的 Kubernetes 集群，包含一个服务器节点。

3. 验证集群是否运行。

   ```bash
   kubectl get nodes
   ```

4. 如需删除 k3s 集群，可使用以下命令。

   ```bash
   k3d cluster delete mycluster
   ```
