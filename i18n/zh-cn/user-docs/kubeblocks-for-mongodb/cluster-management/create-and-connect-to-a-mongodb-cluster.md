---
title: 创建并连接到 MongoDB 集群
description: 如何创建并连接到 MongoDB 集群
keywords: [mongodb, 创建 MongoDB 集群, 连接 MongoDB 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接到 MongoDB 集群

本文档展示如何创建并连接到一个 MongoDB 集群。

## 创建 MongoDB 集群

### 开始之前

* 如果您想通过 `kbcli` 创建并连接 MongoDB 集群，请先[安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 安装 KubeBlocks，可通过 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks.md)。
* 确保 MongoDB 引擎已启用。如果引擎未安装/启用，可参考相关文档，安装/启用该引擎，可通过 [kbcli](./../../installation/install-with-kbcli/install-addons.md) 或者 [Helm](./../../installation/install-with-helm/install-addons.md) 操作。

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get addons.extensions.kubeblocks.io mongodb
   >
   NAME      TYPE   VERSION   PROVIDER   STATUS    AGE
   mongodb   Helm                        Enabled   23m
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli addon list
   >
   NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   
   ...
   mongodb                        Helm   Enabled                   true
   ...
   ```

   </TabItem>

   </Tabs>

* 查看可用于创建集群的数据库类型和版本。

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get clusterdefinition mongodb
   >
   NAME      TOPOLOGIES   SERVICEREFS   STATUS      AGE
   mongodb                              Available   23m
   ```

   ```bash
   kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli clusterdefinition list

   kbcli clusterversion list
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

KubeBlocks 支持创建两种 MongoDB 集群：单机版（Standalone）和主备版（ReplicaSet）。MongoDB 单机版仅支持一个副本，适用于对可用性要求较低的场景。对于高可用性要求较高的场景，建议创建主备版集群，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. 创建 MongoDB 集群。

   KubeBlocks 通过 `Cluster` 定义集群。以下是创建 MongoDB 单机版的示例。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     affinity:
       podAntiAffinity: Preferred
       tenancy: SharedNode
       topologyKeys:
       - kubernetes.io/hostname
     componentSpecs:
     - componentDef: mongodb
       name: mongodb
       replicas: 3
       resources:
         limits:
           cpu: "0.5"
           memory: 0.5Gi
         requests:
           cpu: "0.5"
           memory: 0.5Gi
       serviceVersion: 6.0.16
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
     terminationPolicy: Delete
   EOF
   ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-a-mongodb-cluster.md#终止策略)。|
   | `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
   | `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
   | `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
   | `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
   | `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
   | `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition mongodb -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
   | `spec.componentSpecs.name`            | 定义了 component 的名称。  |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

   KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   执行以下命令，查看已创建的 MongoDB 集群：

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. 验证集群是否创建成功。

   ```bash
   kubectl get cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建 MongoDB 集群。

   ```bash
   kbcli cluster create mongodb mycluster -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create mongodb --help
   kbcli cluster create mongodb -h
   ```

   例如，您可使用 `--mode` 指定集群形态，创建 MongoDB 主备版。

   ```bash
   kbcli cluster create mongodb mycluster --mode replicaset -n demo
   ```

   如果只有一个节点用于部署主备版集群，请在创建集群时将 `availability-policy` 设置为 `none`。但需要注意的是，生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

   ```bash
   kbcli cluster create mongodb mycluster --mode replicaset --availability-policy none -n demo
   ```

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        mongodb              mongodb-7.0.12    Delete               Running   Sep 19,2024 16:01 UTC+0800
   ```

</TabItem>

</Tabs>

## 连接到 MongoDB 集群

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MongoDB 集群的连接凭证。该 Secret 包含以下 key：

* `username`：集群的根用户名。
* `password`：根用户的密码。
* `port`：集群的端口。
* `host`：集群的主机。
* `endpoint`：集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   266zfqx5
   ```

2. 使用用户名和密码，进入 Pod `mycluster-mongodb-0` 并连接到数据库。

   ```bash
   kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

还可以使用端口转发在本地计算机上连接数据库。

1. 通过端口转发服务。

   ```bash
   kubectl port-forward -n demo svc/mycluster-mongodb 27017:27017  
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster --namespace demo
```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
