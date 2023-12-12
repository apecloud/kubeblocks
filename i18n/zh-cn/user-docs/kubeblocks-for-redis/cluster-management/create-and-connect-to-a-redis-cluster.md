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

KubeBlocks 支持 Redis 单机版和 Redis 主备版集群。为了提供更好的高可用性体验，KubeBlocks 默认创建的是 Redis 主备版集群。

## 创建 Redis 集群

### 开始之前

* 如果想通过 kbcli 创建和连接 MySQL 集群，请[安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 用 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md) 安装 KubeBlocks。
* 确保 Redis 引擎已启用。
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>
  
  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  redis                     Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io redis
  >            
  NAME    TYPE   STATUS    AGE
  redis   Helm   Enabled   96m
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

  查看 `redis` 集群定义是否可用。

  ```bash
  kubectl get clusterdefinition Redis
  >
  NAME    MAIN-COMPONENT-NAME   STATUS      AGE
  redis   redis                 Available   96m
  ```

  查看可用于创建集群的所有版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   96m
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

KubeBlocks 支持创建两种 Redis 集群：单机版（Standalone）和主备版（Replication）。Redis 单机版仅支持一个副本，适用于对可用性要求较低的场景。 对于高可用性要求较高的场景，建议创建主备版集群，以支持自动故障切换。为了确保高可用性，所有的副本都默认分布在不同的节点上。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

创建 Redis 单机版。

```bash
kbcli cluster create redis <clustername>
```

创建 Redis 主备版。

```bash
kbcli cluster create redis --mode replication <clustername>
```

如果只有一个节点用于部署主备版集群，请在创建集群时将 `availability-policy` 设置为 `none`。

```bash
kbcli cluster create redis --mode replication --availability-policy none <clustername>
```

:::note

* 在生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。
* 执行以下命令，查看创建 Redis 集群的选项和默认值。
  
  ```bash
  kbcli cluster create redis -h
  ```

:::

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks 实现了用 `Cluster` CRD 来定义集群。比如，可以通过下面的命令创建 Redis 单机版集群：

  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: redis
    namespace: demo
    labels: 
      helm.sh/chart: redis-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "7.0.6"
      app.kubernetes.io/instance: redis
  spec:
    clusterVersionRef: redis-7.0.6
    terminationPolicy: Delete  
    affinity:
      podAntiAffinity: Preferred
      topologyKeys:
        - kubernetes.io/hostname
      tenancy: SharedNode
    clusterDefinitionRef: redis  # ref clusterDefinition.name
    componentSpecs:
      - name: redis
        componentDefRef: redis # ref clusterDefinition componentDefs.name      
        monitor: false      
        replicas: 1
        enabledLogs:
          - running
        serviceAccountName: kb-redis
        switchPolicy:
          type: Noop      
        resources:
          limits:
            cpu: "0.5"
            memory: "0.5Gi"
          requests:
            cpu: "0.5"
            memory: "0.5Gi"      
        volumeClaimTemplates:
          - name: data # ref clusterDefinition components.containers.volumeMounts.name
            spec:
              accessModes:
                - ReadWriteOnce
              resources:
                requests:
                  storage: 20Gi      
        services:
  EOF
  ```

* `spec.clusterDefinitionRef` 是集群定义 CRD 的名称，用来定义集群组件。
* `spec.clusterVersionRef` 是集群版本 CRD 的名称，用来定义集群版本。
* `spec.componentSpecs` 是组件列表，用来定义集群组件。
* `spec.componentSpecs.componentDefRef` 是组件定义的名称，在 ClusterDefinition 中定义。你可以使用 `kubectl get clusterdefinition redis -o json | jq '.spec.componentDefs[].name'` 获取组件定义的名称。
* `spec.componentSpecs.name` 是组件的名称。
* `spec.componentSpecs.replicas` 是组件的副本数。
* `spec.componentSpecs.resources` 是组件的资源要求。
* `spec.componentSpecs.volumeClaimTemplates` 是卷声明模板的列表，用于定义组件的卷声明模板。
* `spec.terminationPolicy` 是集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。`DoNotTerminate` 禁止一切删除操作。`Halt` 会删除工作负载资源，如 statefulset 和 deployment 等，但是保留 PVC 。`Delete` 在 `Halt` 的基础上进一步删除了 PVC。`WipeOut` 在 `Delete` 的基础上从备份存储的位置完全删除所有卷快照和快照数据。

KubeBlocks operator 监听 `Cluster` CRD，并创建集群及其依赖资源。你可以使用以下命令获取该集群创建的所有资源。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=redis -n demo
```

查看所创建的 Redis 集群对象：

```bash
kubectl get cluster redis -n demo -o yaml
```

<details>

<summary>输出</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"redis","app.kubernetes.io/version":"7.0.6","helm.sh/chart":"redis-cluster-0.6.0-alpha.36"},"name":"redis","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"redis","clusterVersionRef":"redis-7.0.6","componentSpecs":[{"componentDefRef":"redis","enabledLogs":["running"],"monitor":false,"name":"redis","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-redis","services":null,"switchPolicy":{"type":"Noop"},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T08:33:48Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: redis
    app.kubernetes.io/version: 7.0.6
    clusterdefinition.kubeblocks.io/name: redis
    clusterversion.kubeblocks.io/name: redis-7.0.6
    helm.sh/chart: redis-cluster-0.6.0-alpha.36
  name: redis
  namespace: demo
  resourceVersion: "12967"
  uid: 25ae9193-60ae-4521-88eb-70ea4c3d97ef
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  componentSpecs:
  - componentDefRef: redis
    enabledLogs:
    - running
    monitor: false
    name: redis
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-redis
    switchPolicy:
      type: Noop
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
    redis:
      phase: Running
      podsReady: true
      podsReadyTime: "2023-07-19T08:34:34Z"
      replicationSetStatus:
        primary:
          pod: redis-redis-0
  conditions:
  - lastTransitionTime: "2023-07-19T08:33:48Z"
    message: 'The operator has started the provisioning of Cluster: redis'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-19T08:33:48Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-19T08:34:34Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-19T08:34:34Z"
    message: 'Cluster: redis is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

</Tabs>

## 连接到 Redis 集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

使用 `kubectl exec` 命令进入 Pod 并连接到数据库。

KubeBlocks operator 会创建一个名为 `redis-conn-credential` 的新的 Secret 来存储 Redis 集群的连接凭证。该 Secret 包含以下 key：

* `username`：Redis 集群的根用户名。
* `password`：根用户的密码。
* `port`：Redis 集群的端口。
* `host`：Redis 集群的主机。
* `endpoint`：Redis 集群的终端节点，与 `host:port` 相同。

1. 获取用于 `kubectl exec` 命令的 `username` 和 `password`。
   
   ```bash
   kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   default

   kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   p7twmbrd
   ```

2. 使用用户名和密码，进入 Pod `redis-redis-0` 并连接到数据库。
   
   ```bash
   kubectl exec -ti -n demo redis-redis-0 -- bash

   root@redis-redis-0:/# redis-cli -a p7twmbrd  --user default
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

你还可以使用端口转发在本地计算机上连接数据库。

1. 端口转发。
  
   ```bash
   kubectl port-forward -n demo svc/redis-redis 6379:6379
   ```

2. 在新的终端窗口中执行以下命令，连接到数据库。

   ```bash
   root@redis-redis-0:/# redis-cli -a p7twmbrd  --user default
   ```

</TabItem>

</Tabs>

有关详细的数据库连接指南，请参考[连接数据库](./../../create-and-connect-databases/overview-on-connect-databases.md)。