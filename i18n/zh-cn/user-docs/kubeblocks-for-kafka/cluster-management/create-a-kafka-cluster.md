---
title: 创建 Kafka 集群
description: 如何创建 Kafka 集群
keywords: [kafka, 集群, 管理]
sidebar_position: 1
sidebar_label: 创建
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 创建 Kafka 集群

本文档展示如何创建一个 Kafka 集群。

## 开始之前

* 如果您想通过 `kbcli` 创建和管理集群，请先[安装 kbcli](./../../installation/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* 确保 Kafka 引擎 已启用。如果引擎未启用，可参考相关文档，[启用该引擎](./../../installation/install-addons.md)。

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io kafka
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  kafka   Helm                        Enabled   13m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  kafka                          Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* 为保持隔离，本教程中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  ```

:::note

- KubeBlocks 集成了 Kafka v3.3.2，以 KRaft 模式运行。
- 在生产环境中，不建议以组合模式使用 KRaft 集群。
- 建议将控制器数量设置在 3 到 5 个之间，实现复杂性和可用性的平衡。

:::

## 创建 Kafka 集群

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. 创建 Kafka 集群。

   KubeBlocks 通过 `Cluster` 定义集群。以下为创建不同模式 Kafka 集群的示例。

   如果您只有一个节点可用于部署集群版，可将 `spec.affinity.topologyKeys` 设置为 `null`。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

   * 创建组合模式的 Kafka 集群。

     ```yaml
     # 组合模式
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1
     kind: Cluster
     metadata:
       name: mycluster
       namespace: demo
     spec:
       terminationPolicy: Delete
       clusterDef: kafka
       topology: combined_monitor
       componentSpecs:
         - name: kafka-combine
           env:
             - name: KB_KAFKA_BROKER_HEAP # use this ENV to set BROKER HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_KAFKA_CONTROLLER_HEAP # use this ENV to set CONTOLLER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_BROKER_DIRECT_POD_ACCESS # set to FALSE for node-port
               value: "true"
           replicas: 1
           resources:
             limits:
               cpu: "1"
               memory: "1Gi"
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
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-exporter 
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "1Gi"
             requests:
               cpu: "0.1"
               memory: "0.2Gi"
     EOF
     ```

   * 创建分离模式的 Kafka 集群。

     ```yaml
     # 分离模式
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1
     kind: Cluster
     metadata:
       name: mycluster
       namespace: demo
     spec:
       terminationPolicy: Delete
       clusterDef: kafka
       topology: separated_monitor
       componentSpecs:
         - name: kafka-broker
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "0.5Gi"
             requests:
               cpu: "0.5"
               memory: "0.5Gi"
           env:
             - name: KB_KAFKA_BROKER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_KAFKA_CONTROLLER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_BROKER_DIRECT_POD_ACCESS
               value: "true"
           volumeClaimTemplates:
             - name: data
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 20Gi
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-controller
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "0.5Gi"
             requests:
               cpu: "0.5"
               memory: "0.5Gi"
           volumeClaimTemplates:
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-exporter
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "1Gi"
             requests:
               cpu: "0.1"
               memory: "0.2Gi"
     EOF
     ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-mysql-cluster.md#终止策略)。 |
   | `spec.clusterDef` | 指定了创建集群时要使用的 ClusterDefinition 的名称。**注意**：**请勿更新此字段**。创建 Kafka 集群时，该值必须为 `kafka`。 |
   | `spec.topology` | 指定了在创建集群时要使用的 ClusterTopology 的名称。有效值为 [combined,combined_monitor,separated,separated_monitor]。 |
   | `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |
   | `spec.componentSpecs.volumeClaimTemplates` | PersistentVolumeClaim 模板列表，定义 component 的存储需求。 |
   | `spec.componentSpecs.volumeClaimTemplates.name` | 引用了在 `componentDefinition.spec.runtime.containers[*].volumeMounts` 中定义的 volumeMount 名称。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | 定义了 StorageClass 的名称。如果未指定，系统将默认使用带有 `storageclass.kubernetes.io/is-default-class=true` 注释的 StorageClass。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | 可按需配置存储容量。 |

   您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

2. 查看集群是否创建成功。

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
   mycluster   kafka                kafka-3.3.2   Delete               Running   2m2s
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. 创建 Kafka 集群。

   使用 `kbcli cluster create` 命令创建集群。您还可以使用 `--set` 参数自定义集群资源。

   ```bash
   kbcli cluster create kafka mycluster -n demo
   ```

   如果您需要自定义集群规格，kbcli 也提供了诸多参数，如支持设置引擎版本、终止策略、CPU、内存规格。您可通过在命令结尾添加 `--help` 或 `-h` 来查看具体说明。比如，

   ```bash
   kbcli cluster create kafka --help
   kbcli cluster create kafka -h
   ```

2. 验证集群是否创建成功。

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
   ```

</TabItem>

</Tabs>
