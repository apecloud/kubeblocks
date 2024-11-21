---
title: Redis Cluster 模式
description: Redis Cluster 模式概览及基本操作
keywords: [redis, redis cluster, 功能]
sidebar_position: 7
sidebar_label: Redis Cluster 模式
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Redis Cluster 模式

虽然 Redis Sentinel 集群提供了出色的故障转移支持，但其本身不提供数据分片，所有数据仍然驻留在单个 Redis 实例上，并受到该实例的内存和性能限制，因此可能会影响系统在处理大型数据集和高读/写操作时的水平扩展能力。

KubeBlocks 现已支持 Redis Cluster 模式。该模式不仅允许更大的内存分布，还支持并行处理，从而显著地提高了数据密集型操作的性能。本文档将简要介绍 Redis Cluster 模式及其基本操作。

## 什么是 Redis Cluster 模式

Redis Cluster 是 Redis 数据库的一种分布式部署模式，用于在多个节点上水平扩展数据存储和提高系统的可用性。

在 Redis Cluster 中，集群通过分片（sharding）模式来对数据进行管理，并具备分片间数据复制、故障转移和流量调度的能力。这种分片机制允许将大量数据分散存储在不同的节点上，从而实现数据的横向扩展和负载均衡。

Redis Cluster 采用主从复制的方式保证数据的高可用性。每个主节点可以有一个或多个从节点，从节点复制主节点的数据并提供读取服务。当主节点发生故障时，从节点可以自动接管主节点的功能，并继续提供服务，从而实现故障转移和容错性。

Redis Cluster 还提供集群的节点间通信和数据迁移机制。当集群中的节点发生变更（如新增节点、节点故障、节点移除）时，Redis Cluster 会自动进行数据迁移和重新分片，以保持数据的平衡和可用性。

## 基本运维操作

下面简单介绍 Redis Cluster 的基本运维操作。

### 开始之前

* [安装 KubeBlocks](./../installation/install-kubeblocks.md)。
   - KubeBlocks 及 Addon 的版本都需要 0.9 版本以上。
* 确保 Redis 引擎已启用。
* 查看可用于创建集群的数据库类型和版本。

  查看 redis 组件定义是否可用。组件定义用于描述和定义数据库集群的组件，通常呈现诸如名称、类型、版本、状态等基本信息。

  ```bash
  kubectl get componentdefinition redis-cluster-7.0
  >
  NAME                SERVICE         SERVICE-VERSION   STATUS      AGE
  redis-cluster-7.0   redis-cluster   7.0.6             Available   33m
  ```

* 为了保持隔离，本文档中创建一个名为 demo 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### 创建集群

因为 Redis Cluster 是基于最新的 ShardingSpec API 创建的，目前只支持通过 helm 或者 kubectl apply yaml 创建。

:::note

Redis Cluster 至少需要三个分片，所以分片数量不能小于 3。

:::

<Tabs>

<TabItem value="Helm" label="Helm" default>

```bash
# 示例一：生产一个具有三个分片，每个分片一个副本（一主一备）的 Redis Cluster 集群
helm install redisc xxxxx(chart remote address) --set mode=cluster --set redisCluster.shardCount=3 --set replicas=2 -n demo

# 示例二：生产一个具有三个分片，每个分片一个副本（一主一备）， 并开启 NodePort 访问的 Redis cluster 集群
helm install redisc xxxxx(chart remote address) --set mode=cluster --set nodePortEnabled=true --set redisCluster.shardCount=3 --set replicas=2 -n demo
```

</TabItem>

<TabItem value="YAML" label="YAML">

```bash
kubectl apply -f redis-cluster-example.yaml
```

**示例 1: 未开启 NodePort 的 Redis Cluster**

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redisc
  namespace: demo
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterVersionRef: redis-7.0.6
  shardingSpecs:
  - name: shard
    # 指定分片数量，不能小于3
    shards: 3
    template:
      componentDef: redis-cluster-7
      name: redis
      replicas: 2
      # 指定单个分片的资源
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: 7.0.6
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 20Gi
  terminationPolicy: Delete
```

**示例 2: 开启 NodePort 的 Redis Cluster**

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redisc
  namespace: demo
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterVersionRef: redis-7.0.6
  shardingSpecs:
  - name: shard
    # 指定分片数量，不能小于 3
    shards: 3
    template:
      componentDef: redis-cluster-7.0
      name: redis
      replicas: 2
      # 指定单个分片的资源
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: 7.0.6
      # 指定开启 NodePort
      services:
      - name: redis-advertised
        podService: true
        serviceType: NodePort
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 20Gi
  terminationPolicy: Delete
```

</TabItem>

</Tabs>

创建后，观察集群状态变成 `running` 后，并通过 `kubectl get component -n demo | grep redisc-shard` 命令看到对应的角色。

```bash
kubectl get component -n demo | grep redisc-shard
>
redisc-shard-5hn            redis-cluster          Running   79m
redisc-shard-7zk            redis-cluster          Running   79m
redisc-shard-5tw            redis-cluster          Running   79m
```

### 连接集群

<Tabs>

<TabItem value="SDK" label="SDK" default>

Redis Cluster 需要使用特定的客户端 SDK 进行连接，目前不同的主流编程语言都有对应的 Redis Cluster 客户端 SDK 实现，可以根据实际需求选择。例如 Java 的 Jedis 或者 Lettuce，Golang 的 go-redis 等，这部分可以参考对应 SDK 的文档。

需要注意的是，在使用 SDK 时需要在 SDK 中配置 Redis Cluster 各个节点的地址，根据上述创建集群时是否开启 NodePort 的不同，分为两种情况：

**示例 1: 未开启 Nodeport**

配置每个 Shard 对应 Pod 对应的 headless 地址。

```bash
redisc-shard-qst-0.redisc-shard-qst-headless:6379
redisc-shard-qst-1.redisc-shard-qst-headless:6379
redisc-shard-kr2-0.redisc-shard-kr2-headless:6379
redisc-shard-kr2-1.redisc-shard-kr2-headless:6379
redisc-shard-mv6-0.redisc-shard-mv6-headless:6379
redisc-shard-mv6-1.redisc-shard-mv6-headless:6379
```

**示例 2: 开启 NodePort**

Configure each Pod's HostIP and the corresponding NodePort.

```bash
# 例如 172.18.0.3 是 redisc-shard-qst-0 所在节点的主机 ip，32269 是 redis 6379 服务端口映射的 NodePort
172.18.0.3:32269 
172.18.0.4:32051
172.18.0.5:32650
172.18.0.6:30151
172.18.0.7:31603
172.18.0.8:31718
```

</TabItem>

<TabItem value="直连特定节点" label="直连特定节点">

:::note

直连特定节点仅用于测试。

:::

简单验证连通性时，可以通过 `kubectl` 命令，登录到集群中的某个 Pod，查看集群状态。

```bash
# 登录到 Pod 中
➜  ~ kubectl exec -it redisc-shard-qst-0 -c redis-cluster -- bash
root@redisc-shard-qst-0:/#

# 查看集群拓扑信息
root@redisc-shard-qst-0:/# redis-cli -a $REDIS_DEFAULT_PASSWORD cluster nodes
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
f2f729eb9074d42dc58ba544f35be0b6652134c2 172.18.0.3:32650@31171,redisc-shard-mv6-1.redisc-shard-mv6-headless.default.svc slave 04e309001ce12857558ab721b47ce802e15b84f4 0 1713927855356 3 connected
04e309001ce12857558ab721b47ce802e15b84f4 172.18.0.3:32051@32632,redisc-shard-mv6-0.redisc-shard-mv6-headless.default.svc master - 0 1713927854000 3 connected 12288-16383
11da6bb8875ed4b3787888fec56bf9212e664246 172.18.0.3:32663@32115,redisc-shard-qst-1.redisc-shard-qst-headless.default.svc slave 7f062b0b5fdd0d09ea70920a5ad3612a9d217c9d 0 1713927853848 2 connected
7f062b0b5fdd0d09ea70920a5ad3612a9d217c9d 172.18.0.3:30649@30003,redisc-shard-qst-0.redisc-shard-qst-headless.default.svc myself,master - 0 1713927853000 2 connected 6827-10922
a04da6edbf99faff53b2b94d0fc64f378d27b4aa 172.18.0.3:30151@31667,redisc-shard-kr2-0.redisc-shard-kr2-headless.default.svc master - 0 1713927853000 1 connected 1365-5460
d392d129e27ced8603681cd666f705bfad7ae91a 172.18.0.3:31718@30646,redisc-shard-fm6-1.redisc-shard-fm6-headless.default.svc slave 568bb6ced7f186caddb2b2b7ab560e61ff95438c 0 1713927853036 4 connected
b695dbac46efd9ec24ea608358f92dfd749e8e71 172.18.0.3:31603@30000,redisc-shard-kr2-1.redisc-shard-kr2-headless.default.svc slave a04da6edbf99faff53b2b94d0fc64f378d27b4aa 0 1713927854350 1 connected
568bb6ced7f186caddb2b2b7ab560e61ff95438c 172.18.0.3:32269@31457,redisc-shard-fm6-0.redisc-shard-fm6-headless.default.svc master - 0 1713927853340 4 connected 0-1364 5461-6826 10923-12287
```

</TabItem>

</Tabs>

### 分片扩缩容

:::note

1. 目前仅支持一次性扩容或者缩容一个分片节点，如需要扩缩多个分片，需要串行操作，后续版本会进行优化。

2. 暂不支持通过 OpsRequest 操作对分片进行扩缩容，后续版本会支持。

:::

<Tabs>

<TabItem value="kubectl patch" label="kubectl patch" default>

使用 kubectl patch 更新 shards 字段，对分片进行扩缩容。

```bash
# 扩容成 4 分片
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/shards", "value":4}]' -n demo

# 重新缩容成 3 分片
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/shards", "value":3}]' -n demo
```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

使用 `kubectl edit` 直接编辑集群 YAML 文件，修改 `spec.ShardingSpecs[0].shards` 字段的值。

```bash
kubectl edit cluster redisc
```

</TabItem>

</Tabs>

### 分片副本扩缩容

分片副本数量对所有分片生效。

<Tabs>

<TabItem value="kubectl patch" label="kubectl patch" default>

使用 `kubectl patch` 更新 `replicas` 字段，对分片副本进行扩缩容。

```bash
# 扩容至 3 副本（每个分片中含有一主两备）
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/template/replicas", "value": 3}]' -n default
```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

使用 `kubectl edit` 直接编辑集群 YAML 文件，修改 `spec.ShardingSpecs[0].template.replicas` 字段的值。

```bash
kubectl edit cluster redisc
```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

应用 OpsRequest 进行分片副本扩缩容。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: horizontal-scaling-redisc
spec:
  clusterName: redisc
  horizontalScaling:
  - componentName: shard
    replicas: 3
  type: HorizontalScaling
```

</TabItem>

</Tabs>

### 资源变配

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: vertical-scaling-redisc
spec:
  clusterName: redisc
  verticalScaling:
  - componentName: shard
    limits: 
      cpu: 2
      memory: 4Gi
    requests:
      cpu: 2
      memory: 4Gi  
  type: VerticalScaling
```

### 磁盘扩容

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: volume-expand-redisc
spec:
  clusterName: redisc
  volumeExpansion:
  - componentName: shard
    volumeClaimTemplates:
    - name: data
      storage: 200Gi
  type: VolumeExpansion
```

### 重启

当前暂不支持重启 Redis Cluster。

### 停止/启动

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: stop-redisc
spec:
  clusterName: redisc
  type: Stop
```

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: start-redisc
spec:
  clusterName: redisc
  type: Start
```

### 备份恢复

备份集群：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: backup-redisc-1
spec:
  clusterName: redisc
  type: Backup
  backup:
    backupMethod: datafile
    backupName: backup-redisc-1
    backupPolicyName: redisc-redis-backup-policy
    deletionPolicy: Delete
```

恢复集群：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: restore-redisc-backup
spec:
  clusterName: restore-redisc
  type: Backup
  restore:
    backupName: backup-redisc-1
```
