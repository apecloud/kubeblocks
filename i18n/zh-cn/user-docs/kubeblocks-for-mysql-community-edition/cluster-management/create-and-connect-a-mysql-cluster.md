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

* 如果您想通过 `kbcli` 创建并连接 ApeCloud MySQL 集群，请先[安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 安装 KubeBlocks，可通过 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks.md) 安装。
* 确保 MySQL 引擎已启用。KubeBlocks 默认已安装 MySQL，如果您在安装 KubeBlcoks 关闭/卸载了该引擎，可参考相关文档，再次启用/安装该引擎，可通过 [kbcli](./../../installation/install-with-kbcli/install-addons.md) 或者 [Helm](./../../installation/install-with-helm/install-addons.md) 操作。
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  mysql                          0.9.1           community   Enabled    true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io mysql
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  mysql   Helm                        Enabled   27h
  ```

  </TabItem>

  </Tabs>

* 查看可用于创建集群的数据库类型和版本。

  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli clusterdefinition list
  kbcli clusterversion list
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

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

  </Tabs>

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种类型的 MySQL 集群：单机版（Standalone）和主备版（Replication）。单机版仅支持一个副本，适用于对可用性要求较低的场景。主备版包含两个副本，适用于对高可用性要求较高的场景。为了确保高可用性，所有的副本都默认分布在不同的节点上。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 创建 MySQL 集群。

   创建单机版。

   ```bash
   kbcli cluster create mycluster --cluster-definition mysql -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create mysql --help
   kbcli cluster create mysql -h
   ```

   例如，您可以使用 `--replicas` 指定副本数，创建主备版集群。

   ```bash
   kbcli cluster create mycluster --cluster-definition mysql --replicas=2 -n demo
   ```

   如果您只有一个节点可用于部署主备版，可将 `topology-keys` 设置为 `null`。

   ```bash
   kbcli cluster create mycluster --cluster-definition mysql --set replicas=2 --topology-keys null -n demo
   ```

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        mysql                mysql-8.0.30      Delete               Running   Jul 05,2024 18:46 UTC+0800
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 创建 MySQL 集群。
   
   KubeBlocks 通过 `Cluster` 定义集群。以下是创建 MySQL 主备版的示例。

   如果您只有一个节点可用于部署集群版，可将 `spec.affinity.topologyKeys` 设置为 `null`。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: mysql
     clusterVersionRef: mysql-8.0.33
     terminationPolicy: Delete
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     tolerations:
       - key: kb-data
         operator: Equal
         value: 'true'
         effect: NoSchedule
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       enabledLogs:
       - error
       - slow
       disableExporter: true
       replicas: 2
       serviceAccountName: kb-mysql-cluster
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
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
   EOF
   ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.clusterDefinitionRef`           | 集群定义 CRD 的名称，用来定义集群组件。  |
   | `spec.clusterVersionRef`              | 集群版本 CRD 的名称，用来定义集群版本。 |
   | `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-mysql-cluster.md#终止策略)。 |
   | `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
   | `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
   | `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
   | `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
   | `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
   | `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition mysql -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
   | `spec.componentSpecs.name`            | 定义了 component 的名称。  |
   | `spec.componentSpecs.disableExporter` | 定义了是否开启监控功能。 |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

   KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

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

</Tabs>

## 连接集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect mycluster --namespace demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MySQL 集群的连接凭证。该 Secret 包含以下 key：

* `username`：集群的根用户名。
* `password`：根用户的密码。
* `port`：集群的端口。
* `host`：集群的主机。
* `endpoint`：集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root
   ```

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
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

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect-databases/overview-on-connect-databases.md)。
