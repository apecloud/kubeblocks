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

* [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
* 安装 KubeBlocks：你可以使用 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](../../installation/install-with-helm/install-kubeblocks-with-helm.md) 进行安装。
* 确保 `kbcli addon list` 已启用。

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  kafka                        Helm   Enabled                   true
  ...
  ```

:::note

- KubeBlocks 集成了 Kafka v3.3.2，以 KRaft 模式运行。
- 在生产环境中，不建议以组合模式使用 KRaft 集群。
- 建议将控制器数量设置在 3 到 5 个之间，实现复杂性和可用性的平衡。

:::
## 创建 Kafka 集群

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

使用 `kbcli cluster create` 命令创建集群。你还可以使用 `--set` 参数自定义集群资源。

```bash
kbcli cluster create kafka
```

下表详细描述了各类自定义参数。请务必设置 `--termination-policy`。此外，强烈建议你打开监视器并启用所有日志。

📎 Table 1. kbcli cluster create 选项详情

|    选项                                                                 | 解释                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|---------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --mode='combined'                                                         | 表示 Kafka kraft 集群的模式。'combined' 表示使用组合的代理和控制器节点，'separated' 表示独立运行代理和控制器。有效值为 [combined, separated]。                                                                                                                                                                                                                                                                   |
| --replicas=1                                                              | 表示组合模式下的 Kafka 代理的副本数。在组合模式下，此值还表示 kraft 控制器的数量。有效值为 [1,3,5]。                                                                                                                                                                                                                                                           |
| --broker-replicas=1                                                       | 表示分离模式下的 Kafka 代理的副本数。                                                                                                                                                                                                                                                                                                                                                                                           |
| --controller-replicas=1                                                   | 表示分离模式下的 Kafka 控制器的副本数。在分离模式下，此数字表示 kraft 控制器的数量。有效值为 [1,3,5]。                                                                                                                                                                                                                                                                                  |
| --termination-policy='Delete'                                             | 表示集群的终止策略。有效值为 [DoNotTerminate, Halt, Delete, WipeOut]。 <br /> DoNotTerminate：DoNotTerminate 禁止删除操作。 <br /> Halt：Halt 删除工作负载资源（如 statefulset、deployment 等），但保留 PVC。 <br /> Delete：Delete 在 Halt 的基础上删除了 PVC。 <br /> WipeOut：WipeOut 在 Delete 的基础上删除了备份存储位置中的所有卷快照和快照数据。 |
| --storage-enable=false                                                    | 表示是否启用 Kafka 的存储功能。                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --host-network-accessible=false                                           | 指定集群是否可以从 VPC 内部访问。                                                                                                                                                                                                                                                                                                                                                                                  |
| --publicly-accessible=false                                               | 指定集群是否可以从公共互联网访问。                                                                                                                                                                                                                                                                                                                                                                             |
| --broker-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64'     | 表示 Kafka 代理的 JVM 堆设置。                                                                                                                                                                                                                                                                                                                                                                                                                  |
| --controller-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64' | 表示分离模式下 Kafka 控制器的 JVM 堆设置。仅在 mode='separated' 时生效。                                                                                                                                                                                                                                                                                                                                     |
| --cpu=1                                                                   | 表示 CPU 内核数。                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --memory=1                                                                | 表示内存，单位为 Gi。                                                                                                                                                                                                                                                                                                                                                                                                                          |
| --storage=20                                                              | 表示数据存储大小，单位为 Gi。                                                                                                                                                                                                                                                                                                                                                                                                               |
| --storage-class=''                                                        | 表示 Kafka 数据存储的 StorageClass。                                                                                                                                                                                                                                                                                                                                                                                                          |
| --meta-storage=5                                                          | 表示元数据存储大小，单位为 Gi。                                                                                                                                                                                                                                                                                                                                                                                                           |
| --meta-storage-class=''                                                   | 表示 Kafka 元数据存储的 StorageClass。                                                                                                                                                                                                                                                                                                                                                                                                      |
| --monitor-enable=false                                                    | 表示是否启用 Kafka 的监视器。                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --monitor-replicas=1                                                      | 表示 Kafka 监视器的副本数。                                                                                                                                                                                                                                                                                                                                                                                                            |
| --sasl-enable=false                                                       | 表示是否启用 SASL/PLAIN 进行 Kafka 身份验证。 <br /> -server: admin/kubeblocks <br /> -client: client/kubeblocks  <br /> 内置的 jaas 文件存储在 /tools/client-ssl.properties 中。                                                                                                                                                                                                                                                              |
</TabItem>

<TabItem value="kubectl" label="kubectl" default>

* 创建组合模式的 Kafka 集群。

    ```bash
    # 创建组合模式的 Kafka 集群  
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: kafka-combined
      namespace: default
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: kafka-server
        monitor: false
        name: broker
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
      terminationPolicy: Delete
    EOF
    ```

* 创建分离模式的 Kafka 集群。

    ```bash
    # 创建分离模式的 Kafka 集群 
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: kafka-separated
      namespace: default
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: controller
        monitor: false
        name: controller
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
        tls: false
      - componentDefRef: kafka-broker
        monitor: false
        name: broker
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
        tls: false
      terminationPolicy: Delete
    EOF
    ```

</TabItem>

</Tabs>
