---
title: 用 KubeBlocks 管理 Xinference
description: 如何用 KubeBlocks 管理 Xinference
keywords: [xinference, LLM, AI]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Xinference
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 用 KubeBlocks 管理 Xinference

Xorbits Inference (Xinference) 是一个开源平台，用于简化各种 AI 模型的运行和集成。借助 Xinference，您可以使用任何开源 LLM、嵌入模型和多模态模型在云端或本地环境中运行推理，并创建强大的 AI 应用。

本教程演示了如何通过 `kubectl` 或 YAML 文件创建并管理 Xinference 集群。您可在 [GitHub 仓库](https://github.com/apecloud/kubeblocks-addons/tree/release-0.9/examples/xinference)查看相应的 YAML 示例和指南。

## 开始之前

* [安装 KubeBlocks](./../installation/install-kubeblocks.md)。
* 查看可用于创建集群的数据库类型和版本。
  
  确保 `xinference` cluster definition 已安装。如果该 cluster definition 不可用，可[参考相关文档](./../installation/install-addons.md)启用。

  ```bash
  kubectl get clusterdefinition xinference
  >
  NAME           TOPOLOGIES   SERVICEREFS   STATUS      AGE
  xinference                                Available   30m
  ```

  查看可用于创建集群的引擎版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=xinference
  ```

* 为保证资源隔离，本教程将创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

## 创建集群

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 Xinference 集群的示例。Pod 默认分布在不同节点。但如果您只有一个节点可用于部署集群，可将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: xinference
  clusterVersionRef: xinference-0.11.0-cpu
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
  - name: xinference
    componentDefRef: xinference
    serviceAccountName: kb-xinference-cluster
    replicas: 1
    resources:
      limits:
        cpu: '1'
        memory: 1Gi
      requests:
        cpu: '1'
        memory: 1Gi
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
| `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition xinference -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
| `spec.componentSpecs.name`            | 定义了 component 的名称。  |
| `spec.componentSpecs.disableExporter` | 定义了是否开启监控功能。 |
| `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
| `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

执行以下命令，查看已创建的 Xinference 集群：

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## 垂直扩缩容

您可以通过更改资源需求和限制（例如 CPU 和存储）来实现集群垂直扩缩容。例如，您可以通过垂直扩缩容将资源类别从 1C2G 更改为 2C4G。

#### 开始之前

检查集群状态是否为 `Running`。否则，后续操作可能会失败。

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION     VERSION                 TERMINATION-POLICY     STATUS    AGE
mycluster   xinference             xinference-0.11.0-cpu   Delete                 Running   47m
```

### 步骤

可通过以下两种方式实现垂直扩缩容。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定的集群应用 OpsRequest，可根据您的需求配置参数。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: xinference
       requests:
         memory: "2Gi"
         cpu: "2"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. 查看运维任务状态，验证垂直扩缩容操作是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                         TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        mycluster-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: xinference
     clusterVersionRef: xinference-0.11.0-cpu
     componentSpecs:
     - name: xinference
       componentDefRef: xinference
       replicas: 1
       resources: # 修改该参数值
         requests:
           memory: "2Gi"
           cpu: "2"
         limits:
           memory: "4Gi"
           cpu: "2"
   ```

2. 查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>
</Tabs>

## 停止/启动集群

您可以停止/启动集群以释放计算资源。当集群停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你想恢复集群资源，可通过快照重新启动集群。

### 停止集群

您可通过创建 OpsRequest 或修改集群 YAML 文件来停止集群。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

执行以下命令，停止集群。

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

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

将副本数设置为 0，删除 Pod。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
    namespace: demo
spec:
  clusterDefinitionRef: xinference
  clusterVersionRef: xinference-0.11.0-cpu
  terminationPolicy: Delete
  componentSpecs:
  - name: xinference
    componentDefRef: xinference
    disableExporter: true  
    replicas: 0
```

</TabItem>

</Tabs>

### 启动集群

您可通过创建 OpsRequest 或修改集群 YAML 文件来启动集群。
  
<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

执行以下命令，启动集群。

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

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

将副本数改为原始数量，重新启动该集群。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
    namespace: demo
spec:
  clusterDefinitionRef: xinference
  clusterVersionRef: xinference-0.11.0-cpu
  terminationPolicy: Delete
  componentSpecs:
  - name: xinference
    componentDefRef: xinference
    disableExporter: true  
    replicas: 1
```

</TabItem>

</Tabs>

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
     - componentName: xinference
   EOF
   ```

2. 查看 pod 和运维任务状态，确认重启操作是否成功。

   ```bash
   kubectl get pod -n demo

   kubectl get ops mycluster-restart -n demo
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。
