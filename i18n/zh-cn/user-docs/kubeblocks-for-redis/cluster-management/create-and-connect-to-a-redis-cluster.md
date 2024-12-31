---
title: 创建并连接到 Redis 集群
description: 如何创建并连接到 Redis 集群
keywords: [redis, 创建 Redis 集群, 连接到 Redis 集群, 集群, redis sentinel]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接到 Redis 集群

本文档展示如何创建并连接到一个 Redis 集群。

## 创建 Redis 集群

### 开始之前

* 如果您想通过 `kbcli` 创建并连接 MongoDB 集群，请先[安装 kbcli](./../../installation/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* 确保 Redis 引擎已启用。如果未启用，请先[启用该引擎](./../../installation/install-addons.md)。
  
  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io redis
  >
  NAME      TYPE   VERSION   PROVIDER   STATUS    AGE
  redis     Helm                        Enabled   61m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL   
  ...
  redis                     Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* 查看可用于创建集群的数据库类型和版本。

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get clusterdefinition redis
  >
  NAME    TOPOLOGIES                                              SERVICEREFS   STATUS      AGE
  redis   replication,replication-twemproxy,standalone                          Available   16m
  ```

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   16m
  redis-7.2.4   redis                Available   16m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list
  >
  NAME               TOPOLOGIES                                              SERVICEREFS   STATUS      AGE
  redis              replication,replication-twemproxy,standalone                          Available   16m

  kbcli clusterversion list
  >
  NAME                 CLUSTER-DEFINITION   STATUS      IS-DEFAULT   CREATED-TIME
  redis-7.0.6          redis                Available   false        Sep 27,2024 11:36 UTC+0800
  redis-7.2.4          redis                Available   false        Sep 27,2024 11:36 UTC+0800
  ```

  </TabItem>

  </Tabs>

* 为了保持隔离，本文档中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### 创建集群

KubeBlocks 支持创建两种 Redis 集群：单机版（Standalone）和主备版（Replication）。Redis 单机版仅支持一个副本，适用于对可用性要求较低的场景。 对于高可用性要求较高的场景，建议创建主备版集群，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。如果您只有一个节点可用于部署多副本集群，可设置 `spec.schedulingPolicy` 或 `spec.componentSpecs.schedulingPolicy`，具体可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy)。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. 创建 Redis 集群。

   KubeBlocks 通过 `Cluster` 定义集群。以下是创建 Redis 集群的示例。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: redis
     topology: replication
     componentSpecs:
       - name: redis
         serviceVersion: "7.2.4"
         disableExporter: false
         replicas: 2
         resources:
           limits:
             cpu: '0.5'
             memory: 0.5Gi
           requests:
             cpu: '0.5'
             memory: 0.5Gi
         volumeClaimTemplates:
           - name: data
             spec:
               storageClassName: ""
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
       - name: redis-sentinel
         replicas: 3
         resources:
           limits:
             cpu: '0.5'
             memory: 0.5Gi
           requests:
             cpu: '0.5'
             memory: 0.5Gi
         volumeClaimTemplates:
           - name: data
             spec:
               storageClassName: ""
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
   EOF
   ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-a-redis-cluster.md#终止策略)。 |
   | `spec.clusterDef` | 指定了创建集群时要使用的 ClusterDefinition 的名称。**注意**：**请勿更新此字段**。创建 Redis 集群时，该值必须为 `redis`。 |
   | `spec.topology` | 指定了在创建集群时要使用的 ClusterTopology 的名称。 |
   | `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
   | `spec.componentSpecs.serviceVersion`  | 定义了 component 部署的服务版本。有效值为 [7.0.6,7.2.4]。 |
   | `spec.componentSpecs.disableExporter` | 定义了是否在 component 无头服务（headless service）上标注指标 exporter 信息，是否开启监控 exporter。有效值为 [true, false]。 |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |
   | `spec.componentSpecs.volumeClaimTemplates` | PersistentVolumeClaim 模板列表，定义 component 的存储需求。 |
   | `spec.componentSpecs.volumeClaimTemplates.name` | 引用了在 `componentDefinition.spec.runtime.containers[*].volumeMounts` 中定义的 volumeMount 名称。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | 定义了 StorageClass 的名称。如果未指定，系统将默认使用带有 `storageclass.kubernetes.io/is-default-class=true` 注释的 StorageClass。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | 可按需配置存储容量。 |

   您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

   监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   执行以下命令，查看已创建 Redis 集群的 YAML 文件。

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. 验证集群是否创建成功。

   ```bash
   kubectl get cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建 Redis 集群。

   ```bash
   kbcli cluster create redis mycluster -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create redis --help

   kbcli cluster create redis -h
   ```

   如果您只有一个节点用于部署多副本集群，可在创建集群时配置集群亲和性，配置 `--pod-anti-afffinity`, `--tolerations` 和 `--topology-keys`。但需要注意的是，生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   demo        redis                          Delete               Running    Sep 29,2024 09:46 UTC+0800
   ```

</TabItem>

</Tabs>

## 连接到 Redis 集群

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MySQL 集群的连接凭证。该 Secret 包含以下 key：

* `username`：Redis 集群的根用户名。
* `password`：根用户的密码。
* `port`：Redis 集群的端口。
* `host`：Redis 集群的主机。
* `endpoint`：Redis 集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   default

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   5bv7czc4
   ```

2. 使用用户名和密码，进入 Pod `mycluster-redis-0` 并连接到数据库。

   ```bash
   kubectl exec -ti -n demo mycluster-redis-0 -- bash

   root@mycluster-redis-0:/# redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

还可以使用端口转发在本地计算机上连接数据库。

1. 通过端口转发服务。
  
   ```bash
   kubectl port-forward -n demo svc/mycluster-redis 6379:6379
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster  --namespace demo
```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
