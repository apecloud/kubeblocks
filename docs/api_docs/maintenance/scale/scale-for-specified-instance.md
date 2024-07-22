
# Scale for specified instances

## Why you need to scale for specified instances

KubeBlocks generated workloads as *StatefulSets*, which was a double-edged sword. While KubeBlocks could leverage advantages of a *StatefulSets* to manage stateful applications like databases, it inherited its limitations.

One of these limitations is evident in horizontal scaling scenarios, where *StatefulSets* offload Pods sequentially based on *Ordinal* order, potentially impacting the availability of databases running within.

For example, managing a PostgreSQL database with one primary and two secondary replicas using a *StatefulSet* named `foo-bar`. Over time, Pod `foo-bar-2` becomes the primary node. Now, if we decide to scale down the database due to low read load, according to *StatefulSet* rules, we can only offload Pod `foo-bar-2`, which is currently the primary node. Therefore we can either directly offload `foo-bar-2`, triggering a failover mechanism to elect a new primary pod from `foo-bar-0` and `foo-bar-1`, or use a switchover mechanism to convert `foo-bar-2` into a secondary pod before offloading it. Either way, there will be a period where write is not applicable.

Another issue arises in the same scenario: if the node hosting `foo-bar-1` experiences a hardware failure, causing disk damage and rendering data read-write inaccessible, according to operational best practices, we need to offload `foo-bar-1` and rebuild replicas on healthy nodes. However, performing such operational tasks based on *StatefulSets* isn't easy.

[Similar discussions](https://github.com/kubernetes/kubernetes/issues/83224) can be observed in the Kubernetes community. Starting from version 0.9, KubeBlocks introduces the *specified instance scaling* feature to address these issues.

## Scale for specified instances

To specify the instance to be offloaded, use `OfflineInstances`.

***Steps：***

Use an OpsRequest to specify the instance to scale.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: foo-horizontalscaling-
spec:
  clusterRef: foo
  force: false
  horizontalScaling:
  - componentName: bar
    replicas: 2
    offlineInstances: ["instancename"]
  ttlSecondsAfterSucceed: 0
  type: HorizontalScaling
```

The OpsRequest Controller directly overrides the values of `replicas` and `offlineInstances` in the request, mapping them to the corresponding fields in the Cluster object. Eventually, the Cluster Controller completes the task of offlining the instance named `foo-bar-1`.

***Example：***

In the scenario of the above section, the PostgreSQL instance status is as follows:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: foo
spec:
  componentSpecs:
  - name: bar
    replicas: 3
# ...
```

When we scale it down to 2 replicas and offload the `foo-bar-1`, we can update it as follows:
```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: foo
spec:
  componentSpecs:
  - name: bar
    replicas: 2
    offlineInstances: ["foo-bar-1"]
# ...
```
Performing the above procedure, KubeBlocks scales down the cluster to 2 replicas and offloads the instance with Ordinal as 1 and not 2. As a result, the remaining instances in the cluster are `foo-bar-0` and `foo-bar-2`.
