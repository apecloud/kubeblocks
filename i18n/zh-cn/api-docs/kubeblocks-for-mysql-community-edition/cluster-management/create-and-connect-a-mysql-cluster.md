---
title: 创建并连接到 MySQL 集群
description: 如何创建并连接到 MySQL 集群
keywords: [mysql, 创建 MySQL 集群, 连接 MySQL 集群]
sidebar_position: 1
sidebar_label: 创建并连接
---

# 创建并连接到 MySQL 集群

本文档展示如何创建并连接到一个 MySQL 集群。

## 创建 MySQL 集群

### 开始之前

* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* 查看可用于创建集群的数据库类型和版本。

  确保 `mysql` cluster definition 已安装。如果该 cluster definition 不可用，可[参考相关文档](./../../installation/install-addons.md)启用。

  ```bash
  kubectl get clusterdefinition mysql
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mysql                              Available   27m
  ```

  查看可用的集群版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mysql
  ```

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种类型的 MySQL 集群：单机版（Standalone）和主备版（Replication）。单机版仅支持一个副本，适用于对可用性要求较低的场景。主备版包含两个个副本，适用于对高可用性要求较高的场景。

为了确保高可用性，所有的副本都默认分布在不同的节点上。如果您只有一个节点可用于部署集群版，可将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 MySQL 主备版的示例。

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
| `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。 <p> - `DoNotTerminate` 会阻止删除操作。 </p><p> - `Halt` 会删除工作负载资源，如 statefulset 和 deployment 等，但是保留了 PVC 。  </p><p> - `Delete` 在 `Halt` 的基础上进一步删除了 PVC。 </p><p> - `WipeOut` 在 `Delete` 的基础上从备份存储的位置完全删除所有卷快照和快照数据。 </p>|
| `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
| `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
| `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
| `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
| `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
| `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition apecloud-mysql -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
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

<details>
<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  creationTimestamp: "2024-03-28T10:08:23Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    clusterdefinition.kubeblocks.io/name: mysql
    clusterversion.kubeblocks.io/name: mysql-8.0.33
  name: mycluster
  namespace: default
  resourceVersion: "5466612"
  uid: 2f630f27-3de3-4fd0-85eb-5fd7e4150c8c
spec:
  affinity:
    podAntiAffinity: Preferred
  clusterDefinitionRef: mysql
  clusterVersionRef: mysql-8.0.33
  componentSpecs:
  - componentDefRef: mysql
    enabledLogs:
    - error
    - slow
    disableExporter: true
    name: mysql
    replicas: 2
    resources:
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    serviceAccountName: kb-mycluster
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  resources:
    cpu: "0"
    memory: "0"
  storage:
    size: "0"
  terminationPolicy: Delete
status:
  clusterDefGeneration: 2
  components:
    mysql:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-03-29T18:12:43Z"
  conditions:
  - lastTransitionTime: "2024-03-28T10:08:23Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-03-28T10:08:23Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-03-29T18:12:43Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-03-29T18:12:43Z"
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

## 连接集群

<Tabs>

<TabItem value="kubectl" label="kubectl">

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MySQL 集群的连接凭证。该 Secret 包含以下 key：

* `username`：MySQL 集群的根用户名。
* `password`：根用户的密码。
* `port`：MySQL 集群的端口。
* `host`：MySQL 集群的主机。
* `endpoint`：MySQL 集群的终端节点，与 `host:port` 相同。

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
   kubectl exec -ti mycluster-mysql-0 -- bash

   mysql -u root -p b8wvrwlm
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

还可以使用端口转发在本地计算机上连接数据库。

1. 通过端口转发服务。

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n default
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   mysql -uroot -pb8wvrwlm
   ```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect_database/overview-of-database-connection.md)。
