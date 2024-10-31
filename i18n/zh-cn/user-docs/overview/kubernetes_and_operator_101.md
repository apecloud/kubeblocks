---
title: Kubernetes 及 Operator 入门
description: K8s 入门知识 
keywords: [K8s, operator, concept]
sidebar_position: 3
---

# Kubernetes 及 Operator 入门

# K8s

什么是 Kubernetes？有人说它是一个容器编排系统，有人称它为分布式操作系统，有人认为它是一个多云的 PaaS（平台即服务）平台，还有人把它看作是构建 PaaS 解决方案的平台。

本文将介绍 Kubernetes 的关键概念和构建模块。

## K8s 控制平面（Control Plane）

Kubernetes 控制平面是 Kubernetes 的大脑和心脏。它负责管理整个集群的运行，包括处理 API 请求、存储配置数据以及确保集群处于期望的状态。关键组件包括 API 服务器（API Server，负责通信）、etcd（存储所有集群数据）、控制器管理器（Controller Manager，确保集群处于期望的状态）、调度器（Scheduler，将工作负载分配给节点）以及云控制器管理器（Cloud Controller Manager，管理与云平台的集成，如负载均衡、存储和网络）。这些组件共同协调容器在集群中的部署、扩展和管理。

## 节点（Node）

有人将 Kubernetes 描述为一个分布式操作系统，能够管理多个节点。节点是集群中的物理或虚拟机器，作为集群中的工作单元。每个节点都运行一些核心服务，包括容器运行时（如 Docker 或 containerd）、kubelet 和 kube-proxy。kubelet 确保容器按 Pod 中的配置运行，而 Pod 是 Kubernetes 中最小的可部署单元。kube-proxy 处理网络路由，维护网络规则，并允许 Pod 和服务之间的通信。节点提供运行容器化应用所需的计算资源，并由 Kubernetes 主节点管理，主节点负责分配任务、监控节点健康状况并保持集群的期望状态。

:::note

在某些语境中，同时讨论 Kubernetes 和数据库时，“节点”一词可能会产生混淆。在 Kubernetes 中，“节点”指的是集群中作为工作单元的物理或虚拟机器，用于运行容器化应用。而在 Kubernetes 中运行数据库时，“数据库节点”一般是指承载数据库实例的 Pod。

在 KubeBlocks 文档中，“节点”通常指的是数据库节点。如果我们指的是 Kubernetes 节点，我们将明确说明为“K8s 节点”以避免混淆。

:::

## kubelet

kubelet 是 Kubernetes 控制平面用于管理集群中每个节点的代理。它确保 Pod 中的容器按 Kubernetes 控制平面定义的方式运行。kubelet 持续监控容器的状态，确保它们健康且按预期运行。如果容器失败，kubelet 会根据指定的策略尝试重启它。

## Pod

在 Kubernetes 中，Pod 在某种程度上类似于虚拟机，但更轻量和专用。它是 Kubernetes 中最小的可部署单元。

Pod 代表一个或多个紧密耦合且需要协同工作的容器，以及共享存储（卷）、网络资源和运行容器的规范。这些容器可以使用本地地址（localhost）进行相互通信，并共享内存和存储等资源。

Kubernetes 动态管理 Pods，确保它们按指定的方式运行，并在失败时自动重启或替换 Pods。Pods 可以分布在多个节点上以实现冗余，因此在 Kubernetes 中部署和管理容器化应用（包括数据库）时，Pods 是基础构件。

## 存储类（Storage Class）

在为 Pod 内的工作负载（例如数据库）创建磁盘时，您可能需要指定磁盘介质的类型，无论是 HDD 还是 SSD。在云环境中，通常会有更多的选项可供选择。例如，AWS EBS 提供多种卷类型，如通用 SSD（gp2/gp3）、预配置 IOPS SSD（io1/io2）和优化吞吐量的 HDD（st1）。在 Kubernetes 中，您可以通过存储类（StorageClass）选择所需的磁盘类型。

## 持久卷声明（PVC）

在 Kubernetes 中，持久卷声明（PVC，Persisten Volume Claim）是用户对存储的请求。PVC 本质上是请求具有特定特性的存储的一种方式，例如存储类、大小和访问模式（如读写或只读）。PVC 使 Pods 能够使用存储，而无需了解底层基础设施的详细信息。

在 K8s 中，为了使用存储，用户会创建 PVC。创建 PVC 时，Kubernetes 会查找与请求匹配的 StorageClass。如果找到匹配的 StorageClass，Kubernetes 将根据定义的参数自动配置存储，无论是 SSD、HDD、EBS 还是 NAS。如果 PVC 未指定 StorageClass，Kubernetes 将使用默认的 StorageClass（如果已配置）来配置存储。

## 容器存储接口（CSI）

在 Kubernetes 中，通过容器存储接口（CSI，Container Storage Interface）提供各种存储类，CSI 负责为应用程序配置所需的底层存储“磁盘”。CSI 在 Kubernetes 中的功能类似于“磁盘驱动程序”，使平台能够适应并集成多种存储系统，如本地磁盘、AWS EBS 和 Ceph。这些存储类及其相关的存储资源由特定的 CSI 驱动程序提供，这些驱动程序处理与底层存储基础设施的交互。

CSI 是标准 API，使 Kubernetes 能够以一致和可扩展的方式与各种存储系统进行交互。由存储供应商或 Kubernetes 社区创建的 CSI 驱动程序向 Kubernetes 暴露了动态配置、附加、挂载和快照等基本存储功能。

当您在 Kubernetes 中定义一个 StorageClass 时，它通常会指定一个 CSI 驱动程序作为其配置器。这个驱动程序会根据 StorageClass 和相关持久卷声明（PVC）中的参数自动配置持久卷（PV），确保为您的应用程序提供适当类型和配置的存储，无论是 SSD、HDD 还是其他类型。

## 持久卷（PV）

在 Kubernetes 中，持久卷（PV，Persisten Volume）代表可以由多种系统（如本地磁盘、NFS 或基于云的存储，例如 AWS EBS、Google Cloud Persistent Disks）支持的存储资源，通常由不同的 CSI 驱动程序管理。

PV 有自己独立于 Pod 的生命周期，由 Kubernetes 控制平面进行管理。即使关联的 Pod 被删除，PV 也允许数据持续存在。PV 与持久卷声明（PVC）绑定，PVC 请求特定的存储特性，如大小和访问模式，确保应用程序获得所需的存储。

总之，PV 是实际的存储资源，而 PVC 是对存储的请求。通过 PVC 中的 StorageClass，PVC 可以绑定到由不同 CSI 驱动程序配置的 PV。

## 服务（Service）

在 Kubernetes 中，服务（Service）充当负载均衡器。它定义了一组逻辑上的 Pods，并提供了访问这些 Pods 的策略。由于 Pods 是短暂的，可能会动态创建和销毁，因此它们的 IP 地址并不稳定。服务通过提供一个稳定的网络端点（虚拟 IP 地址，称为 ClusterIP）解决了这个问题，该地址保持不变，使其他 Pods 或外部客户端能够与服务后面的 Pods 通信，而无需知道它们的具体 IP 地址。

服务支持不同类型：ClusterIP（内部集群访问）、NodePort（通过 `<NodeIP>:<NodePort>` 进行外部访问）、LoadBalancer（使用云提供商的负载均衡器公开服务）、ExternalName（将服务映射到外部 DNS）。

## ConfigMap

ConfigMap 用于以键值对的形式存储配置信息，通过 ConfigMap，您能够将配置与应用程序代码解耦。通过这种方式，您可以单独管理应用程序设置，并在多个环境中复用它们。ConfigMaps 可用于将配置数据注入到 Pods 中，作为环境变量、命令行参数或配置文件。它们提供了一种灵活且方便的方式来管理应用程序配置，而无需将值直接硬编码到您的应用程序容器中。

## Secret

Secret 用于存储敏感数据，例如密码、令牌或加密密钥。Secrets 将机密信息与应用程序代码分开管理，避免在容器镜像中暴露敏感数据。Kubernetes Secrets 可以作为环境变量注入到 Pods 中，或作为文件挂载，确保以安全和受控的方式处理敏感信息。

但 Secrets 默认情况下并不加密，它们只是进行 Base64 编码，并不能提供真正的加密。因此，仍需谨慎使用，确保适当的访问控制到位。

## Custom Resource Definition (CRD)

如果您希望使用 Kubernetes 管理数据库对象，则需要扩展 Kubernetes API，以描述您正在管理的数据库对象。这就是 CRD（Custom Resource Definition）机制的用途所在，CRD 支持定义特定用例的自定义资源，如数据库集群或备份，并以 K8s 原生的方式管理资源。

## 自定义资源（CR）

自定义资源（CR，Custom Resource）是 CRD 的实例。它表示扩展 Kubernetes API 的特定配置或对象。CR 允许您使用 Kubernetes 的原生工具定义和管理自定义资源，例如数据库或应用程序。一旦创建了 CR，Kubernetes 控制器或 Operator 会开始监控，并执行操作以保持所需状态。

CRD 和 CR 是开发 Kubernetes Operator 的基础。CRDs 通常用于实现自定义控制器或 Operator，允许持续监视 CR 的变化（例如，表示数据库集群的 CR），并自动执行相应的操作。

## 什么是 Kubernetes Operator?

Kubernetes Operator 是一种软件，通常由一个或多个控制器组成，自动管理复杂应用程序，通过将对面向定义资源（CR）所做的更改转换为面向 Kubernetes 原生对象（如 Pods、Services、PVCs、ConfigMaps 和 Secrets）的操作。

- 输入：用户修改 CR。
- 输出：根据管理应用程序的要求，对应修改底层 Kubernetes 资源或与外部系统（例如，写入数据库或调用 API）。

Operator 持续监视这些 Kubernetes 对象的状态。当发生变化（例如，Pod 崩溃）时，Operator 会自动采取纠正措施，例如重新创建 Pod 或调整流量（例如，更新 Service Endpoints）。

本质上，Kubernetes Operator 将复杂的操作知识封装到软件中，自动化任务，如部署、扩展、升级和备份，确保应用程序在无需人工干预的情况下持续保持所需状态。

## Helm 和 Helm Chart

Helm 是流行的 Kubernetes 包管理工具，帮助管理和部署应用程序。Helm 将所有必要的 Kubernetes 资源打包到 Helm Chart 中，支持通过单个命令（helm install）安装应用程序。Helm 还支持处理配置管理和更新（helm upgrade），简化应用程序的生命周期管理。

Helm Chart 的关键组件：

- 模板（Templates）：包含占位符的 YAML 文件，定义 Kubernetes 资源（如 Pods、Services 和 ConfigMaps）。
- values.yaml：用户可通过该 YAML 文件指定模板的默认值，用于自定义参数值。Helm 支持使用现有的 chart，通过 values.yaml 或命令行标志覆盖默认值，在不修改底层模板的情况下实现某种环境特定的配置。
- Chart.yaml：chart 的元数据，包括名称、版本和描述。

Helm 可与 CI/CD 工具（如 Jenkins、GitLab CI 和 GitHub Actions）集成。Helm 可以作为持续交付管道的一部分，用于自动化部署和回滚，确保应用程序在不同环境中部署的一致性。
