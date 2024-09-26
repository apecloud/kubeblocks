---
title: 使用 KubeBlocks 管理 RabbitMQ
description: 如何使用 KubeBlocks 管理 RabbitMQ
keywords: [rabbitmq, 消息队列, broker]
sidebar_position: 1
sidebar_label: 使用 KubeBlocks 管理 RabbitMQ
---

# 使用 KubeBlocks 管理 RabbitMQ

RabbitMQ 是可靠且成熟的消息和流处理代理，可通过简单的方式在云环境、本地数据中心以及本地机器上部署。

KubeBlocks 支持管理 RabiitMQ。

:::note

当前，KubeBlocks 仅支持通过 `kubectl` 管理 RabbitMQ。

:::

## 开始之前

- 安装 KubeBlocks：可按需采用 [kbcli](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../installation/install-with-helm/install-kubeblocks.md)方式。
- 安装并启用 rabbitmq Addon：可按需采用 [kbcli](./../installation/install-with-kbcli/install-addons.md) 或 [Helm](./../installation/install-with-helm/install-addons.md) 方式。

## 创建集群

KubeBlocks implements a Cluster CRD to define a cluster. Here is an example of creating a RabbitMQ cluster with three replicas. Pods are distributed on different nodes by default. But if you only have one node for a cluster with three replicas, set `spec.affinity.topologyKeys` as `null`.

KubeBlocks 通过执行 Cluster CRD 定义集群。以下是创建三副本 RabbitMQ 集群的示例。Pod 默认分布在不同节点上。如果您只有一个节点，但仍想要创建三副本集群，建议将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 3
      serviceAccountName: kb-mycluster
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

| 字段                                   | 定义  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。 <p> - `DoNotTerminate` 会阻止删除操作。 </p><p> - `Halt` 会删除工作负载资源，如 statefulset 和 deployment 等，但是保留了 PVC 。  </p><p> - `Delete` 在 `Halt` 的基础上进一步删除了 PVC。 </p><p> - `WipeOut` 在 `Delete` 的基础上从备份存储的位置完全删除所有卷快照和快照数据。 </p>|
| `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
| `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
| `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
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

执行以下命令，查看已创建的 RabbitMQ 集群：

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## 连接集群

使用 [RabbitMQ 工具](https://www.rabbitmq.com/docs/cli) 连接并管理 RabbitMQ 集群。

## 集群扩缩容

### 垂直扩缩容

检查集群状态是否为 `Running`。否则，后续操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### Option 1. Apply an OpsRequest

1. Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: rabbitmq
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

#### Option 2. Edit the cluster YAML file

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 3
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Delete
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### Option 1. Apply an OpsRequest

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means deleting two replicas.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: rabbitmq
       scaleIn:
         replicaChanges: 2
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn` and change the value in `replicaChanges`.

2. Check the operation status to validate the horizontal scaling status.

   ```bash
   kubectl get ops -n demo
   >
   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   2/2        6m
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

#### Option 2. Edit the cluster YAML file

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 1 # Change the amount
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

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

## Volume expansion

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

### Option 1. Apply an OpsRequest

1. Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: rabbitmq
        volumeClaimTemplates:
        - name: data
          storage: "40Gi"
    EOF
    ```

2. Validate the volume expansion operation.

    ```bash
    kubectl get ops -n demo
    >
    NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    ops-volume-expansion   VolumeExpansion   mycluster   Succeed   1/1        6m
    ```

    If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

### Option 2. Edit the cluster YAML file

1. Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # Change the volume storage size.
     terminationPolicy: Delete
   ```

2. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

## Restart

1. Restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: rabbitmq
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo

   kubectl get ops -n demo
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

### Stop a cluster

#### Option 1. Apply an OpsRequest

Run the command below to stop a cluster.

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: mycluster-stop
  namespace: demo
spec:
  clusterName: mycluster
  type: Stop
EOF
```

#### Option 2. Edit the cluster YAML file

Configure `replicas` as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 0
      serviceAccountName: kb-mycluster
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
```

### Start a cluster

#### Option 1. Apply an OpsRequest

Run the command below to start a cluster.

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: mycluster-start
  namespace: demo
spec:
  clusterName: mycluster
  type: Start
EOF 
```

#### Option 2. Edit the cluster YAML file

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 3
      serviceAccountName: kb-mycluster
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
```

## Monitor

The monitoring function of RabbitMQ is the same as other engines. For details, refer to related docs:

- [Monitor databases by kbcli](./../observability/monitor-database.md)
- [Monitor databases by kubectl](./../../api_docs/observability/monitor-database.md)
