---
title: 用 KubeBlocks 管理 RabbitMQ
description: 如何使用 KubeBlocks 管理 RabbitMQ
keywords: [rabbitmq, 消息队列, streaming, broker]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 RabbitMQ
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 用 KubeBlocks 管理 RabbitMQ

RabbitMQ 是一靠且成熟的消息和流处理中间件，支持在云环境、本地数据中心或个人计算机上部署。

本文档展示了如何通过 kbcli、kubectl 或 YAML 文件等当时创建和管理 RabbitMQ 集群。您可以在 [GitHub 仓库](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/rabbitmq)查看 YAML 示例。

:::note

当前，KubeBlocks 仅支持通过 `kubectl` 管理 RabbitMQ 集群.

:::

## 开始之前

- 安装 KubeBlocks，可通过 [kbcli](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../installation/install-with-helm/install-kubeblocks.md) 安装.
- 安装并启用 rabbitmq 引擎，可通过 [kbcli](./../installation/install-with-kbcli/install-addons.md) 或 [Helm](./../installation/install-with-helm/install-addons.md)操作.

## 创建集群

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 RabbitMQ 集群的示例。Pod 默认分布在不同节点。但如果您只有一个节点可用于部署集群，可将 `spec.affinity.topologyKeys` 设置为 `null`。

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
| `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。 具体定义可参考 [终止策略](#终止策略)。|
| `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
| `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
| `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
| `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
| `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
| `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition rabbitmq -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
| `spec.componentSpecs.name`            | 定义了 component 的名称。  |
| `spec.componentSpecs.disableExporter` | 定义了是否开启监控功能。 |
| `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
| `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

执行以下命令，查看已创建的 RabbitMQ 集群。

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## 连接集群

可使用 [RabbitMQ tools](https://www.rabbitmq.com/docs/cli) 连接并管理 RabbitMQ 集群。

## 扩缩容

### 垂直扩缩容

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

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

2. 查看运维任务状态，验证垂直扩缩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

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
       resources: # 修改 resources 中的参数值
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

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>

### 水平伸缩

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容文档](./../maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   以下示例演示了增加 2 个副本。

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

   如果您想要缩容，可将 `scaleOut` 替换为 `scaleIn`，并修改 `replicaChanges` 的参数值。

2. 查看运维操作状态，验证水平扩缩容是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   2/2        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>
  
<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.replicas` 的配置。`spec.componentSpecs.replicas` 定义了 pod 数量，修改该参数将触发集群水平扩缩容。

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
       replicas: 1 # 修改参数值
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

2. 当集群状态再次回到 `Running` 后，查看相关资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

## 磁盘扩容

### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

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

2. 查看磁盘扩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   ops-volume-expansion   VolumeExpansion   mycluster   Succeed   1/1        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

1. 修改集群 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的值。

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` 定义了 Pod 存储资源信息，修改该数值将触发集群磁盘扩容。

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
               storage: 40Gi # 修改该参数值
     terminationPolicy: Delete
   ```

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>

## 停止/启动集群

您可以停止/启动集群以释放计算资源。当集群停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。您也可以重新启动该集群，使其恢复到停止集群前的状态。

### 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

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

   </TabItem>

   <TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

   将 replicas 的值修改为 0，删除 pod。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
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
           - name: data
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
         services:
   ```

   </TabItem>

   </Tabs>

2. 查看集群状态，确认集群是否已停止。

   ```bash
   kubectl get cluster mycluster -n demo
   ```

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

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

   </TabItem>

   <TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

   将 replicas 数值修改为初始值，启动集群。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
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
           - name: data
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
         services:
   ```

   </TabItem>

   </Tabs>

2. 查看集群状态，确认集群是否已再次运行。

   ```bash
   kubectl get cluster mycluster -n demo
   ```

## 重启集群

1. 执行以下命令，重启集群。

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

2. 查看 pod 和运维操作状态，验证重启操作是否成功。

   ```bash
   kubectl get pod -n demo

   kubectl get ops -n demo
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

## 删除集群

### 终止策略

:::note

终止策略决定了删除集群的方式。

:::

| **终止策略** | **删除操作**                                                                     |
|:----------------------|:-------------------------------------------------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。                                                  |
| `Halt`                | `Halt` 删除集群资源（如 Pods、Services 等），但保留 PVC。停止其他运维操作的同时，保留了数据。但 `Halt` 策略在 v0.9.1 中已删除，设置为 `Halt` 的效果与 `DoNotTerminate` 相同。  |
| `Delete`              | `Delete` 在 `Halt` 的基础上，删除 PVC 及所有持久数据。                              |
| `WipeOut`             | `WipeOut`  删除所有集群资源，包括外部存储中的卷快照和备份。使用该策略将会删除全部数据，特别是在非生产环境，该策略将会带来不可逆的数据丢失。请谨慎使用。   |

执行以下命令查看终止策略。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

### 步骤

执行以下命令，删除集群。

```bash
kubectl delete cluster mycluster -n demo
```

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
