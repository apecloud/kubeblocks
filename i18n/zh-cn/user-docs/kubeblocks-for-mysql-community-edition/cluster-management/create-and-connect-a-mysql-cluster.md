---
title: 创建并连接 MySQL 集群
description: 如何创建并连接到 MySQL 集群
keywords: [mysql, 创建 mysql 集群, 连接 mysql 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接 MySQL 集群

本文档展示如何创建并连接到一个 MySQL 集群。

## 创建 MySQL 集群

### 开始之前

* 如果您想通过 `kbcli` 创建并连接 MySQL 集群，请先[安装 kbcli](./../../installation/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* 确保 MySQL 引擎已启用。KubeBlocks 默认已安装 MySQL，如果您在安装 KubeBlocks 关闭/卸载了该引擎，可参考相关文档，再次[启用/安装该引擎](./../../installation/install-addons.md)。
  
  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io mysql
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  mysql   Helm                        Enabled   27h
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  mysql                          0.9.1           community   Enabled    true
  ...
  ```

  </TabItem>

  </Tabs>

* 查看可用于创建集群的数据库类型和版本。

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  确保 `mysql` cluster definition 已安装。

  ```bash
  kubectl get clusterdefinition mysql
  >
  NAME             TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mysql                                       Available   85m
  ```

  查看可用的集群版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mysql
  >
  NAME           CLUSTER-DEFINITION   STATUS      AGE
  mysql-5.7.44   mysql                Available   27h
  mysql-8.0.33   mysql                Available   27h
  mysql-8.4.2    mysql                Available   27h
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list
  kbcli clusterversion list
  ```

  </TabItem>

  </Tabs>

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种类型的 MySQL 集群：单机版（Standalone）和主备版（Replication）。单机版仅支持一个副本，适用于对可用性要求较低的场景。主备版包含两个副本，适用于对高可用性要求较高的场景。为了确保高可用性，所有的副本都默认分布在不同的节点上。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. 创建 MySQL 集群。
   
   KubeBlocks 通过 `Cluster` 定义集群。以下是创建 MySQL 主备版的示例。

   如果您只有一个节点可用于部署集群版，可将 `spec.affinity.topologyKeys` 设置为 `null`。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     componentSpecs:
       - name: mysql
         componentDef: "mysql-8.0" 
         serviceVersion: 8.0.35
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
   EOF
   ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-mysql-cluster.md#终止策略)。 |
   | `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
   | `spec.componentSpecs.componentDef` | 指定了定义 component 特性和行为的 ComponentDefinition 自定义资源(CR)。支持三种不同的 ComponentDefinition 指定方式：正则表达式（推荐）、完整名称（推荐）和名称前缀。 |
   | `spec.componentSpecs.serviceVersion`  | 定义了 component 部署的服务版本。有效值为[8.0.30,8.0.31,8.0.32,8.0.33,8.0.34,8.0.35,8.0.36,8.0.37,8.0.38,8.0.39]。 |
   | `spec.componentSpecs.disableExporter` | 定义了是否在 component 无头服务（headless service）上标注指标 exporter 信息，是否开启监控 exporter。有效值为 [true, false]。 |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |
   | `spec.componentSpecs.volumeClaimTemplates` | PersistentVolumeClaim 模板列表，定义 component 的存储需求。 |
   | `spec.componentSpecs.volumeClaimTemplates.name` | 引用了在 `componentDefinition.spec.runtime.containers[*].volumeMounts` 中定义的 volumeMount 名称。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | 定义了 StorageClass 的名称。如果未指定，系统将默认使用带有 `storageclass.kubernetes.io/is-default-class=true` 注释的 StorageClass。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | 可按需配置存储容量。 |

   您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   执行以下命令，查看已创建的 MySQL 集群：

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. 验证集群是否创建成功。

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
   mycluster   mysql                mysql-8.0.30      Delete               Running   6m53s
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建 MySQL 集群。

   创建单机版。

   ```bash
   kbcli cluster create mysql mycluster -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create mysql --help
   kbcli cluster create mysql -h
   ```

   例如，如果您只有一个节点可用于部署主备版，可将 `topology-keys` 设置为 `null`。

   ```bash
   kbcli cluster create mysql mycluster --topology-keys=null -n demo
   ```

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        mysql                mysql-8.0.30      Delete               Running   Jul 05,2024 18:46 UTC+0800
   ```

</TabItem>

</Tabs>

## 连接集群

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MySQL 集群的连接凭证。该 Secret 包含以下 key：

* `username`：集群的根用户名。
* `password`：根用户的密码。
* `port`：集群的端口。
* `host`：集群的主机。
* `endpoint`：集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   root
   ```

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   b8wvrwlm
   ```

2. 使用用户名和密码，进入 Pod `mycluster-mysql-0` 并连接到数据库。

   ```bash
   kubectl exec -ti mycluster-mysql-0 -n demo -- bash

   mysql -u root -p b8wvrwlm
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

还可以使用端口转发在本地计算机上连接数据库。

1. 通过端口转发服务。

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   mysql -uroot -pb8wvrwlm
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster --namespace demo
```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
