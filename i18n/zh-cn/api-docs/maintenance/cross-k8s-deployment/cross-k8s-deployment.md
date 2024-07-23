---
title: 多 k8s 部署
description: 如何使用 KubeBlocks 实现多 k8s 部署
keywords: [cross k8s deployment]
sidebar_position: 1
sidebar_label: 使用 KubeBlocks 实现多 k8s 部署
---

# 使用 KubeBlocks 实现多 k8s 部署

KubeBlocks 支持管理多个 Kubernetes 集群，为用户在实例容灾、k8s 集群管理等方面提供新的选项。为支持多 K8s 管理，KubeBlocks 引入了 control plane 和 data plane。

* Control plane：一个独立的 k8s 集群，KubeBlocks operator 运行在该集群当中，KubeBlocks 定义的相关对象大都存放在这个集群（比如 definition、cluster、backup、ops 等）。用户通过跟这个集群的 API 进行交互来实现对多集群实例的管理。
* Data plane：用于运行最终 workload 的 k8s 集群，数量可以是一到多个。这些集群当中会 hosting 实例相关的计算、存储、网络等资源，如 pod、pvc、service、sa、cm、secret、jobs 等，而 KubeBlocks operator 目前（v0.9.0）不会运行在当中。

实际物理部署上，control plane 可以选择部署在单个 AZ，简单灵活；也可以选择部署在多个不同 AZ，提供更高的可用性保证；也可以复用某个 data plane 集群，以更低成本的方式运行。

## 环境准备

Create several K8s clusters and prepare the configuration information for deploying KubeBlocks. This tutorial takes three data plane K8s clusters as an example and their contexts are named as k8s-1, k8s-2, and k8s-3.
准备 K8s 集群，并准备部署 KubeBlocks 所需的配置信息。本文示例准备了三个 data plane 集群，context 分别命名为：k8s-1、k8s-2、k8s-3。

* 准备 K8s 集群：1 个设定为 control plane，其他几个设定为 data plane，确保这些 data plane 集群的 API server 在 control plane 集群中可以联通。这里的联通包含两个层面：一是网络连通，二是访问配置。
* 准备 KubeBlocks operator 访问 data plane 所需的配置信息，以 secret 形式放置在 control plane 集群当中，部署 KubeBlocks operator 时需要传入。其中，secret key 要求为 “kubeconfig”，value 为标准 kubeconfig 内容格式。示例如下：

   ```bash
   apiVersion: v1
   kind: Secret
   metadata:
     namespace: kb-system
     name: <your-secret-name> 
   type: kubernetes.kubeconfig
   stringData:
     kubeconfig: |
       apiVersion: v1
       clusters:
         ...
       contexts:
         ...
       kind: Config
       users:
         ...
   ```

## 部署多 K8s 集群

### 部署 Kubeblocks operator

在 control plane 安装 KubeBlocks。

1. 安装 KubeBlocks.

   ```bash
   # multiCluster.kubeConfig 指定存放 data plane k8s kubeconfig 信息的 secret
   # multiCluster.contexts 指定 data plane K8s contexts
   kbcli kubeblocks install --version=0.9.0 --set multiCluster.kubeConfig=<secret-name> --set multiCluster.contexts=<contexts>
   ```

2. 查看安装状态，确保 KubeBlocks 安装完成。

   ```bash
   kbcli kubeblocks status
   ```

### RBAC

实例 workload 在 data plane 中运行时，需要特定的 RBAC 资源进行管理动作，因此需要预先在各 data plane 集群单独安装 KubeBlocks 所需的 RBAC 资源。

```bash
# 1. 从 control plane dump 所需的 clusterrole 资源：kubeblocks-cluster-pod-role
kubectl get clusterrole kubeblocks-cluster-pod-role -o yaml > /tmp/kubeblocks-cluster-pod-role.yaml

# 2. 编辑文件内容，去除不必要的 meta 信息（比如 UID、resource version），保留其他内容

# 3. Apply 文件内容到其他 data plane 集群
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-1
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-2
kubectl apply -f /tmp/kubeblocks-cluster-pod-role.yaml --context=k8s-3
```

### 网络

KubeBlocks 基于 K8s service 抽象来提供内外部的服务访问。对于 service 的抽象，集群内的访问 K8s 一般会有默认的实现，对于来自集群外的流量通常需要用户自己提供方案。而在多 K8s 形态下，无论是实例间的复制流量、还是客户端的访问流量，基本都属于“集群外流量”。因此为了让跨集群实例能够正常工作，网络部分一般需要进行一些额外的处理。

这里会以一组可选的方案为例，用来完整描述整个流程。实际使用中，用户可以根据自身集群和网络环境，选择适合的方案进行部署。

#### 东西向流量

##### 云上方案

云厂商提供的 K8s 服务一般都提供了内/外网 load balancer service 可供使用，这样可以直接基于 LB service 来构建副本之间的互访，简单易用。

##### 自建方案

东西向互访的自建方案以 Cillium Cluster Mesh 为例来进行说明，Cillium 的部署选择 overlay 模式，各 data plane 集群配置如下：

| Cluster | Context | Name  | ID | CIDR        |
|:-------:|:-------:|:-----:|:--:|:-----------:|
| 1       | k8s-1   | k8s-1 | 1  | 10.1.0.0/16 |
| 2       | k8s-2   | k8s-2 | 2  | 10.2.0.0/16 |
| 3       | k8s-3   | k8s-3 | 3  | 10.3.0.0/16 |

:::note

这里的 CIDR 是 Cilium Overlay 网络的地址，具体设置时要跟主机网络地址段区分开。

:::

***步骤：***

下述操作步骤相关命令，可以在各个集群分别执行（不需要指定 `--context` 参数），也可以在有三个 context 信息的环境里统一执行（分别指定 `--context` 参数）。

1. 安装 cilium，指定 cluster ID/name 和 cluster pool pod CIDR。可参考官方文档：[Specify the Cluster Name and ID](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#specify-the-cluster-name-and-id)。

   ```bash
   cilium install --set cluster.name=k8s-1 --set cluster.id=1 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.1.0.0/16 —context k8s-1
   cilium install --set cluster.name=k8s-2 --set cluster.id=2 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.2.0.0/16 —context k8s-2
   cilium install --set cluster.name=k8s-3 --set cluster.id=3 --set ipam.operator.clusterPoolIPv4PodCIDRList=10.3.0.0/16 —context k8s-3
   ```

2. 开启 Cilium Cluster Mesh，并等待其状态为 ready。这里以 NodePort 方式提供对 cluster mesh control plane 的访问，其他可选方式及具体信息请参考官方文档：[Enable Cluster Mesh](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#enable-cluster-mesh)。

   ```bash
   cilium clustermesh enable --service-type NodePort —context k8s-1
   cilium clustermesh enable --service-type NodePort —context k8s-2
   cilium clustermesh enable --service-type NodePort —context k8s-3
   cilium clustermesh status —wait —context k8s-1
   cilium clustermesh status —wait —context k8s-2
   cilium clustermesh status —wait —context k8s-3
   ```

3. 打通各集群，并等待集群状态为 ready。具体可参考官方文档：[Connect Clusters](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#connect-clusters)。

   ```bash
   cilium clustermesh connect --context k8s-1 --destination-context k8s-2
   cilium clustermesh connect --context k8s-1 --destination-context k8s-3
   cilium clustermesh connect --context k8s-2 --destination-context k8s-3
   cilium clustermesh status —wait —context k8s-1
   cilium clustermesh status —wait —context k8s-2
   cilium clustermesh status —wait —context k8s-3
   ```

4. （可选）可以通过 cilium-dbg 工具检查跨集群的 tunnel 情况。具体可参考官方文档：[cilium dbg](https://docs.cilium.io/en/stable/cmdref/cilium-dbg/)。

   ```bash
   cilium-dbg bpf tunnel list
   ```

5. （可选）集群连通性测试，可参考官方文档：[Test Pod Connectivity Between Clusters](https://docs.cilium.io/en/stable/network/clustermesh/clustermesh/#test-pod-connectivity-between-clusters)。

#### 南北向流量

南北向流量为客户端提供服务，需要每个 data plane 的 Pod 都有对外的连接地址，这个地址的实现可以是 NodePort、LoadBalancer 或者其他方案，我们以 NodePort 和 LoadBalancer 为例介绍。

如果客户端不具备读写路由能力，那在 Pod 地址之上，还需要提供读写分离地址，实现上可以用七层的 Proxy，四层的 SDN VIP，或者纯粹的 DNS。为了简化问题，此处先假设客户端具备读写路由能力，可以直接配置所有 Pod 连接地址。

##### NodePort

为每个 data plane 集群的 Pod 创建 NodePort Service，客户端使用用主机网络 IP 和 NodePort 即可连接。

##### LoadBalancer

此处以 MetalLB 提供 LoadBalancer Service 为例。

1. 准备 data plane 的 LB 网段，该网段需要跟客户端路由可达，并且不同 K8s 集群要错开

   | Cluster | Context | Name  | ID | CIDR        |
   |:-------:|:-------:|:-----:|:--:|:-----------:|
   | 1       | k8s-1   | k8s-1 | 1  | 10.4.0.0/16 |
   | 2       | k8s-2   | k8s-2 | 2  | 10.5.0.0/16 |
   | 3       | k8s-3   | k8s-3 | 3  | 10.6.0.0/16 |

2. 在所有 data plane 部署 MetalLB。

   ```bash
   helm repo add metallb https://metallb.github.io/metallb
   helm install metallb metallb/metallb
   ```

3. 等待相关 Pod 状态变为 ready。

   ```bash
   kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s
   ```

4. 在三个 K8s 集群执行以下 YAML 文件，请注意替换 `spec.addresses` 为对应 K8s 集群的 LB 网段。

   ```yaml
   apiVersion: metallb.io/v1beta1
   kind: IPAddressPool
   metadata:
     name: example
     namespace: metallb-system
   spec:
     addresses:
     - x.x.x.x/x
   ---
   apiVersion: metallb.io/v1beta1
   kind: L2Advertisement
   metadata:
     name: empty
     namespace: metallb-system
   ```

5. 为每个data plane 集群的 Pod 创建 LoadBalancer Service，拿到所有 VIP，即可供客户端连接。

## 验证

多集群实例的运行，各个副本之间的访问地址不能直接简单使用原 domain 内的地址（比如 Pod FQDN），需要显式的创建并配置使用跨集群的服务地址来进行通信，因此需要对引擎进行适配。

这里以社区版 etcd 为例来进行演示，相关适配的结果可以参考 [etcd 引擎](https://github.com/apecloud/kubeblocks-addons/blob/release-0.9/addons/etcd/templates/componentdefinition.yaml)。

### 创建实例

由于不同网络要求的配置不同，这里分别以云上和自建两种方式为例说明如果创建一个三副本的跨集群 etcd 实例。

#### 云上方案

这里以阿里云为例，其他厂商的配置可以参考官方文档。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # 可选：可以用该 annotation 显式指定当前实例要求分布的集群
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi # 云上 provisioning 要求的最小 size
        - name: peer
          serviceType: LoadBalancer
          annotations:
            # 如果运行在基于 LoadBalancer service 提供的互访方案上，这个 annotation key 为必填项
            apps.kubeblocks.io/multi-cluster-service-placement: unique
            # ACK LoadBalancer service 要求的 annotation key
            service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
          podService: true
```

如下示例展示了如何跨云厂商部署。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # 可选：可以用该 annotation 显式指定当前实例要求分布的集群
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi # 云上 provisioning 要求的最小 size
      services:
        - name: peer
          serviceType: LoadBalancer
          annotations:
            # 如果运行在基于 LoadBalancer service 提供的互访方案上，这个 annotation key 为必填项
            apps.kubeblocks.io/multi-cluster-service-placement: unique
            # ACK LoadBalancer service 要求的 annotation key。因为要跨云访问，因此需要配置为公网类型
            service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
            # VKE LoadBalancer service 要求的 annotation keys。因为要跨云访问，因此需要配置为公网类型
            service.beta.kubernetes.io/volcengine-loadbalancer-subnet-id: <subnet-id>
            service.beta.kubernetes.io/volcengine-loadbalancer-address-type: "PUBLIC"
          podService: true
```

#### 自建方案

该示例展示了如何在自建环境创建实例。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  namespace: default
  generateName: etcd
  annotations:
    # 可选：可以用该 annotation 显式指定当前实例要求分布的集群
    apps.kubeblocks.io/multi-cluster-placement: "k8s-1,k8s-2,k8s-3"
spec:
  terminationPolicy: WipeOut
  componentSpecs:
    - componentDef: etcd-0.9.0
      name: etcd
      replicas: 3
      resources:
        limits:
          cpu: 100m
          memory: 100M
        requests:
          cpu: 100m
          memory: 100M
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
      services:
        - name: peer
          serviceType: ClusterIP
          annotations:
            service.cilium.io/global: "true" # cilium clustermesh global service
          podService: true
```
