---
title: 概述
description: 原地更新概述
keywords: [原地更新, 概述]
sidebar_position: 1
sidebar_label: 概述
---

# 概述

早期的版本中，KubeBlocks 最终生成的 Workload 是 StatefulSet。StatefulSet 通过 PodTemplate 渲染最终的 Pod，当 PodTemplate 中有任何字段发生了改变，都会导致所有 Pod 被更新，而 StatefulSet 采用的更新方式是 `Recreate`，即首先删除现有的 Pod，然后再新建一个 Pod。对于数据库等对可用性要求很高的系统，这样的方式显然不是最佳选择。

为了解决这个问题，KubeBlocks 从 v0.9 开始，新增实例原地更新特性，降低实例更新时对系统可用性的影响。

## 实例（Instance）哪些字段支持原地更新

从原理上来讲，KubeBlocks 实例原地更新复用了 [Kubernetes Pod API 原地更新能力](https://kubernetes.io/docs/concepts/workloads/pods/#pod-update-and-replacement)。所以具体支持的字段如下：

* `annotations`
* `labels`
* `spec.activeDeadlineSeconds`
* `spec.initContainers[*].image`
* `spec.containers[*].image`
* `spec.tolerations (only supports adding Toleration)`

Kubernetes 从 1.27 版本开始，通过 `InPlacePodVerticalScaling` 特性开关可进一步开启对 CPU 和 Memory 的原地更新支持。KubeBlocks 同样增加了 `InPlacePodVerticalScaling` 特性开关，以便进一步支持如下能力：

对于大于等于 1.27 且 InPlacePodVerticalScaling 已开启的 Kubernetes，支持如下字段的原地更新：

* `spec.containers[*].resources.requests["cpu"]`
* `spec.containers[*].resources.requests["memory"]`
* `spec.containers[*].resources.limits["cpu"]`
* `spec.containers[*].resources.limits["memory"]`

需要注意的是，当 resource resize 成功后，有的应用可能需要重启才能感知到新的资源配置，此时需要在 ClusterDefinition 或 ComponentDefinition 中进一步配置 container `restartPolicy`。

对于 PVC，KubeBlocks 同样复用 PVC API 的能力，仅支持 Volume 的扩容，当因为某些原因扩容失败时，支持缩容回原来的容量值。而 StatefulSet 中的 VolumeClaimTemplate 一经声明，便不能修改，目前官方正在[开发相关能力](https://github.com/kubernetes/enhancements/pull/4651)，但至少需要等到 K8s 1.32 版本了。

## 从上层 API 视角，哪些字段更新后使用的是原地更新

KubeBlocks 跟实例相关的上层 API 包括 Cluster、ClusterDefinition、ClusterVersion、ComponentDefinition 和 ComponentVersion。这些 API 中有若干字段最终会直接或间接用来渲染实例对象，从而可能会触发实例原地更新。

这些字段非常多，这里对这些字段进行罗列和简单描述。

:::note

API 中标记为 deprecated 的字段不在列表内，immutable 的字段不在列表内。

:::

| API |   字段名称    |   描述  |
|:-----|:-------|:-----------|
|Cluster| `annotations`, <p>`labels`, </p><p>`spec.tolerations`, </p><p>`spec.componentSpecs[*].serviceVersion`, </p><p>`spec.componentSpecs[*].tolerations`, </p><p>`spec.componentSpecs[*].resources`, </p><p>`spec.componentSpecs[*].volumeClaimTemplates`, </p><p>`spec.componentSpecs[*].instances[*].annotations`, </p><p>`spec.componentSpecs[*].instances[*].labels`, </p><p>`spec.componentSpecs[*].instances[*].image`, </p><p>`spec.componentSpecs[*].instances[*].tolerations`, </p><p>`spec.componentSpecs[*].instances[*].resources`, </p><p>`spec.componentSpecs[*].instances[*].volumeClaimTemplates`, </p><p>`spec.shardingSpecs[*].template.serviceVersion`, </p><p>`spec.shardingSpecs[*].template.tolerations`, </p><p>`spec.shardingSpecs[*].template.resources`, </p><p>`spec.shardingSpecs[*].template.volumeClaimTemplates`</p> | Resources 相关字段都指的是：<p>`requests["cpu"]`,</p><p>`requests["memory"]`,</p><p>`limits["cpu"]`,</p>`limits["memory"]` |
|   ComponentVersion  | `spec.releases[*].images`   | 是否会触发实例原地更新取决于最终匹配的 Image 是否有变化。           |
| KubeBlocks Built-in |  `annotations`, `labels` |    |
