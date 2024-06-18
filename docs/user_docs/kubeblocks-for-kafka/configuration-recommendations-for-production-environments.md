---
title: Resource description
description: Kafka resource description
keywords: [kafka, java heap, hardware resource]
sidebar_position: 3
sidebar_label: Resource description
---

# Resource Description

## Java Heap

Kafka Server's JVM Heap configuration, production environment can refer to the [official recommended configuration](https://kafka.apache.org/33/documentation.html#java):

```bash
-Xmx6g -Xms6g -XX:MetaspaceSize=96m -XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:G1HeapRegionSize=16M -XX:MinMetaspaceFreeRatio=50 -XX:MaxMetaspaceFreeRatio=80 -XX:+ExplicitGCInvokesConcurrent
```

- Combined mode
    When creating a Kafka Cluster, specify the `--broker-heap` parameter.
- Separated mode
    When creating a Kafka Cluster, specify the component parameters with the `--broker-heap`; specify controller with the `--controller-heap` parameter.

:::note

When modifying the Java Heap configuration, attention should be paid to the resources allocated to the Cluster at the same time. For example, `--memory=1Gi`, but `-Xms` in `--broker-heap` is specified as `6g`, the broker service will not start normally due to insufficient memory.

:::

## Hardware resources

It is recommended to use the following hardware resource in a production environment:

- `-cpu` >= 16 cores
- `-memory` >= 64Gi

Since Kafka uses a large amount of Page Cache to improve read and write speed, memory resources outside the heap memory allocated can improve overall performance.

- `-storage` configure it according to the specific situations

The default compression algorithm for Kafka Cluster is `compression.type=producer`, which is specified by the Producer end. Refer to the average message size of the Topic, compression ratio, number of Topic replicas, and data retention time to perform a comprehensive evaluation.