# Overview on in-place update

In its earlier versions, KubeBlocks ultimately generated Workloads as StatefulSets. For statefulsets, any change in the segment of PodTemplate may result in the update of all pods, and  the method of update is called `Recreate`, that is deleting all current pods and create a new one. This is obviously not the best practice for database management, which has a high requirement on system availability.
To address this issue, KubeBlocks introduced the instance in-place update feature starting from version 0.9, reducing the impact on system availability during instance updates.
