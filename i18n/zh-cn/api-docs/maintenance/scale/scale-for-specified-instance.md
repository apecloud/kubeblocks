---
title: 指定实例缩容
description: 如何执行指定实例缩容
keywords: [指定实例缩容]
sidebar_position: 1
sidebar_label: 指定实例缩容
---

# 指定实例缩容

## 为什么你想要对指定实例缩容

以往 KubeBlocks 最终生成的 Workload 是 StatefulSet，这是一把双刃剑。一方面，KubeBlocks 可以借助 StatefulSet 实现对数据库等有状态应用的管理，另一方面，这也导致 KubeBlocks 继承了 StatefulSet 的局限性。

其中的局限性之一是，在水平缩容场景下，StatefulSet 会按照 Ordinal 顺序从大到小依次下线 Pod。当 StatefulSet 中运行的是数据库时，这个局限性会使得数据库的可用性降低。

比如我们用 StatefulSet 管理了一个一主两从这样的三副本 PostgreSQL 数据库，这个 StatefulSet 的名字叫 `foo-bar`，运行一段时间后，名字为 `foo-bar-2` 的 Pod 成了主节点。

某一天，通过分析我们发现这个数据库的读负载并不是很高，为了节省资源，我们计划对这个数据库进行缩容，下线一个从节点。此时问题来了，按照 StatefulSet 的规则，我们只能下线 `foo-bar-2` 这个 Pod，但不幸的是它当前是主节点。此时我们有两个选择：要么直接下线 `foo-bar-2`，通过 failover 机制，在 `foo-bar-0` 和 `foo-bar-1` 中选择一个新的主节点；要么通过 switchover 机制，在下线 `foo-bar-2` 前，先将其切换成一个从节点。无论选择哪种方式，应用侧都会有一段时间无法写入。

另一个问题是，我们仍以上面的场景为例。如果 `foo-bar-1` 所在 Node 因为物理机故障，导致磁盘损坏，最终导致数据无法正常读写。按照数据库运维最佳实践，我们需要将 `foo-bar-1` 下线，并在其它健康 Node 上搭建新的副本，但基于 *StatefulSet* 来做这样的运维操作并不容易。

在 Kubernetes 社区中，我们也可以看到[类似场景的讨论](https://github.com/kubernetes/kubernetes/issues/83224)。所以 KubeBlocks 从 0.9 版本开始支持 *指定实例缩容* 特性，以解决上述问题。

## 对指定实例执行缩容

使用 `OfflineInstances` 字段，对指定实例执行缩容。

***步骤：***

使用 OpsRequest 指定需要缩容的实例。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: foo-horizontalscaling-
spec:
  clusterRef: foo
  force: false
  horizontalScaling:
  - componentName: bar
    replicas: 2
    offlineInstances: ["instancename"]
  ttlSecondsAfterSucceed: 0
  type: HorizontalScaling
```

OpsRequest Controller 会将请求中 `replicas` 和 `offlineInstances` 的值直接覆盖 Cluster 对象中对应的字段，最终由 Cluster Controller 完成名称为 `foo-bar-1` 的实例下线任务。

***实例：***

对于上述场景，PostgreSQL 集群的当前状态为：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: foo
spec:
  componentSpecs:
  - name: bar
    replicas: 3
# ...
```

当需要将集群缩容到 2 个副本，并指定下线 `foo-bar-1` 时，集群对象可做如下更新：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: foo
spec:
  componentSpecs:
  - name: bar
    replicas: 2
    offlineInstances: ["foo-bar-1"]
# ...
```

KubeBlocks 在处理上述 Spec 时，会将集群缩容到 2 个副本，并将 Ordinal 为 1 而不是 2 的 实例下线。最终，集群中留下的实例为： `foo-bar-0` 和 `foo-bar-2`。
