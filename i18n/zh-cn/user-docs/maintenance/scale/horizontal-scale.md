---
title: 水平扩缩容
description: 如何对集群和实例进行水平扩缩容
keywords: [水平扩缩容, 水平伸缩]
sidebar_position: 2
sidebar_label: 水平扩缩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 水平扩缩容

从 v0.9.0 开始，KubeBlocks 支持 ScaleIn 和 ScaleOut 两类操作，同时支持副本（replcia）和具体实例（instance）的扩缩容。

- ScaleIn：支持水平缩容指定副本及指定实例下线。
- ScaleOut: 支持水平扩容指定副本及指定实例重新上线。

可通过声明式 API 或创建 OpsRequest 的方式执行水平扩缩容：

- 以声明式 API 的方式修改集群

    通过声明式 API 方式，可直接修改 Cluster YAML 文件来指定每个组件的副本数量和实例模板。如果新的副本数量大于当前 Pod 的数量，则表示扩展（scale-out）；相反，如果新的副本数量少于当前 Pod 的数量，则表示缩容（scale-in）。

- 创建 OpsRequest

    另一种方法是在 OpsRequest 中指定副本数量的增量。控制器（controller）将根据集群组件中当前 Pod 的数量和增量值计算所需的副本数量，并相应地执行扩缩容操作。

:::note

- 在并发修改的情况下，例如多个控制器同时修改 Pod 的数量，计算出的 Pod 数量可能会不准确。你可以在客户端确保操作顺序，或者设置 `KUBEBLOCKS_RECONCILE_WORKERS=1`。
- 如果正在使用声明式 API 进行扩缩容操作，该操作将被终止。
- 从 v0.9.0 开始，MySQL 和 PostgreSQL 集群在进行水平扩缩容后，KubeBlocks 会根据新的规格自动匹配合适的配置模板。这因为 KubeBlocks 在 v0.9.0 中引入了动态配置功能。该功能简化了配置参数的过程，节省了时间和精力，并减少了由于配置错误引起的性能问题。有关详细说明，请参阅[配置](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md)。

:::

## 为什么需要指定实例扩缩容

早期版本中，KubeBlocks 最终生成的 Workload 是 *StatefulSet*，这是一把双刃剑。一方面，KubeBlocks 可以借助 *StatefulSet* 实现对数据库等有状态应用的管理，另一方面，这也导致 KubeBlocks 继承了其局限性。

其中的局限性之一是，在水平缩容场景下，*StatefulSet* 会按照 *Ordinal* 顺序从大到小依次下线 Pod。当 *StatefulSet* 中运行的是数据库时，这个局限性会使得数据库的可用性降低。

另一个问题是，我们仍以上面的场景为例。某一天，Pod 所在 Node 因为物理机故障，导致磁盘损坏，最终导致数据无法正常读写。按照数据库运维最佳实践，我们需要将受损的 Pod 下线，并在其它健康 Node 上搭建新的副本，但基于 *StatefulSet* 来做这样的运维操作并不容易。在 Kubernetes 社区中，我们也可以看到[类似场景的讨论](https://github.com/kubernetes/kubernetes/issues/83224)。

为了解决上述局限性，KubeBlocks 从 0.9 版本开始使用 *InstanceSet* 替代 *StatefulSet*。*InstanceSet* 是一个通用 Workload API，负责管理一组实例。引入 *InstanceSet* 后，KubeBlocks 支持了“指定实例缩容”特性，以提升可用性。

## 开始之前

本文档以 Redis 为例进行演示，以下为原组件拓扑。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis
  namespace: kubeblocks-cloud-ns
spec:
  componentSpecs:
  - name: proxy
    componentDef: redis-proxy
    replicas: 10
    instances:
    - name: proxy-2c
      replicas: 3
      resources:
        limits:
          cpu: 2
          memory: 4Gi
    resources:
      limits:
        cpu: 4
        memory: 8Gi
    offlineInstances:
    - redis-proxy-proxy-2c4g-0
    - redis-proxy-proxy-2c4g-1     
```

查看集群状态是否为 `Running`，否则，后续操作可能失败。

```bash
kubectl get cluster redis
```

## 水平缩容

### 示例 1

本示例演示了如何应用 OpsRequest 将副本数缩容至 8。本示例将按照默认规则删除 pod，未指定任何实例。

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 2  
EOF 
```

### 示例 2

示例 2 演示了如何应用 OpsRequest 将副本数缩容至 8，并指定仅缩容 2 个 2C4G 的实例。

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 2
      instances: 
      - name: proxy-2c4g
        replicaChanges: 2
EOF
```

### 示例 3

示例 3 演示了如何下线指定实例。本示例中，`replicas` 和 `instance replicas` 的数量都会减少 1。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      onlineInstancesToOffline:
      - redis-proxy-proxy-2c4g-2
  ttlSecondsAfterSucceed: 0
EOF
```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

您也可通过修改集群 YAML 文件指定实例下线。

```yaml
kubectl edit cluster redis
```

在编辑器中修改 `spec.componentSpecs.replicas` 及 `spec.componentSpecs.offlineInstances` 的参数值。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis
spec:
  componentSpecs:
  - name: proxy
    replicas: 9
    offlineInstances: ["redis-proxy-proxy-2c4g-2"]
...
```

</TabItem>
</Tabs>

## 水平扩容

后续实例演示了如何对副本和实例进行水平扩容。如果您只想对副本进行扩容，仅需修改 `replicaChanges`，例如，

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut: 
      replicaChanges: 6
EOF
```

### 示例 1

示例 1 演示了如何通过应用 OpsRequest 将副本数扩容至 16。

新创建的实例如下：

3 个 4C8G 的实例: `redis-proxy-7`, `redis-proxy-8`, `redis-proxy-9`
3 个 2C4G 的实例: `redis-proxy-2c4g-5`, `redis-proxy-2c4g-6`, `redis-proxy-2c4g-7`

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut: 
      replicaChanges: 6
      instances: 
      - name: proxy-2c4g
        replicasChanges: 3
EOF
```

### 示例 2

示例 2 演示了如何通过应用 OpsRequest 将副本数扩容至 16，同时其中 3 个为 8C16G 的 proxy。

新创建的实例如下：

4C8G: `redis-proxy-7`, `redis-proxy-8`, `redis-proxy-9`
8C16G: `redis-proxy-8c16g-0`, `redis-proxy-8c16g-1`, `redis-proxy-8c16g-2`

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut:
      replicaChanges: 6 
      newInstances:
      - name: proxy-8c16g
        replicas: 3
        resources:
          limits:
            cpu: 8
            memory: 16Gi  
EOF
```

### 示例 3

示例 3 演示了如何通过应用 OpsRequest 将已下线的实例重新添加到集群。该运维任务执行完成后，副本数为 12。

本示例中添加的 pod 为  `redis-proxy-2c4g-0` 和 `redis-proxy-2c4g-1`。

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut:
      offlineInstancesToOnline:
      - redis-proxy-proxy-2c4g-0
      - redis-proxy-proxy-2c4g-1     
EOF   
```

## 水平扩缩容

### 示例 1

示例 1 演示了如何通过应用 OpsRequest 实现指定实例下线并创建新实例。

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      onlineInstancesToOffline:
      - redis-proxy-proxy-2c4g-2  
    scaleOut:
      instances:
      - name: 2c4g
        replicaChanges: 1
EOF
```

### 示例 2

示例 2 演示了如何通过应用 OpsRequest 将副本数缩容至 8 并将已下线的实例重新加入到集群。

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 4
      instances:
      - name: 2c4g
        replicaChanges: 2
    scaleOut: 
      offlineInstancesToOnline:
      - redis-proxy-proxy-2c4g-0
      - redis-proxy-proxy-2c4g-1
EOF
```

## 处理快照异常

如果在水平扩容过程中出现 `STATUS=ConditionsError`，你可以从 `cluster.status.condition.message` 中找到原因并进行故障排除。如下所示，该例子中发生了快照异常。

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/reids-redis-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***原因***

此异常发生的原因是未配置 `VolumeSnapshotClass`。可以通过配置 `VolumeSnapshotClass` 解决问题。

但此时，水平扩容仍然无法继续运行。这是因为错误的备份（volumesnapshot 由备份生成）和之前生成的 volumesnapshot 仍然存在。需删除这两个错误的资源，KubeBlocks 才能重新生成新的资源。

***步骤：***

1. 配置 VolumeSnapshotClass。

   ```bash
   kubectl create -f - <<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotClass
   metadata:
     name: csi-aws-vsc
     annotations:
       snapshot.storage.kubernetes.io/is-default-class: "true"
   driver: ebs.csi.aws.com
   deletionPolicy: Delete
   EOF
   ```

2. 删除错误的备份和 volumesnapshot 资源。

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=redis
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=redis
   ```

***结果***

删除备份和 volumesnapshot 后，水平扩容继续进行，集群恢复到 `Running` 状态。
