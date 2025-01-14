---
title: Horizontal Scale
description: How to scale a cluster, scale replicas, scale instances
keywords: [horizontal scaling, Horizontal Scale]
sidebar_position: 2
sidebar_label: Horizontal Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Horizontal Scale

From v0.9.0, the horizontal scale provided by KubeBlocks supports ScaleIn and ScaleOut operations and supports scaling both replicas and instances.

- ScaleIn: It supports scaling in the specified replicas and offloading specified instances.
- ScaleOut: It supports scaling out the specified replicas and makes the offline instances online again.

You can perform the horizontal scale by modifying the cluster in a declarative API style or creating an OpsRequest:

- Modifying the Cluster in a declarative API style

    With the declarative API style, users can directly modify the Cluster YAML file to specify the number of replicas for each component and instance template. If the new number of replicas is greater than the current number of Pods, it indicates a scale-out; conversely, if the new number of replicas is less than the current number of Pods, it indicates a scale-in.

- Creating an OpsRequest

    Another approach is to specify the replica count increment in the OpsRequest. The controller will calculate the desired number of replicas based on the current number of Pods in the Cluster's components and the increment value, and perform scaling accordingly.

:::note

- In cases of concurrent modifications, such as multiple controllers concurrently modifying the number of Pods, the calculated number of Pods might be inaccurate. You can ensure the order is on the client side or set KUBEBLOCKS_RECONCILE_WORKERS=1.
- If there is an ongoing scaling operation using the declarative API, this operation will be terminated.
- From v0.9.0, for MySQL and PostgreSQL, after horizontal scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature, which simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md).

:::

## Why do you need to scale for specified instances

Before v0.9.0, KubeBlocks generated workloads as *StatefulSets*, which was a double-edged sword. While KubeBlocks could leverage the advantages of a *StatefulSets* to manage stateful applications like databases, it inherited its limitations.

One of these limitations is evident in horizontal scaling scenarios, where *StatefulSets* offload Pods sequentially based on *Ordinal* order, potentially impacting the availability of databases running within.

Another issue arises in the same scenario: if the node hosting Pods experiences a hardware failure, causing disk damage and rendering data read-write inaccessible, according to operational best practices, we need to offload the damaged Pod and rebuild replicas on healthy nodes. However, performing such operational tasks based on *StatefulSets* isn't easy. [Similar discussions](https://github.com/kubernetes/kubernetes/issues/83224) can be observed in the Kubernetes community.

To solve the limitations mentioned above, starting from version 0.9, KubeBlocks replaces *StatefulSets* with *InstanceSet* which is a general workload API and is in charge of a set of instances. With *InstanceSet*, KubeBlocks introduces the *specified instance scaling* feature to improve the availability.

## Before you start

This tutorial takes Redis for illustration and here is the original component topology.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis
  namespace: kubeblocks-cloud-ns
spec:
  componentSpecs:
  - name: proxy
    componentDef: redis-proxy
    replicas: 10
    instances:
    - name: proxy-2c
      replicas: 3
      resources:
        limits:
          cpu: 2
          memory: 4Gi
    resources:
      limits:
        cpu: 4
        memory: 8Gi
    offlineInstances:
    - redis-proxy-proxy-2c4g-0
    - redis-proxy-proxy-2c4g-1     
```

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster redis
```

## ScaleIn

### Example 1

This example illustrates how to apply an OpsRequest to scale in replicas to 8. This example deletes pods according to the default rules without specifying any instances.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 2  
EOF 
```

### Example 2

This example illustrates how to apply an OpsRequest to scale in replicas to 8 and the specifications of the deleted instances should be 2C4G.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 2
      instances: 
      - name: proxy-2c4g
        replicaChanges: 2
EOF
```

### Example 3

This example illustrates how to scale in the specified instance. In this example, both `replicas` and `instance replicas` will decrease 1 in amount.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      onlineInstancesToOffline:
      - redis-proxy-proxy-2c4g-2
  ttlSecondsAfterSucceed: 0
EOF
```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

You can also edit the cluster YAML file to scale in a specified instance.

```yaml
kubectl edit cluster redis
```

Edit the value of `replicas`.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis
spec:
  componentSpecs:
  - name: proxy
    replicas: 9 # Change this value
    offlineInstances: ["redis-proxy-proxy-2c4g-2"]
...
```

</TabItem>
</Tabs>

## ScaleOut

The following examples illustrate scaling out both replicas and instances. If you only need to scale out replicas, just edit `replicaChanges`. For example,

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut: 
      replicaChanges: 6
EOF
```

### Example 1

This example illustrates how to scale out replicas to 16 by applying an OpsRequest.

The created instances are as follows:

Three replicas equipped with 4C8G: `redis-proxy-7`, `redis-proxy-8`, `redis-proxy-9`
Three replicas equipped with 2C4G: `redis-proxy-2c4g-5`, `redis-proxy-2c4g-6`, `redis-proxy-2c4g-7`

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut: 
      replicaChanges: 6
      instances: 
      - name: proxy-2c4g
        replicasChanges: 3
EOF
```

### Example 2

This example illustrates how to apply an OpsRequest to scale out replicas to 16 and three of the new replicas are proxies equipped with 8C16G.

The created instances are as follows:

4C8G: `redis-proxy-7`, `redis-proxy-8`, `redis-proxy-9`
8C16G: `redis-proxy-8c16g-0`, `redis-proxy-8c16g-1`, `redis-proxy-8c16g-2`

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut:
      replicaChanges: 6 
      newInstances:
      - name: proxy-8c16g
        replicas: 3
        resources:
          limits:
            cpu: 8
            memory: 16Gi  
EOF
```

### Example 3

This example illustrates how to apply an OpsRequest to add the offline instances to the cluster. After the operation, there will be 12 replicas.

The added pods in this example are `redis-proxy-2c4g-0` and `redis-proxy-2c4g-1`.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleOut:
      offlineInstancesToOnline:
      - redis-proxy-proxy-2c4g-0
      - redis-proxy-proxy-2c4g-1     
EOF   
```

## ScaleIn and ScaleOut

### Example 1

This example illustrates how to apply an OpsRequest to scale in a specified instance and create a new instance.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      onlineInstancesToOffline:
      - redis-proxy-proxy-2c4g-2  
    scaleOut:
      instances:
      - name: 2c4g
        replicaChanges: 1
EOF
```

### Example 2

This example illustrates how to apply an OpsRequest to scale in replicas to 8 and add the offline instances to the cluster.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: ops-horizontalscaling
spec:
  type: HorizontalScaling
  clusterName: redis
  horizontalScaling:
  - componentName: proxy
    scaleIn:
      replicaChanges: 4
      instances:
      - name: 2c4g
        replicaChanges: 2
    scaleOut: 
      offlineInstancesToOnline:
      - redis-proxy-proxy-2c4g-0
      - redis-proxy-proxy-2c4g-1
EOF
```

## Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/reids-redis-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***Reason***

This exception occurs because the `VolumeSnapshotClass` is not configured. This exception can be fixed after configuring `VolumeSnapshotClass`, but the horizontal scaling cannot continue to run. It is because the wrong backup (volumesnapshot is generated by backup) and volumesnapshot generated before still exist. First, delete these two wrong resources and then KubeBlocks re-generates new resources.

***Steps:***

1. Configure the VolumeSnapshotClass by running the command below.

   ```bash
   kubectl create -f - <<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotClass
   metadata:
     name: csi-aws-vsc
     annotations:
       snapshot.storage.kubernetes.io/is-default-class: "true"
   driver: ebs.csi.aws.com
   deletionPolicy: Delete
   EOF
   ```

2. Delete the wrong backup (volumesnapshot is generated by backup) and volumesnapshot resources.

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=redis
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=redis
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
