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

1. 创建 Pulsar 集群。

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
     annotations:
       "kubeblocks.io/extra-env": '{"KB_PULSAR_BROKER_NODEPORT": "false"}'
   spec:
     terminationPolicy: Delete
     services:
     - name: proxy
       serviceName: proxy
       componentSelector: pulsar-proxy
       spec:
         type: ClusterIP
         ports:
         - name: pulsar
           port: 6650
           targetPort: 6650
         - name: http
           port: 80
           targetPort: 8080
     - name: broker-bootstrap
       serviceName: broker-bootstrap
       componentSelector: pulsar-broker
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
     componentSpecs:
     - name: pulsar-broker
       componentDef: pulsar-broker
       disableExporter: true
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
     - name: pulsar-proxy
       componentDef: pulsar-proxy
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
     - name: bookies
       componentDef: pulsar-bookkeeper
       replicas: 3
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
       volumeClaimTemplates:
       - name: journal
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
       - name: ledgers
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
     - name: bookies-recovery
       componentDef: pulsar-bkrecovery
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
     - name: zookeeper
       componentDef: pulsar-zookeeper
       replicas: 3
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
   | `metadata.annotations."kubeblocks.io/extra-env"` | 定义了是否启用 NodePort 服务。 |
   | `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。 <p> - `DoNotTerminate` 会阻止删除操作。 </p><p> - `Halt` 会删除工作负载资源，如 statefulset 和 deployment 等，但是保留了 PVC 。  </p><p> - `Delete` 在 `Halt` 的基础上进一步删除了 PVC。 </p><p> - `WipeOut` 在 `Delete` 的基础上从备份存储的位置完全删除所有卷快照和快照数据。 </p>|
   | `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
   | `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
   | `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
   | `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
   | `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
   | `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition postgresql -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
   | `spec.componentSpecs.name`            | 定义了 component 的名称。  |
   | `spec.componentSpecs.disableExporter` | 定义了是否开启监控功能。 |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

2. 验证已创建的集群。

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    当状态显示为 `Running` 时，表示集群已成功创建。
