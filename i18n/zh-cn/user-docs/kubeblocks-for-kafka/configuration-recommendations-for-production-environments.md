---
title: 资源描述
description: Kafka 资源描述
keywords: [kafka, Java 堆, 硬件资源]
sidebar_position: 2
sidebar_label: 资源描述
---

# 资源描述

## Java Heap

指 Kafka 服务器的 JVM 堆配置。在生产环境中，请参考[官方推荐配置](https://kafka.apache.org/33/documentation.html#java)：

```bash
-Xmx6g -Xms6g -XX:MetaspaceSize=96m -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:G1HeapRegionSize=16M -XX:MinMetaspaceFreeRatio=50 -XX:MaxMetaspaceFreeRatio=80 -XX:+ExplicitGCInvokesConcurrent
```

- 组合模式
    在创建 Kafka 集群时，使用 `--broker-heap` 指定堆配置。
- 分离模式
    在创建Kafka集群时，使用 `--broker-heap` 指定组件的堆配置；使用 `--controller-heap` 指定控制器的堆配置。

:::note

在修改 Java 堆配置时，请特别注意分配给集群的资源。例如，如果设置了 `--memory=1Gi`，但在 `--broker-heap` 中指定了 `-Xms` 为 `6g`，那么就可能由于内存不足导致 broker 服务无法正常启动。

:::

## 硬件资源

建议在生产环境中使用以下硬件资源：

- `-cpu` >= 16 核
- `-memory` >= 64 Gi

由于 Kafka 使用大量的页面缓存来提高读写速度，因此分配在堆内存之外的内存资源可以提高整体性能。

- `-storage` 需根据具体情况进行配置。

Kafka 集群的默认压缩算法是 `compression.type=producer`，由生产者端指定。在设置前，你可以参考主题的平均消息大小、压缩比率、主题副本数量和数据保留时间进行综合评估。