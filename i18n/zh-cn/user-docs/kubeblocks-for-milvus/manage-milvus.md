---
title: 用 KubeBlocks 管理 Milvus
description: 如何用 KubeBlocks 管理 Milvus
keywords: [milvus, 向量数据库]
sidebar_position: 1
sidebar_label: 用 KubeBlocks 管理 Milvus
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 用 KubeBlocks 管理 Milvus

生成式人工智能的爆火引发了人们对向量数据库的关注。目前，KubeBlocks 支持 Milvus 的管理和运维。本文档展示如何使用 KubeBlocks 管理 Milvus。

Milvus 是高度灵活、可靠且速度极快的云原生开源矢量数据库。它为 embedding 相似性搜索和 AI 应用程序提供支持，并努力使每个组织都可以访问矢量数据库。 Milvus 可以存储、索引和管理由深度神经网络和其他机器学习 (ML) 模型生成的十亿级别以上的 embedding 向量。

本文档展示了如何通过 kbcli、kubectl 或 YAML 文件等当时创建和管理  Milvus 集群。您可以在 [GitHub 仓库](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/milvus)查看 YAML 示例。

## 开始之前

- 如果您想通过 `kbcli` 创建并连接 Milvus 集群，请先[安装 kbcli](./../installation/install-kbcli.md)。
- [安装 KubeBlocks](./../installation/install-kubeblocks.md)。
- [安装并启用 milvus 引擎](./../installation/install-addons.md)。
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

KubeBlocks 通过 `Cluster` 定义集群。以下是创建 Milvus 集群的示例。Pod 默认分布在不同节点。但如果您只有一个节点可用于部署集群，可将 `spec.affinity.topologyKeys` 设置为 `null`。

:::note

生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

:::

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  namespace: demo
  name: mycluster
spec:
  terminationPolicy: Delete
  clusterDef: milvus
  topology: cluster
  componentSpecs:
    - name: proxy
      replicas: 1
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      serviceRefs:
        - name: milvus-meta-storage 
          namespace: demo        
          clusterServiceSelector:
            cluster: etcdm-cluster  
            service:
              component: etcd       
              service: headless     
              port: client          
        - name: milvus-log-storage
          namespace: demo
          clusterServiceSelector:
            cluster: pulsarm-cluster
            service:
              component: broker
              service: headless
              port: pulsar
        - name: milvus-object-storage
          namespace: demo
          clusterServiceSelector:
            cluster: miniom-cluster
            service:
              component: minio
              service: headless
              port: http
            credential:            
              component: minio     
              name: admin          
      disableExporter: true
    - name: mixcoord
      replicas: 1
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      serviceRefs:
        - name: milvus-meta-storage
          namespace: demo
          clusterServiceSelector:
            cluster: etcdm-cluster
            service:
              component: etcd
              service: headless
              port: client

        - name: milvus-log-storage
          namespace: demo
          clusterServiceSelector:
            cluster: pulsarm-cluster
            service:
              component: broker
              service: headless
              port: pulsar

        - name: milvus-object-storage
          namespace: demo
          clusterServiceSelector:
            cluster: miniom-cluster
            service:
              component: minio
              service: headless
              port: http
            credential:
              component: minio
              name: admin

      disableExporter: true
    - name: datanode
      replicas: 1
      disableExporter: true
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      serviceRefs:
        - name: milvus-meta-storage
          namespace: demo
          clusterServiceSelector:
            cluster: etcdm-cluster
            service:
              component: etcd
              service: headless
              port: client

        - name: milvus-log-storage
          namespace: demo
          clusterServiceSelector:
            cluster: pulsarm-cluster
            service:
              component: broker
              service: headless
              port: pulsar

        - name: milvus-object-storage
          namespace: demo
          clusterServiceSelector:
            cluster: miniom-cluster
            service:
              component: minio
              service: headless
              port: http
            credential:
              component: minio
              name: admin

      disableExporter: true
    - name: indexnode
      replicas: 1
      disableExporter: true
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      serviceRefs:
        - name: milvus-meta-storage
          namespace: demo
          clusterServiceSelector:
            cluster: etcdm-cluster
            service:
              component: etcd
              service: headless
              port: client

        - name: milvus-log-storage
          namespace: demo
          clusterServiceSelector:
            cluster: pulsarm-cluster
            service:
              component: broker
              service: headless
              port: pulsar

        - name: milvus-object-storage
          namespace: demo
          clusterServiceSelector:
            cluster: miniom-cluster
            service:
              component: minio
              service: headless
              port: http
            credential:
              component: minio
              name: admin

      disableExporter: true
    - name: querynode
      replicas: 1
      disableExporter: true
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      serviceRefs:
        - name: milvus-meta-storage
          namespace: demo
          clusterServiceSelector:
            cluster: etcdm-cluster
            service:
              component: etcd
              service: headless
              port: client

        - name: milvus-log-storage
          namespace: demo
          clusterServiceSelector:
            cluster: pulsarm-cluster
            service:
              component: broker
              service: headless
              port: pulsar

        - name: milvus-object-storage
          namespace: demo
          clusterServiceSelector:
            cluster: miniom-cluster
            service:
              component: minio
              service: headless
              port: http
            credential:
              component: minio
              name: admin

      disableExporter: true
EOF
```

| 字段                                   | 定义  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](#终止策略)。 |
| `spec.clusterDef` | 指定了创建集群时要使用的 ClusterDefinition 的名称。**注意**：**请勿更新此字段**。创建 Milvus 集群时，该值必须为 `milvus`。 |
| `spec.topology` | 指定了在创建集群时要使用的 ClusterTopology 的名称。可选值为[standalone, cluster]。 |
| `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
| `spec.componentSpecs.serviceRefs` | 定义了 component 的 ServiceRef 列表。 |
| `spec.componentSpecs.serviceRefs.name` | 指定了服务引用声明的标识符，该标识符在 `componentDefinition.spec.serviceRefDeclarations[*].name` 中定义。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector` | 引用了另一个 KubeBlocks 集群提供的服务。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.cluster` | 定义了集群名称，您可以按需修改。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.service.component` | 定义了组件名称。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.service.service` | 引用了默认的无头服务（headless Service）。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.service.port` | 引用了端口名称。 |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.credential` | 指定了用于验证并与被引用集群建立连接的系统账号（SystemAccount）。  |
| `spec.componentSpecs.serviceRefs.clusterServiceSelector.credential.name` | 指定了要引用的凭证（SystemAccount）名称，本例中使用 'admin' 账号。 |
| `spec.componentSpecs.disableExporter` | 定义了是否在 component 无头服务（headless service）上标注指标 exporter 信息，是否开启监控 exporter。有效值为 [true, false]。 |
| `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
| `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

KubeBlocks operator 监控 `Cluster` CRD 并创建集群和全部依赖资源。您可执行以下命令获取集群创建的所有资源信息。

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

执行以下命令，查看已创建的 Milvus 集群：

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建一个 Milvus 集群。

   ```bash
   kbcli cluster create mycluster --cluster-definition=milvus-2.3.2 -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create milvus --help

   kbcli cluster create milvus -h
   ```

2. 检查集群是否已创建。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION        VERSION               TERMINATION-POLICY   STATUS           CREATED-TIME
   mycluster   demo        milvus-2.3.2                                    Delete               Running          Jul 05,2024 17:35 UTC+0800 
   ```

3. 查看集群信息。

   ```bash
   kbcli cluster describe mycluster -n demo
   >
   Name: milvus	 Created Time: Jul 05,2024 17:35 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION   STATUS    TERMINATION-POLICY   
   demo        milvus-2.3.2                   Running   Delete               

   Endpoints:
   COMPONENT   MODE        INTERNAL                                        EXTERNAL   
   milvus      ReadWrite   milvus-milvus.default.svc.cluster.local:19530   <none>     
   minio       ReadWrite   milvus-minio.default.svc.cluster.local:9000     <none>     
   proxy       ReadWrite   milvus-proxy.default.svc.cluster.local:19530    <none>     
                           milvus-proxy.default.svc.cluster.local:9091                

   Topology:
   COMPONENT   INSTANCE             ROLE     STATUS    AZ       NODE     CREATED-TIME                 
   etcd        milvus-etcd-0        <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   minio       milvus-minio-0       <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   milvus      milvus-milvus-0      <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   indexnode   milvus-indexnode-0   <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   mixcoord    milvus-mixcoord-0    <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   querynode   milvus-querynode-0   <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   datanode    milvus-datanode-0    <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   
   proxy       milvus-proxy-0       <none>   Running   <none>   <none>   Jul 05,2024 17:35 UTC+0800   

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS     
   milvus      false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   etcd        false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   minio       false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   proxy       false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   mixcoord    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   datanode    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   indexnode   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
   querynode   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   

   Images:
   COMPONENT   TYPE        IMAGE                                                
   milvus      milvus      milvusdb/milvus:v2.3.2                               
   etcd        etcd        docker.io/milvusdb/etcd:3.5.5-r2                     
   minio       minio       docker.io/minio/minio:RELEASE.2022-03-17T06-34-49Z   
   proxy       proxy       milvusdb/milvus:v2.3.2                               
   mixcoord    mixcoord    milvusdb/milvus:v2.3.2                               
   datanode    datanode    milvusdb/milvus:v2.3.2                               
   indexnode   indexnode   milvusdb/milvus:v2.3.2                               
   querynode   querynode   milvusdb/milvus:v2.3.2                               

   Show cluster events: kbcli cluster list-events -n demo milvus
   ```

</TabItem>

</Tabs>

## 扩缩容

当前，KubeBlocks 支持垂直扩缩容 Milvus 集群。

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster   milvus-2.3.2                                  Delete               Running   4m29s
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        milvus-2.3.2                           Delete               Running   Jul 05,2024 17:35 UTC+0800
```

</TabItem>

</Tabs>

### 步骤

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
     - componentName: milvus
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

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: milvus
     clusterVersionRef: milvus-2.3.2
     componentSpecs:
     - name: milvus
       componentDefRef: milvus
       replicas: 1
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

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

    ```bash
    kbcli cluster vscale milvus -n demo --cpu=1 --memory=1Gi --components=milvus 
    ```

2. 通过以下任意一种方式验证垂直扩容是否完成。

   - 查看 OpsRequest 进度。

     执行命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

     ```bash
     kbcli cluster describe-ops milvus-verticalscaling-rpw2l -n demo
     ```

   - 查看集群状态。

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
     mycluster   demo                                               Delete               Updating   Jul 05,2024 17:35 UTC+0800
     ```

     - STATUS=VerticalScaling 表示正在进行垂直扩容。
     - STATUS=Running 表示垂直扩容已完成。
     - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
       > 您可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已变更。

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## 磁盘扩容

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster   milvus-2.3.2                                  Delete               Running   4m29s
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        milvus-2.3.2                           Delete               Running   Jul 05,2024 17:35 UTC+0800
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
      name: ops-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: milvus
        volumeClaimTemplates:
        - name: data
          storage: "40Gi"
    EOF
    ```

2. 查看运维任务状态，验证垂直扩缩容操作是否成功。

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
    ```

    如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的配置。`spec.componentSpecs.volumeClaimTemplates.spec.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   在编辑器中修改 `spec.componentSpecs.volumeClaimTemplates.spec.resources` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: milvus
     clusterVersionRef: milvus-2.3.2
     componentSpecs:
     - name: milvus
       componentDefRef: milvus
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # 修改磁盘容量
   ...
   ```

2. 当集群状态再次回到 `Running` 后，查看相应资源是否变更。

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 更改配置。配置参数 `--components`、`--volume-claim-templates` 和 `--storage`，并执行以下命令。

   ```bash
   kbcli cluster volume-expand milvus --storage=40Gi --components=milvus
   ```

   - `--components` 表示需扩容的组件名称。
   - `--volume-claim-templates` 表示组件中的 VolumeClaimTemplate 名称。
   - `--storage` 表示磁盘需扩容至的大小。

2. 可通过以下任意一种方式验证扩容操作是否完成。

    - 查看 OpsRequest 进度。

       执行磁盘扩容命令后，KubeBlocks 会自动输出查看 OpsRequest 进度的命令，可通过该命令查看 OpsRequest 进度的细节，包括 OpsRequest 的状态、Pod 状态等。当 OpsRequest 的状态为 `Succeed` 时，表明这一任务已完成。

       ```bash
       kbcli cluster describe-ops milvus-volumeexpansion-5pbd2 -n demo
       ```

    - 查看集群状态。

       ```bash
       kbcli cluster list mycluster -n demo
       ```

       - STATUS=Updating 表示扩容正在进行中。
       - STATUS=Running 表示扩容已完成。

3. 当 OpsRequest 状态为 `Succeed` 或集群状态再次回到 `Running` 后，检查资源规格是否已按要求变更。

   ```bash
   kbcli cluster describe mycluster -n demo
   ```

</TabItem>

</Tabs>

## 重启

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 执行以下命令，重启集群。

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
     - componentName: milvus
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

1. 重启集群。

   配置 `--components` 和 `--ttlSecondsAfterSucceed` 的值，重启指定集群。

   ```bash
   kbcli cluster restart mycluster --components="milvus" -n demo \
     --ttlSecondsAfterSucceed=30
   ```

   - `--components` 表示需要重启的组件名称。
   - `--ttlSecondsAfterSucceed` 表示重启成功后 OpsRequest 作业的生存时间。

2. 验证重启操作。

   执行以下命令检查集群状态，并验证重启操作。

   ```bash
   kbcli cluster list milvus -n demo
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
   milvus   default     milvus-2.3.2           milvus-2.3.2    Delete               Running   Jul 05,2024 18:35 UTC+0800
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

    将 replicas 设为 0，删除 Pods。

    ```yaml
    ...
    spec:
      clusterDefinitionRef: milvus
      clusterVersionRef: milvus-2.3.2
      terminationPolicy: Delete
      componentSpecs:
      - name: milvus
        componentDefRef: milvus
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

    将 replicas 数值改为停止集群前的数值，再次启动集群。

    ```yaml
    ...
    spec:
      clusterDefinitionRef: milvus
      clusterVersionRef: milvus-2.3.2
      terminationPolicy: Delete
      componentSpecs:
      - name: milvus
        componentDefRef: milvus
        disableExporter: true  
        replicas: 1 # 修改该参数值
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
kubectl -n demo get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster   milvus-2.3.2                                  Delete               Running   29m 
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        milvus-2.3.2                           Delete               Running   Jul 05,2024 17:35 UTC+0800
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
