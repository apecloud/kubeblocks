---
title: Create a local Kubernetes test cluster
description: Create a local Kubernetes test cluster
keywords: [kbcli, kubeblocks, addons, installation]
sidebar_position: 3
sidebar_label: Prerequisite for Local Env
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create a local Kubernetes test cluster

This tutorial introduces how to create a local Kubernetes test cluster using Minikube, K3d, and Kind. These tools make it easy to try out KubeBlocks on your local host, offering a great solution for development, testing, and experimentation without the complexity of creating a full production-grade cluster.

## Before you start

Make sure you have the following tools installed on your local host:

- Docker: All three tools rely on Docker to create containerized Kubernetes clusters.
- kubectl: The Kubernetes command-line tool for interacting with clusters. Refer to the [kubectl installation guide](https://kubernetes.io/docs/tasks/tools/)

<Tabs>

<TabItem value="Kind" label="Kind" default>

## Create a Kubernetes cluster using Kind

Kind stands for Kubernetes IN Docker. It runs Kubernetes clusters within Docker containers, making it an ideal tool for local Kubernetes testing.

1. Install Kind. For details, you can refer to [Kind Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/).

   <Tabs>

   <TabItem value="macOS" label="macOS" default>

   ```bash
   brew install kind
   ```

   </TabItem>

   <TabItem value="Linux" label="Linux">

   ```bash
   # For AMD64 / x86_64
   [ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
   chmod +x ./kind
   sudo cp ./kind /usr/local/bin/kind
   rm -rf kind
   ```

   </TabItem>

   <TabItem value="Windows" label="Windows">

   You can use chocolatey to install Kind.

   ```bash
   choco install kind
   ```

   </TabItem>

   </Tabs>

2. Create a Kind cluster.

   ```bash
   kind create cluster --name mykindcluster
   ```

   This command creates a single-node Kubernetes cluster running in a Docker container.

3. Check whether the cluster is started and running.

   ```bash
   kubectl get nodes
   >
   NAME                          STATUS   ROLES           AGE   VERSION
   mykindcluster-control-plane   Ready    control-plane   25s   v1.31.0
   ```

   You can see a node named `mykindcluster-control-plane` from the output, which means the cluster is created successfully.

4. (Optional) Configure a cluster with multiple nodes.

   Kind also supports clusters with multiple nodes. You can create a multi-node cluster by a configuration file.

   ```yaml
   kind: Cluster
   apiVersion: kind.x-k8s.io/v1alpha4
   nodes:
     role: control-plane
     role: worker
     role: worker
   ```

   Use the configuration file to create a multi-node cluster.

   ```bash
   kind create cluster --name multinode-cluster --config kind-config.yaml
   ```

5. If you want to delete the Kind cluster, run the command below.

   ```bash
   kind delete cluster --name mykindcluster
   ```

</TabItem>

<TabItem value="Minikube" label="Minikube">

## Create a Kubernetes cluster using Minikube

Minikube runs a single-node Kubernetes cluster on your local machine, either in a virtual machine or a container.

1. Install Minikube. For details, you can refer to [Minikube Quck Start](https://minikube.sigs.k8s.io/docs/start/).

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

   You can use chocolatey to install Minikube.

   ```bash
   choco install minikube
   ```

   </TabItem>

   </Tabs>

2. Start Minikube. This command will create a local Kubernetes cluster.

   ```bash
   minikube start
   ```

   You can also specify other drivers (such as Docker, Hyperkit, KVM) to start it.

   ```bash
   minikube start --driver=docker
   ```

3. Verify whether Minikube and the K8s cluster is running normally.

   Check whether Minikube is running.

   ```bash
   minikube status
   >
   minikube
   type: Control Plane
   host: Running
   kubelet: Running
   apiserver: Running
   kubeconfig: Configured
   ```

   Check whether the K8s cluster is running.

   ```bash
   kubectl get nodes
   >
   NAME       STATUS   ROLES           AGE    VERSION
   minikube   Ready    control-plane   1d     v1.26.3
   ```

   From the output, we can see that the Minikube node is ready.

</TabItem>

<TabItem value="k3d" label="k3d">

## Create a Kubernetes cluster using k3d

k3d is a lightweight tool that runs k3s (a lightweight Kubernetes distribution) in Docker containers.

1. Install k3d. For details, refer to [k3d Quick Start](https://k3d.io/v5.7.4/#releases).

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

   You can use chocolatey to install k3d.

   ```bash
   choco install k3d
   ```

   </TabItem>

   </Tabs>

2. Create a k3s cluster.

   ```bash
   k3d cluster create myk3s
   ```

   This command will create a Kubernetes cluster named as `myk3s` with a single node.

3. Verify whether this cluster is running normally.

   ```bash
   kubectl get nodes
   >
   NAME                 STATUS   ROLES                  AGE   VERSION
   k3d-myk3s-server-0   Ready    control-plane,master   31s   v1.30.4+k3s1
   ```

4. If you want to delete the k3s cluster, run the command below.

   ```bash
   k3d cluster delete myk3s
   ```

</TabItem>

</Tabs>
