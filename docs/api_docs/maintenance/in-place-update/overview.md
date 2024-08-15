---
title: Overview on in-place update
description: Overview on in-place update
keywords: [in-place update, overview]
sidebar_position: 1
sidebar_label: Overview on in-place update
---

# Overview on in-place update

In its earlier versions, KubeBlocks ultimately generated Workloads as StatefulSets. For StatefulSets, any change in the segment of PodTemplate may result in the update of all pods, and the method of update is called `Recreate`, that deletes all current pods and creates a new one. This is obviously not the best practice for database management, which has a high requirement on system availability.
To address this issue, KubeBlocks introduced the instance in-place update feature starting from version 0.9, reducing the impact on system availability during instance updates.

## Which fields of an instance support in-place updates?

In principle, KubeBlocks instance in-place updates leverage [the Kubernetes Pod API's in-place update capability](https://kubernetes.io/docs/concepts/workloads/pods/#pod-update-and-replacement). Therefore, the specific supported fields are as follows:

* `annotations`
* `labels`
* `spec.activeDeadlineSeconds`
* `spec.initContainers[*].image`
* `spec.containers[*].image`
* `spec.tolerations (only supports adding Toleration)`

Starting from Kubernetes version 1.27, support for in-place updates of CPU and Memory can be further enabled through the `InPlacePodVerticalScaling` feature switch. KubeBlocks also supports the `InPlacePodVerticalScaling` feature switc which further supports the following capabilities:

For Kubernetes versions equal to or greater than 1.27 with InPlacePodVerticalScaling enabled, the following fields' in-place updates are supported:

* `spec.containers[*].resources.requests["cpu"]`
* `spec.containers[*].resources.requests["memory"]`
* `spec.containers[*].resources.limits["cpu"]`
* `spec.containers[*].resources.limits["memory"]`

It is important to note that after successful resource resizing, some applications may need to be restarted to recognize the new resource configuration. In such cases, further configuration of container `restartPolicy` is required in ClusterDefinition or ComponentDefinition.

For PVC, KubeBlocks also leverages the capabilities of the PVC API and only supports volume expansion. If the expansion fails for some reason, it supports reverting to the original capacity. However, once a VolumeClaimTemplate in a StatefulSet is declared, it cannot be modified. Currently, the Kuberenetes community is [developing this capability](https://github.com/kubernetes/enhancements/pull/4651), but it won't be available until at least Kubernetes version 1.32.

## From the upper-level API perspective, which fields utilize in-place updates after being updated?

KubeBlocks upper-level APIs related to instances include Cluster, ClusterDefinition, ClusterVersion, ComponentDefinition, and ComponentVersion. Within these APIs, several fields will ultimately be directly or indirectly used to render instance objects, potentially triggering in-place updates for instances.

There are numerous fields across these APIs. See below table for brief descriptions.

:::note

Fields marked as deprecated or immutable in the API are not included in the list.

:::

| API |   Fields    |   Description  |
|:-----|:-------|:-----------|
|Cluster| `annotations`, <p>`labels`, </p><p>`spec.tolerations`, </p><p>`spec.componentSpecs[*].serviceVersion`, </p><p>`spec.componentSpecs[*].tolerations`, </p><p>`spec.componentSpecs[*].resources`, </p><p>`spec.componentSpecs[*].volumeClaimTemplates`, </p><p>`spec.componentSpecs[*].instances[*].annotations`, </p><p>`spec.componentSpecs[*].instances[*].labels`, </p><p>`spec.componentSpecs[*].instances[*].image`, </p><p>`spec.componentSpecs[*].instances[*].tolerations`, </p><p>`spec.componentSpecs[*].instances[*].resources`, </p><p>`spec.componentSpecs[*].instances[*].volumeClaimTemplates`, </p><p>`spec.shardingSpecs[*].template.serviceVersion`, </p><p>`spec.shardingSpecs[*].template.tolerations`, </p><p>`spec.shardingSpecs[*].template.resources`, </p><p>`spec.shardingSpecs[*].template.volumeClaimTemplates`</p> | Resources related fields means: <p>`requests["cpu"]`,</p><p>`requests["memory"]`,</p><p>`limits["cpu"]`,</p>`limits["memory"]` |
|   ComponentVersion  | `spec.releases[*].images`   | Whether in-place update is triggered depends on whether the corresponding image is changed.            |
| KubeBlocks Built-in |  `annotations`, `labels` |    |
