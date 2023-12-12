---
title: 创建并连接到 MySQL 集群
description: 如何创建并连接到 MySQL 集群
keywords: [mysql, 创建 MySQL 集群, 连接 MySQL 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建并连接 MySQL 集群

本文档展示如何创建并连接到一个 MySQL 集群。

## 创建并连接到 MySQL 集群

### 开始之前

* 如果想通过 kbcli 创建和连接 MySQL 集群，请[安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 用 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md) 安装 KubeBlocks。
* 确保 ApeCloud MySQL 引擎已启用。
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>
  
  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  apecloud-mysql                 Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io apecloud-mysql
  >
  NAME             TYPE   STATUS    AGE
  apecloud-mysql   Helm   Enabled   61s
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
  
  查看 `apecloud-mysql` 集群定义是否可用。


  ```bash
  kubectl get clusterdefinition apecloud-mysql
  >
  NAME             MAIN-COMPONENT-NAME   STATUS      AGE
  apecloud-mysql   mysql                 Available   85m
  ```

  查看可用于创建集群的所有版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=apecloud-mysql
  ```

  </TabItem>

  </Tabs>

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种类型的 MySQL 集群：单机版（Standalone）和集群版（RaftGroup）。单机版仅支持一个副本，适用于对可用性要求较低的场景。 集群版包含三个副本，适用于对高可用性要求较高的场景。为了确保高可用性，所有的副本都默认分布在不同的节点上。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

创建 MySQL 单机版。

```bash
kbcli cluster create mysql <clustername>
```

创建 MySQL 集群版。

```bash
kbcli cluster create mysql --mode raftGroup <clustername>
```

如果只有一个节点用于部署三节点集群，请在创建集群时将 `availability-policy` 设置为 `none`。

```bash
kbcli cluster create mysql --mode raftGroup --availability-policy none <clustername>
```

:::note

* 生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。
* 执行以下命令，查看创建 MySQL 集群的选项和默认值。
  
  ```bash
  kbcli cluster create mysql -h
  ```

:::

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks 实现了用 Cluster CRD 来定义集群。比如，可以通过下面的命令创建一个 MySQL 集群版：

   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: demo
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - componentDefRef: mysql
       name: mysql
       replicas: 3
       resources:
         limits:
           cpu: "1"
           memory: 1Gi
         requests:
           cpu: "1"
           memory: 1Gi
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

* `spec.clusterDefinitionRef` 是集群定义 CRD 的名称，用来定义集群组件。
* `spec.clusterVersionRef` 是集群版本 CRD 的名称，用来定义集群版本。
* `spec.componentSpecs` 是组件列表，用来定义集群组件。
* `spec.componentSpecs.componentDefRef` 是组件定义的名称，在 ClusterDefinition 中定义。你可以使用 `kubectl get clusterdefinition vape cloud-MySQL -o json | jq '.spec.componentDefs[].name'` 获取组件定义的名称。
* `spec.componentSpecs.name` 是组件的名称。
* `spec.componentSpecs.replicas` 是组件的副本数。
* `spec.componentSpecs.resources` 是组件的资源要求。
* `spec.componentSpecs.volumeClaimTemplates` 是卷声明模板的列表，用于定义组件的卷声明模板。
* `spec.terminationPolicy` 是集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。`DoNotTerminate` 会阻止删除操作。`Halt` 会删除工作负载资源，如 statefulset 和 deployment 等，但是保留了 PVC 。`Delete` 在 `Halt` 的基础上进一步删除了 PVC。`WipeOut` 在 `Delete` 的基础上从备份存储的位置完全删除所有卷快照和快照数据。

KubeBlocks operator 监听 `Cluster` CRD，创建集群及其依赖资源。你可以使用以下命令获取该集群创建的所有资源。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mysql-cluster -n demo
```

查看所创建的 MySQL 集群对象：

```bash
kubectl get cluster mysql-cluster -n demo -o yaml
```

<details>
<summary>输出</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"name":"mysql-cluster","namespace":"demo"},"spec":{"clusterDefinitionRef":"apecloud-mysql","clusterVersionRef":"ac-mysql-8.0.30","componentSpecs":[{"componentDefRef":"mysql","name":"mysql","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"1Gi"},"requests":{"cpu":"0.5","memory":"1Gi"}},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-17T09:03:23Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    clusterdefinition.kubeblocks.io/name: apecloud-mysql
    clusterversion.kubeblocks.io/name: ac-mysql-8.0.30
  name: mysql-cluster
  namespace: demo
  resourceVersion: "27158"
  uid: de7c9fa4-7b94-4227-8852-8d76263aa326
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  componentSpecs:
  - componentDefRef: mysql
    monitor: false
    name: mysql
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 1Gi
      requests:
        cpu: "0.5"
        memory: 1Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
status:
  clusterDefGeneration: 2
  components:
    mysql:
      consensusSetStatus:
        leader:
          accessMode: None
          name: ""
          pod: Unknown
      phase: Failed
      podsReady: true
      podsReadyTime: "2023-07-17T09:03:37Z"
  conditions:
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: 'The operator has started the provisioning of Cluster: mysql-cluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-17T09:03:37Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: 'pods are unavailable in Components: [mysql], refer to related component
      message in Cluster.status.components'
    reason: ComponentsNotReady
    status: "False"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

</Tabs>

## 连接集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mysql-cluster-conn-credential` 的新的 Secret 来存储 MySQL 集群的连接凭证。该 Secret 包含以下 key：

* `username`：MySQL 集群的根用户名。
* `password`：根用户的密码。
* `port`：MySQL 集群的端口。
* `host`：MySQL 集群的主机。
* `endpoint`：MySQL 集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`.

   ```bash
   kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. 使用用户名和密码，进入 Pod `mysql-cluster-mysql-0` 并连接到数据库。

   ```bash
   kubectl exec -ti -n demo mysql-cluster-mysql-0 -- bash

   mysql -uroot -p2gvztbvz
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

还可以使用端口转发在本地计算机上连接数据库。

1. 端口转发。

   ```bash
   kubectl port-forward svc/mysql-cluster-mysql 3306:3306 -n demo
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   mysql -uroot -p2gvztbvz
   ```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../create-and-connect-databases/overview-on-connect-databases.md)。
