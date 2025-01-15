---
title: 用 KubeBlocks 管理 StarRocks
description: 如何使用 KubeBlocks 管理 StarRocks
keywords: [starrocks, 分析型数据库, data warehouse]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 StarRocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 用 KubeBlocks 管理 StarRocks

StarRocks 是一款高性能分析型数据仓库，使用向量化、MPP 架构、CBO、智能物化视图、可实时更新的列式存储引擎等技术实现多维、实时、高并发的数据分析。

本文档展示了如何通过 kbcli、kubectl 或 YAML 文件等当时创建和管理 StarRocks 集群。您可以在 [GitHub 仓库](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/starrocks)查看 YAML 示例。

## 开始之前

- 如果您想通过 `kbcli` 创建并连接 StarRocks 集群，请先[安装 kbcli](./../installation/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-kubeblocks.md)。
- [安装并启用 starrocks 引擎](./../installation/install-addons.md)。

## 创建集群

***步骤：***

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 StarRocks 集群的示例。Pod 默认分布在不同节点。如果您只有一个节点可用于部署多副本集群，可设置 `spec.schedulingPolicy` 或 `spec.componentSpecs.schedulingPolicy`，具体可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy)。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

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
  - name: fe
    componentDef: starrocks-ce-fe
    serviceAccountName: kb-starrocks-cluster
    replicas: 1
    resources:
      limits:
        cpu: '1'
        memory: 1Gi
      requests:
        cpu: '1'
        memory: 1Gi
  - name: be
    componentDef: starrocks-ce-be
    replicas: 2
    resources:
      limits:
        cpu: '1'
        memory: 1Gi
      requests:
        cpu: '1'
        memory: 1Gi
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
| `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](#终止策略)。 |
| `spec.componentSpecs`                 | ClusterComponentSpec 对象的列表，定义组成集群 components。该字段支持自定义配置集群中每个 component。注意：`shardingSpecs` 和 `componentSpecs` 不能同时为空；必须至少定义一个才能配置集群。ClusterComponentSpec 定义集群中组件的规格。 |
| `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
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

执行以下命令，查看已创建的 StarRocks 集群。

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## 扩缩容

### 垂直扩缩容

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800  
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
     name: mycluster-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: fe
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
   NAMESPACE   NAME                         TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        mycluster-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
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
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: fe
       componentDefRef: fe
       replicas: 2
       resources: # 修改 resources 下的参数值
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
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
    kbcli cluster vscale mycluster --cpu=2 --memory=20Gi --components=be -n demo
    ```

2. 通过以下任意一种方式验证垂直扩容是否完成。

    - 查看 OpsRequest 进度。

       执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 Succeed 时，表明这一任务已完成。

       ```bash
       kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n demo
       ```

    - 查看集群状态。

       ```bash
       kbcli cluster list mycluster -n demo
       ```

       - STATUS=Updating 表示正在进行垂直扩容。
       - STATUS=Running 表示垂直扩容已完成。
       - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
          > 您可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者您也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### 水平扩缩容

水平扩展改变 Pod 的数量。例如，您可以将副本从三个扩展到五个。

从 v0.9.0 开始，KubeBlocks 还支持了指定实例扩缩容。可通过 [水平扩缩容文档](./../maintenance/scale/horizontal-scale.md) 文档了解更多细节和示例。

#### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800  
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
     name: mycluster-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: fe
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
     name: mycluster-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: fe
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. 查看运维操作状态，验证水平扩缩容是否成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                           TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        mycluster-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
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
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: fe
       componentDefRef: fe
       replicas: 2 # 修改参数值
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
    kbcli cluster hscale mycluster --replicas=3 --components=be -n demo
    ```

    - `--components` 表示准备进行水平扩容的组件名称。
    - `--replicas` 表示指定组件的副本数。 根据需要设定数值，进行扩缩容。

2. 通过以下任意一种方式验证水平扩容是否完成。

    - 查看 OpsRequest 进度。

       执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 Succeed 时，表明这一任务已完成。

       ```bash
       kbcli cluster describe-ops mycluster-horizontalscaling-smx8b -n demo
       ```

    - 查看集群状态。
  
       ```bash
       kbcli cluster list mycluster -n demo
       ```

       - STATUS=Updating 表示正在进行水平扩容。
       - STATUS=Running 表示水平扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## 磁盘扩容

### 开始之前

确认集群状态是否为 `Running`。否则，后续相关操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800  
```

</TabItem>

</Tabs>

### 步骤

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 应用 OpsRequest。根据需求更改 storage 的值，并执行以下命令来更改集群的存储容量。

    ```yaml
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: mycluster-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: be
        volumeClaimTemplates:
        - name: be-storage
          storage: "40Gi"
    EOF
    ```

2. 查看磁盘扩容操作是否成功。

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                         TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        mycluster-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
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
   ```

   在编辑器中修改 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: be
       componentDefRef: be
       volumeClaimTemplates:
       - name: be-storage
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # 修改该参数值
   ...
   ```

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。

    配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

    ```bash
    kbcli cluster volume-expand mycluster --storage=40Gi --components=be -n demo
    ```

    - `--components` 表示需扩容的组件名称。
    - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
    - `--storage` 表示磁盘需扩容至的大小。

2. 可通过以下任意一种方式验证扩容操作是否完成。

   - 查看 OpsRequest 进度。

      执行磁盘扩容命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、PVC 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

      ```bash
      kbcli cluster describe-ops mycluster-volumeexpansion-smx8b -n default
      ```

   - 查看集群状态。

      ```bash
      kbcli cluster list mycluster -n demo
      >
      NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS      CREATED-TIME
      mycluster   demo        starrocks            starrocks-3.1.1   Delete               Updating    Jul 17,2024 19:06 UTC+0800  
      ```

     * STATUS=Updating 表示扩容正在进行中。
     * STATUS=Running 表示扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已按要求变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## 停止/启动集群

您可以停止/启动集群以释放计算资源。当集群停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。您也可以重新启动该集群，使其恢复到停止集群前的状态。

## 停止集群

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
      clusterDefinitionRef: starrocks-ce
      clusterVersionRef: starrocks-ce-3.1.1
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
      - name: fe
        componentDefRef: fe
        serviceAccountName: kb-starrocks-cluster
        replicas: 0
      - name: be
        componentDefRef: be
        replicas: 0
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
    spec:
      clusterDefinitionRef: starrocks-ce
      clusterVersionRef: starrocks-ce-3.1.1
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
      - name: fe
        componentDefRef: fe
        serviceAccountName: kb-starrocks-cluster
        replicas: 1
      - name: be
        componentDefRef: be
        replicas: 2
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

## 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

:::note

集群重启后，主节点可能会发生变化。

:::

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

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
     - componentName: be
   EOF
   ```

2. 查看 pod 和运维操作状态，验证重启操作是否成功。

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

1. 执行以下命令，重启集群。

   配置 `components` 和 `ttlSecondsAfterSucceed` 的值，执行以下命令来重启指定集群。

   ```bash
   kbcli cluster restart mycluster --components="starrocks" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` 表示需要重启的组件名称。
   - `ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启是否成功。

   检查集群状态，验证重启操作是否成功。

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     starrocks              starrocks-3.1.1    Delete               Running   Jul 17,2024 19:06 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

</TabItem>

</Tabs>

## 删除集群

### 终止策略

:::note

终止策略决定了删除集群的方式。

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
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800  
```

</TabItem>

</Tabs>

### 步骤

执行以下命令，删除集群。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl delete cluster mycluster -n demo
```

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster delete mycluster -n demo
```

</TabItem>

</Tabs>
