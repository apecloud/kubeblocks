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

* 如果您想通过 `kbcli` 创建和管理集群，请先[安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 安装 KubeBlocks，可通过 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](./../../installation/install-with-helm/install-kubeblocks.md) 安装。
* 确保 Kafka 引擎 已启用。如果引擎未启用，可参考相关文档，启用该引擎，可通过 [kbcli](./../../installation/install-with-kbcli/install-addons.md) 或 [Helm](./../../installation/install-with-kbcli/install-addons.md) 操作。

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  kafka                        Helm   Enabled                   true
  ...
  ```

  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  kafka                          Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io kafka
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  kafka   Helm                        Enabled   13m
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

<TabItem value="kbcli" label="kbcli" default>

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

<TabItem value="kubectl" label="kubectl">

1. 创建 Kafka 集群。

   KubeBlocks 通过 `Cluster` 定义集群。以下为创建不同模式 Kafka 集群的示例。

   如果您只有一个节点可用于部署集群版，可将 `spec.affinity.topologyKeys` 设置为 `null`。但生产环境中，不建议将所有副本部署在同一个节点上，因为这可能会降低集群的可用性。

   * 创建组合模式的 Kafka 集群。

     ```yaml
     # create kafka in combined mode 
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1alpha1
     kind: Cluster
     metadata:
       name: mycluster
       namespace: demo
       annotations:
         "kubeblocks.io/extra-env": '{"KB_KAFKA_ENABLE_SASL":"false","KB_KAFKA_BROKER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_CONTROLLER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_PUBLIC_ACCESS":"false", "KB_KAFKA_BROKER_NODEPORT": "false"}'
         kubeblocks.io/enabled-pod-ordinal-svc: broker
     spec:
       clusterDefinitionRef: kafka
       clusterVersionRef: kafka-3.3.2
       terminationPolicy: Delete
       affinity:
         podAntiAffinity: Preferred
         topologyKeys:
         - kubernetes.io/hostname
       tolerations:
         - key: kb-data
           operator: Equal
           value: "true"
           effect: NoSchedule
       services:
       - name: bootstrap
         serviceName: bootstrap
         componentSelector: broker
         spec:
           type: ClusterIP
           ports:
           - name: kafka-client
             targetPort: 9092
             port: 9092
       componentSpecs:
       - name: broker
         componentDef: kafka-combine
         tls: false
         replicas: 1
         serviceAccountName: kb-kafka-cluster
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
         - name: metadata
           spec:
             accessModes:
             - ReadWriteOnce
             resources:
               requests:
                 storage: 20Gi
       - name: metrics-exp
         componentDefRef: kafka-exporter
         componentDef: kafka-exporter
         replicas: 1
         resources:
           limits:
             cpu: '0.5'
             memory: 0.5Gi
           requests:
             cpu: '0.5'
             memory: 0.5Gi
     EOF
     ```

   * 创建分离模式的 Kafka 集群。

     ```yaml
     # 分离模式
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1alpha1
     kind: Cluster
     metadata:
       name: kafka-cluster
       namespace: demo
       annotations:
         "kubeblocks.io/extra-env": '{"KB_KAFKA_ENABLE_SASL":"false","KB_KAFKA_BROKER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_CONTROLLER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_PUBLIC_ACCESS":"false", "KB_KAFKA_BROKER_NODEPORT": "false"}'
         kubeblocks.io/enabled-pod-ordinal-svc: broker
     spec:
       clusterDefinitionRef: kafka
       clusterVersionRef: kafka-3.3.2
       terminationPolicy: Delete
       affinity:
         podAntiAffinity: Preferred
         topologyKeys:
         - kubernetes.io/hostname
         tolerations:
           - key: kb-data
             operator: Equal
             value: "true"
             effect: NoSchedule
         services:
           - name: bootstrap
             serviceName: bootstrap
             componentSelector: broker
         spec:
             type: ClusterIP
             ports:
             - name: kafka-client
               targetPort: 9092
               port: 9092
     componentSpecs:
     - name: broker
       componentDef: kafka-broker
       tls: false
       replicas: 1
       serviceAccountName: kb-kafka-cluster
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
       - name: metadata
         spec:
           storageClassName: null
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 5Gi
     - name: controller
       componentDefRef: controller
       componentDef: kafka-controller
       tls: false
       replicas: 1
       serviceAccountName: kb-kafka-cluster
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
       volumeClaimTemplates:
       - name: metadata
         spec:
           storageClassName: null
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
     - name: metrics-exp
       componentDefRef: kafka-exporter
       componentDef: kafka-exporter
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
     EOF
     ```

   | 字段                                   | 定义  |
   |---------------------------------------|--------------------------------------|
   | `metadata.annotations."kubeblocks.io/extra-env"` | 定义了 Kafka broker 的 jvm heap 配置。 |
   | `metadata.annotations.kubeblocks.io/enabled-pod-ordinal-svc` | 为 nodeport 特性门控定义了 Kafka 集群注释键。您还可以设置 `kubeblocks.io/enabled-node-port-svc: broker` 和 `kubeblocks.io/disabled-cluster-ip-svc: broker`。 |
   | `spec.clusterDefinitionRef`           | 集群定义 CRD 的名称，用来定义集群组件。  |
   | `spec.clusterVersionRef`              | 集群版本 CRD 的名称，用来定义集群版本。 |
   | `spec.terminationPolicy`              | 集群的终止策略，默认值为 `Delete`，有效值为 `DoNotTerminate`、`Halt`、`Delete` 和 `WipeOut`。具体定义可参考 [终止策略](./delete-kafka-cluster.md#终止策略)。 |
   | `spec.affinity`                       | 为集群的 Pods 定义了一组节点亲和性调度规则。该字段可控制 Pods 在集群中节点上的分布。 |
   | `spec.affinity.podAntiAffinity`       | 定义了不在同一 component 中的 Pods 的反亲和性水平。该字段决定了 Pods 以何种方式跨节点分布，以提升可用性和性能。 |
   | `spec.affinity.topologyKeys`          | 用于定义 Pod 反亲和性和 Pod 分布约束的拓扑域的节点标签值。 |
   | `spec.tolerations`                    | 该字段为数组，用于定义集群中 Pods 的容忍，确保 Pod 可被调度到具有匹配污点的节点上。 |
   | `spec.services`                       | 定义了访问集群的服务。 |
   | `spec.componentSpecs`                 | 集群 components 列表，定义了集群 components。该字段允许对集群中的每个 component 进行自定义配置。 |
   | `spec.componentSpecs.componentDefRef` | 表示 cluster definition 中定义的 component definition 的名称，可通过执行 `kubectl get clusterdefinition kafka -o json \| jq '.spec.componentDefs[].name'` 命令获取 component definition 名称。 |
   | `spec.componentSpecs.name`            | 定义了 component 的名称。  |
   | `spec.componentSpecs.replicas`        | 定义了 component 中 replicas 的数量。 |
   | `spec.componentSpecs.resources`       | 定义了 component 的资源要求。  |

2. 查看集群是否创建成功。

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
   mycluster   kafka                kafka-3.3.2   Delete               Running   2m2s
   ```

</TabItem>

</Tabs>
