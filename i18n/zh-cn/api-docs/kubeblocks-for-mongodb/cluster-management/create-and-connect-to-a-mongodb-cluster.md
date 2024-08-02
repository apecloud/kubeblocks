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

* [Install KubeBlocks](./../../installation/install-kubeblocks.md)。

* 查看可用于创建集群的数据库类型和版本。
  
  确保 `mongodb` cluster definition 已安装。如果该 cluster definition 不可用，可[参考相关文档](./../../installation/install-addons.md)启用。

  ```bash
  kubectl get clusterdefinition mongodb
  >
  NAME      TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mongodb                              Available   30m
  ```

  查看可用于创建集群的引擎版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  ```

* 为保证资源隔离，本教程将创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

### 创建集群

KubeBlocks 支持创建两种 MongoDB 集群：单机版（Standalone）和主备版（ReplicaSet）。MongoDB 单机版仅支持一个副本，适用于对可用性要求较低的场景。 对于高可用性要求较高的场景，建议创建主备版集群，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。但如果您只有一个节点可用于创建主备版集群，可将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 MongoDB 单机版的示例。

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-6.0
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
  - name: mongodb
    componentDefRef: mongodb
    enabledLogs:
    - running
    disableExporter: true
    serviceAccountName: kb-mongo-cluster
    replicas: 1
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
| `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition mongodb -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
| `spec.componentSpecs.name`            | 定义了 component 的名称。  |
| `spec.componentSpecs.disableExporter` | 定义了是否开启监控功能。 |
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

<details>

<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mycluster","app.kubernetes.io/version":"5.0.14","helm.sh/chart":"mycluster-0.8.0"},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"mongodb","clusterVersionRef":"mongodb-5.0","componentSpecs":[{"componentDefRef":"mongodb","disableExporter":true,"name":"mongodb","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":null,"services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2024-05-07T10:23:13Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: mycluster
    app.kubernetes.io/version: 5.0.14
    clusterdefinition.kubeblocks.io/name: mongodb
    clusterversion.kubeblocks.io/name: mongodb-5.0
    helm.sh/chart: mongodb-cluster-0.8.0
  name: mycluster
  namespace: demo
  resourceVersion: "560727"
  uid: 3fced3b6-34bf-4d3a-88e2-baf4e2d73b44
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-5.0
  componentSpecs:
  - componentDefRef: mongodb
    disableExporter: true
    name: mongodb
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
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
    mongodb:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-05-07T10:23:55Z"
  conditions:
  - lastTransitionTime: "2024-05-07T10:23:13Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-05-07T10:23:13Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-05-07T10:23:55Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-05-07T10:23:55Z"
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

<TabItem value="kubectl" label="kubectl" default>

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `mycluster-conn-credential` 的新的 Secret 来存储 MongoDB 集群的连接凭证。该 Secret 包含以下 key：

* `username`：MongoDB 集群的根用户名。
* `password`：根用户的密码。
* `port`：MongoDB 集群的端口。
* `host`：MongoDB 集群的主机。
* `endpoint`：MongoDB 集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
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

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../connect_database/overview-of-database-connection.md)。
