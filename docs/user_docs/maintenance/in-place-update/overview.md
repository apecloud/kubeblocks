# Overview on in-place update

In its earlier versions, KubeBlocks ultimately generated Workloads as StatefulSets. For statefulsets, any change in the segment of PodTemplate may result in the update of all pods, and  the method of update is called `Recreate`, that is deleting all current pods and create a new one. This is obviously not the best practice for database management, which has a high requirement on system availability.
To address this issue, KubeBlocks introduced the instance in-place update feature starting from version 0.9, reducing the impact on system availability during instance updates.

## Fields of an instance support in-place updates?
In principle, KubeBlocks instance in-place updates leverage the Kubernetes Pod API's in-place update capability. Therefore, the specific supported fields are as follows:

`annotations`
`labels`
`spec.activeDeadlineSeconds`
`spec.initContainers[*].image`
`spec.containers[*].image`
`spec.tolerations (only supports adding Toleration)`

Starting from Kubernetes version 1.27, support for in-place updates of CPU and Memory can be further enabled through the PodInPlaceVerticalScaling feature switch, which is enabled by default from version 1.29 onwards. KubeBlocks automatically detects the Kubernetes version and feature switches, and further supports the following capabilities:
For Kubernetes versions equal to or greater than 1.29, or greater than or equal to 1.27 and less than 1.29 with PodInPlaceVerticalScaling enabled, the following fields' in-place updates are supported:

`spec.containers[*].resources.requests["cpu"]`
`spec.containers[*].resources.requests["memory"]`
`spec.containers[*].resources.limits["cpu"]`
`spec.containers[*].resources.limits["memory"]`

It is important to note that after successful resource resizing, some applications may need to be restarted to recognize the new resource configuration. In such cases, further configuration of container restartPolicy is required in ClusterDefinition or ComponentDefinition.

For PVC, KubeBlocks similarly leverages the capabilities of the PVC API, supporting only volume expansion.

## From the upper-level API perspective, which fields utilize in-place updates after being updated?

KubeBlocks upper-level APIs related to instances include Cluster, ClusterDefinition, ClusterVersion, ComponentDefinition, and ComponentVersion. Within these APIs, several fields will ultimately be directly or indirectly used to render instance objects, potentially triggering in-place updates for instances.

There are numerous fields across these APIs. See below table for brief descriptions. :::Note 
Fields marked as deprecated or immutable in the API are not included in the list).
:::

|         API         |                                                                                                                                                                                                                                                                                                                                                                            Fields                                                                                                                                                                                                                                                                                                                                                                           |                                                    Description                                                    |
|:-------------------:|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|:-----------------------------------------------------------------------------------------------------------------:|
|       Cluster       | annotations, <br>labels, <br>spec.tolerations, <br>spec.componentSpecs[*].serviceVersion, <br>spec.componentSpecs[*].tolerations, <br>spec.componentSpecs[*].resources, <br>spec.componentSpecs[*].volumeClaimTemplates, <br>spec.componentSpecs[*].instances[*].annotations, <br>spec.componentSpecs[*].instances[*].labels, <br>spec.componentSpecs[*].instances[*].image, <br>spec.componentSpecs[*].instances[*].tolerations, <br>spec.componentSpecs[*].instances[*].resources, <br>spec.componentSpecs[*].instances[*].volumeClaimTemplates, <br>spec.shardingSpecs[*].template.serviceVersion, <br>spec.shardingSpecs[*].template.tolerations, <br>spec.shardingSpecs[*].template.resources, <br>spec.shardingSpecs[*].template.volumeClaimTemplates | Resources related fields means: <br>requests["cpu"],<br>requests["memory"],<br>limits["cpu"],<br>limits["memory"] |
|   ComponentVersion  |                                                                                                                                                                                                                                                                                                                                                                   spec.releases[*].images                                                                                                                                                                                                                                                                                                                                                                   |            Whether in-place update is triggered depends on whether the corresponding image is changed.            |
| KubeBlocks Built-in |                                                                                                                                                                                                                                                                                                                                                                     annotations, labels                                                                                                                                                                                                                                                                                                                                                                     |                                                                                                                   |