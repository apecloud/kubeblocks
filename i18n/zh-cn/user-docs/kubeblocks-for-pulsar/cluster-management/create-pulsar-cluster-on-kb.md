---
title: 创建 Pulsar 集群
description: 如何创建 Pulsar 集群
keywords: [pulsar, 创建集群]
sidebar_position: 1
sidebar_label: 创建
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## 概述

KubeBlocks 可以通过良好的抽象快速集成新引擎，并支持 Pulsar 集群的创建和删除、集群组件的垂直扩缩容和水平扩缩容、存储扩容、重启和配置更改等。

本系列文档重点展示 KubeBlocks 对 Pulsar 日常运维能力的支持，包括集群创建、删除、重启等基本生命周期操作，以及水平扩容、垂直扩容、存储扩容、配置变更等高阶操作。

## 环境推荐

关于各组件的规格（如内存、CPU 和存储容量等），请参考 [Pulsar 官方文档](https://pulsar.apache.org/docs/3.1.x/)。

|      组件        |                                 所需副本数                                  |
| :--------------------  | :------------------------------------------------------------------------ |
|       zookeeper        |   测试环境 1 个，生产环境 3 个           |
|        bookies         |  测试环境至少 3 个，生产环境至少 4 个   |
|        broker          |      至少 1 个，生产环境建议 3 个       |
| recovery (可选)    | 至少 1 个；如果 bookie 未启用 autoRecovery 功能，则至少需要 3 个 |
|   proxy (可选)     |         至少 1 个；生产环境需要 3 个           |

### 开始之前

* 如果您想通过 `kbcli` 创建和管理集群，请先[安装 kbcli](./../../installation/install-kbcli.md)。
* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* 确保 Pulsar 引擎已启用。如果引擎未启用，可参考相关文档，[启用该引擎](./../../installation/install-addons.md)。

* 查看可用于创建集群的数据库类型和版本。

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get clusterdefinition pulsar
  >
  NAME    TOPOLOGIES                                        SERVICEREFS    STATUS      AGE
  pulsar  pulsar-basic-cluster,pulsar-enhanced-cluster                     Available   16m
  ```

  查看可用版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=pulsar
  >
  NAME            CLUSTER-DEFINITION   STATUS      AGE
  pulsar-2.11.2   pulsar               Available   16m
  pulsar-3.0.2    pulsar               Available   16m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list
  kbcli clusterversion list
  ```

  </TabItem>

  </Tabs>

* 为了保持隔离，本文档中创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

## 创建 Pulsar 集群

1. 创建基础模式的 Pulsar 集群。如需创建其他集群模式，您可查看 [GitHub 仓库中的示例](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/pulsar)。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: pulsar
     topology: pulsar-basic-cluster
     services:
       - name: broker-bootstrap
         serviceName: broker-bootstrap
         componentSelector: broker
         spec:
           type: ClusterIP
           ports:
             - name: pulsar
               port: 6650
               targetPort: 6650
             - name: http
               port: 80
               targetPort: 8080
             - name: kafka-client
               port: 9092
               targetPort: 9092
       - name: zookeeper
         serviceName: zookeeper
         componentSelector: zookeeper
         spec:
           type: ClusterIP
           ports:
             - name: client
               port: 2181
               targetPort: 2181
     componentSpecs:
       - name: broker
         serviceVersion: 3.0.2
         replicas: 1
         env:
           - name: KB_PULSAR_BROKER_NODEPORT
             value: "false"
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "200m"
             memory: "512Mi"
       - name: bookies
         serviceVersion: 3.0.2
         replicas: 4
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "200m"
             memory: "512Mi"
         volumeClaimTemplates:
           - name: ledgers
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
           - name: journal
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
       - name: zookeeper
         serviceVersion: 3.0.2
         replicas: 1
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "100m"
             memory: "512Mi"
         volumeClaimTemplates:
           - name: data
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
   EOF
   ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | 集群终止策略，有效值为 `DoNotTerminate`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-pulsar-cluster.md#终止策略)。 |
   | `spec.clusterDef` | 指定了创建集群时要使用的 ClusterDefinition 的名称。**注意**：**请勿更新此字段**。创建 Pulsar 集群时，该值必须为 `pulsar`。 |
   | `spec.topology` | 指定了在创建集群时要使用的 ClusterTopology 的名称。 |
   | `spec.services` | 定义了集群暴露的额外服务列表。 |
   | `spec.componentSpecs`                 | 集群 component 列表，定义了集群 components。该字段支持自定义配置集群中每个 component。  |
   | `spec.componentSpecs.serviceVersion`  | 定义了 component 部署的服务版本。有效值为 [2.11.2,3.0.2]。 |
   | `spec.componentSpecs.disableExporter` | 定义了是否在 component 无头服务（headless service）上标注指标 exporter 信息，是否开启监控 exporter。有效值为 [true, false]。 |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |
   | `spec.componentSpecs.volumeClaimTemplates` | PersistentVolumeClaim 模板列表，定义 component 的存储需求。 |
   | `spec.componentSpecs.volumeClaimTemplates.name` | 引用了在 `componentDefinition.spec.runtime.containers[*].volumeMounts` 中定义的 volumeMount 名称。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | 定义了 StorageClass 的名称。如果未指定，系统将默认使用带有 `storageclass.kubernetes.io/is-default-class=true` 注释的 StorageClass。  |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | 可按需配置存储容量。 |

   您可参考 [API 文档](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster)，查看更多 API 字段及说明。

2. 验证已创建的集群。

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    当状态显示为 `Running` 时，表示集群已成功创建。
