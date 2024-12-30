---
title: 用 KubeBlocks 管理 Qdrant
description: 如何用 KubeBlocks 管理 Qdrant
keywords: [qdrant, 向量数据库]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Qdrant
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 用 KubeBlocks 管理 Qdrant

生成式人工智能的爆火引发了人们对向量数据库的关注。

Qdrant（读作：quadrant）是向量相似性搜索引擎和向量数据库。它提供了生产可用的服务和便捷的 API，用于存储、搜索和管理点（即带有额外负载的向量）。Qdrant 专门针对扩展过滤功能进行了优化，使其在各种神经网络或基于语义的匹配、分面搜索以及其他应用中充分发挥作用。

目前，KubeBlocks 支持 Qdrant 的管理和运维。本文档展示了如何通过 kbcli、kubectl 或 YAML 文件等当时创建和管理 Qdrant 集群。您可以在 [GitHub 仓库](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/qdrant)查看 YAML 示例。

## 开始之前

- 如果您想通过 `kbcli` 创建和管理集群，请先[安装 kbcli](./../installation/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-kubeblocks.md)。
- [安装并启用 qdrant 引擎](./../installation/install-addons.md)。
- 为了保持隔离，本文档中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

## 创建集群

***步骤：***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 Qdrant 集群的示例。Pod 默认分布在不同节点。但如果您只有一个节点可用于部署集群，可将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  annotations: {}
spec:
  terminationPolicy: Delete
  clusterDef: qdrant
  topology: cluster
  componentSpecs:
    - name: qdrant
      annotations: {}
      serviceVersion: 1.10.0
      replicas: 3
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
| `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](#终止策略)。 |
| `spec.clusterDef` | 指定了创建集群时要使用的 ClusterDefinition 的名称。**注意**：**请勿更新此字段**。创建 Qdrant 集群时，该值必须为 `qdrant`。 |
| `spec.topology` | 指定了在创建集群时要使用的 ClusterTopology 的名称。 |
| `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
| `spec.componentSpecs.serviceVersion`  | 定义了 component 部署的服务版本。可选值为 [1.10.0,1.5.0,1.7.3,1.8.1,1.8.4]。 |
| `spec.componentSpecs.disableExporter` | 定义了是否在 component 无头服务（headless service）上标注指标 exporter 信息，是否开启监控 exporter。有效值为 [true, false]。 |
| `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。推荐值为 [3,5,7]。|
| `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |
| `spec.componentSpecs.volumeClaimTemplates` | PersistentVolumeClaim 模板列表，定义 component 的存储需求。 |
| `spec.componentSpecs.volumeClaimTemplates.name` | 引用了在 `componentDefinition.spec.runtime.containers[*].volumeMounts` 中定义的 volumeMount 名称。  |
| `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | 定义了 StorageClass 的名称。如果未指定，系统将默认使用带有 `storageclass.kubernetes.io/is-default-class=true` 注释的 StorageClass。  |
| `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | 可按需配置存储容量。 |

您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

执行以下命令，查看已创建的 Qdrant 集群：

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建一个 Qdrant 集群。

   ```bash
   kbcli cluster create qdrant mycluster -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create qdrant --help
   kbcli cluster create qdrant -h
   ```

2. 检查集群是否已创建。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        qdrant                              Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

3. 查看集群信息。

   ```bash
   kbcli cluster describe mycluster -n demo
   >
   Name: mycluster         Created Time: Aug 15,2023 23:03 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION        STATUS    TERMINATION-POLICY
   demo        qdrant                              Running   Delete

   Endpoints:
   COMPONENT   MODE        INTERNAL                                                 EXTERNAL
   qdrant      ReadWrite   mycluster-qdrant-qdrant.default.svc.cluster.local:6333   <none>
                           mycluster-qdrant-qdrant.default.svc.cluster.local:6334

   Topology:
   COMPONENT   INSTANCE             ROLE     STATUS    AZ       NODE                   CREATED-TIME
   qdrant      mycluster-qdrant-0   <none>   Running   <none>   x-worker3/172.20.0.3   Aug 15,2023 23:03 UTC+0800
   qdrant      mycluster-qdrant-1   <none>   Running   <none>   x-worker2/172.20.0.5   Aug 15,2023 23:03 UTC+0800
   qdrant      mycluster-qdrant-2   <none>   Running   <none>   x-worker/172.20.0.2    Aug 15,2023 23:04 UTC+0800

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
   qdrant      false       1 / 1                1Gi / 1Gi               data:20Gi      standard

   Images:
   COMPONENT   TYPE     IMAGE
   qdrant      qdrant   docker.io/qdrant/qdrant:latest

   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE     BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME
   Disabled      <none>            <none>   7d           <none>          <none>

   Show cluster events: kbcli cluster list-events -n demo mycluster
   ```

</TabItem>

</Tabs>

## 连接到 Qdrant 集群

Qdrant 提供两种客户端访问协议：HTTP 和 gRPC，它们分别使用端口 6333 和 6334 进行通信。根据客户端所在的位置，你可以使用不同的方法连接到 Qdrant 集群。

:::note

如果你的集群在 AWS 上，请先安装 AWS 负载均衡控制器。

:::

- 如果客户端在 K8s 集群内部，执行 `kbcli cluster describe qdrant` 命令获取集群的 ClusterIP 地址或相应的 K8s 集群域名。
- 如果客户端在 K8s 集群外部但在同一 VPC 内，执行 `kbcli cluster expose qdrant --enable=true --type=vpc` 命令获取数据库集群的 VPC 负载均衡器地址。
- 如果客户端在 VPC 外部，执行 `kbcli cluster expose qdrant --enable=true --type=internet` 命令打开数据库集群的公共网络可达地址。

## 扩缩容

### 水平扩缩容

水平扩缩容改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容文档](./../maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

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
     - componentName: qdrant
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   如果您想要缩容，可将 `scaleOut` 替换为 `scaleIn`。

   以下示例演示了删除 2 个副本。

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
     - componentName: qdrant
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. 查看运维操作状态，验证水平扩缩容是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
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
   ```

   在编辑器中修改 `spec.componentSpecs.replicas` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       replicas: 2 # 修改该参数值
   ...
   ```

2. 当集群状态再次回到 `Running` 后，查看相关资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

    配置参数 `--components` 和 `--replicas`，并执行以下命令。

    ```bash
    kbcli cluster hscale qdrant --replicas=5 --components=qdrant
    ```

2. 验证水平扩容是否完成。

    - 查看 OpsRequest 进度。

       执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

       ```bash
       kubectl get ops mycluster-horizontalscaling-xpdwz -n demo
       >
       NAME                                TYPE                CLUSTER      STATUS    PROGRESS   AGE
       mycluster-horizontalscaling-xpdwz   HorizontalScaling   mycluster    Running   0/2        16s
       ```

    - 查看集群状态。

       ```bash
       kbcli cluster list mycluster -n demo
       >
       NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
       mycluster   demo        qdrant                         Delete               Running   Jul 24,2023 11:38 UTC+0800
       ```

       - STATUS=Updating 表示正在进行水平扩容。
       - STATUS=Running 表示水平扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查相关资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl"  default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

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
     - componentName: qdrant
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
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

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
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       replicas: 1
       resources: # 修改资源参数值
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

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ```bash
   kbcli cluster vscale mycluster --cpu=0.5 --memory=512Mi --components=qdrant -n demo
   ```

2. 验证垂直扩缩容。

   - 查看 OpsRequest 进度。

     执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

     ```bash
     kubectl get ops qdrant-verticalscaling-rpw2l -n demo
     >
     NAME                              TYPE              CLUSTER      STATUS    PROGRESS   AGE
     mycluster-verticalscaling-rpw2l   VerticalScaling   mycluster    Running   1/5        44s
     ```

   - 查看集群状态。

     - STATUS=VerticalScaling 表示正在进行垂直扩容。
     - STATUS=Running 表示垂直扩容已完成。
     - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
       > 您可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看扩缩容是否已经完成。

   ```bash
   kbcli cluster describe mycluster -n demo
   ```

</TabItem>

</Tabs>

## 磁盘扩容

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl"  default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

***步骤：***

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

    ```yaml
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
      - componentName: qdrant
        volumeClaimTemplates:
        - name: data
          storage: "40Gi"
    EOF
    ```

2. 验证磁盘扩容操作是否成功。

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
    ```

    如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

3. 查看对应的集群资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

1. 更改集群 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的值。

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` 定义了 Pod 的存储资源信息，更改此值会触发磁盘扩容。

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   在编辑器中修改 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
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
   ...
   ```

2. 查看对应的集群资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

   配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

   ```bash
   kbcli cluster volume-expand mycluster --storage=40Gi --components=qdrant -t data -n demo
   ```

2. 验证扩容操作是否成功。

   - 查看 OpsRequest 进度。

     执行磁盘扩容命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、PVC 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

     ```bash
     kubectl get ops mycluster-volumeexpansion-5pbd2 -n demo
     >
     NAME                              TYPE              CLUSTER      STATUS   PROGRESS   AGE
     mycluster-volumeexpansion-5pbd2   VolumeExpansion   mycluster    Succeed  1/1        67s
     ```

   - 查看集群状态。

     ```bash
     kbcli cluster list mycluster -n demo
     ```

     * STATUS=Updating 表示扩容正在进行中。
     * STATUS=Running 表示扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已变更。

   ```bash
   kbcli cluster describe mycluster -n demo
   ```

</TabItem>

</Tabs>

## 重启

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 创建 OpsRequest 重启集群。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: qdrant
   EOF
   ```

2. 查看 Pod 和重启操作的状态，验证该操作是否成功。

   ```bash
   kubectl get pod -n demo
   kubectl get ops ops-restart -n demo
   ```

   重启过程中，Pod 有如下两种状态：

   - STATUS=Terminating：表示集群正在重启。
   - STATUS=Running：表示集群已重启。

   如果操作过程中出现报错，可通过 `kubectl describe ops -n demo` 查看该操作的事件，协助排障。

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 重启集群。

   配置 `--components` 和 `--ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart mycluster --components="qdrant" -n demo \
   --ttlSecondsAfterSucceed=30
   ```

   - `--components` 表示需要重启的组件名称。
   - `--ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME       NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster  demo        qdrant                 qdrant-1.8.1    Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

   * STATUS=Updating 表示集群正在重启中。
   * STATUS=Running 表示集群已重启。

</TabItem>

</Tabs>

## 停止/启动集群

您可以停止/启动集群以释放计算资源。停止集群后，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。您也可以重新启动该集群，使其恢复到停止集群前的状态。

### 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-stop
     namespace: demo
   spec:
     clusterName: mycluster
     type: Stop
   EOF
   ```

   </TabItem>

   <TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   将 replicas 设为 0，删除 pods。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     terminationPolicy: Delete
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       disableExporter: true  
       replicas: 0 # 修改该参数值
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. 查看集群状态，确认集群是否已停止。

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list mycluster -n demo
   ```

   </TabItem>

   </Tabs>

### 启动集群

1. 配置集群名称，并执行以下命令来启动该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-start
     namespace: demo
   spec:
     clusterName: mycluster
     type: Start
   EOF 
   ```

   </TabItem>

   <TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   将 replicas 修改为停止集群前的数量，再次启动集群。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     terminationPolicy: Delete
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       disableExporter: true  
       replicas: 1
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster start mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. 查看集群状态，确认集群是否已再次运行。

    <Tabs>

    <TabItem value="kubectl" label="kubectl" default>

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>

## 删除集群

### 终止策略

:::note

终止策略决定了删除集群的方式，可在创建集群时进行设置。

:::

| **终止策略** | **删除操作**                                                                     |
|:----------------------|:-------------------------------------------------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。                                                  |
| `Delete`              | `Delete` 删除 Pod、服务、PVC 等集群资源，删除所有持久数据。                              |
| `WipeOut`             | `WipeOut`  删除所有集群资源，包括外部存储中的卷快照和备份。使用该策略将会删除全部数据，特别是在非生产环境，该策略将会带来不可逆的数据丢失。请谨慎使用。   |

执行以下命令查看终止策略。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl -n demo get cluster mycluster
>
NAME           CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
mycluster      kafka                kafka-3.3.2    Delete               Running    19m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
```

</TabItem>

</Tabs>

### 步骤

执行以下命令，删除集群。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl delete -n demo cluster mycluster
```

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster delete mycluster
```

</TabItem>

</Tabs>
