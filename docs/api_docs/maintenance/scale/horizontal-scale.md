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

From v0.9.0, the horizontal scale can be divided into two types to better support various scenarios. Both final-state-oriented and procedure-oriented scale support scale in and out, but what distinguishes them is that the final-state-oriented scale only scales replicas and the procedure-oriented one supports scaling both the replicas and instances.

- Final-state-oriented

    Final-state-oriented horizontal scale refers to the operation in which components and instances specify replicas. This type of horizontal scale is performed by simply specifying the replica amount and overwriting the replicas. However, this method does not support specifying an instance to scale in or out.

    It is also recommended to edit the cluster YAML file to perform a horizontal scale since its corresponding OpsRequest option might be deprecated later.

- Procedure-oriented

    The procedure-oriented horizontal scale is designed to add or delete specified instances to meet the frequent scaling in and out demands.

    For the procedure-oriented horizontal scale, an operation will calculate the horizontal scale state based on the status of the pods involved in the OpsRequest operation. In extreme cases, due to the non-atomic nature of the operation, the recorded lastCompReplicas may be inaccurate, leading to incorrect pod count calculations (you can ensure the order on the client side or set `KUBEBLOCKS_RECONCILE_WORKERS=1`).

     1. If the issued OpsRequest attempts to delete an instance created by a running OpsRequest, it will not be allowed and will fail directly.
     2. If there is a final-state-oriented operation in execution, this operation will be terminated.

:::note

From v0.9.0, for MySQL and PostgreSQL, after horizontal scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature, which simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md).

:::

## Scale replicas

Horizontal scaling changes the amount of replicas. For example, you can scale replicas up from three to five. The scaling process includes the backup and restore of data.

Here we take a MySQL cluster as an example.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   mysql                mysql-8.0.33   Delete               Running   4d19h
```

### Steps

There are two ways to apply horizontal scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mysql
       replicas: 1
   EOF
   ```

2. Check the operation status to validate the horizontal scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the horizontal scaling operation, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>
  
<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the value of `spec.componentSpecs.replicas` in the YAML file.

   `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 1 # Change the amount
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
    terminationPolicy: Delete
   ```

2. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>

## Scale instances

Here we take a Redis Proxy cluster as an example to illustrate how to scale in or out a cluster for different scenarios.

The original topology of this cluster is as follows:

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

### Scaleout

#### Example 1

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

#### Example 2

This example illustrates how to apply an OpsRequest to scale out replicas to 16 and three of the new replicas are proxies equipped with 8C16G.

The created instances are as follows:

4C8G: `redis-proxy-7`, `redis-proxy-8`, `redis-proxy-9`
2C4G: `redis-proxy-8c16g-0`, `redis-proxy-8c16g-1`, `redis-proxy-8c16g-2`

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

#### Example 3

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

### Scalein

#### Why do you need to scale for specified instances

Before v0.9.0, KubeBlocks generated workloads as *StatefulSets*, which was a double-edged sword. While KubeBlocks could leverage the advantages of a *StatefulSets* to manage stateful applications like databases, it inherited its limitations.

One of these limitations is evident in horizontal scaling scenarios, where *StatefulSets* offload Pods sequentially based on *Ordinal* order, potentially impacting the availability of databases running within.

Another issue arises in the same scenario: if the node hosting Pods experiences a hardware failure, causing disk damage and rendering data read-write inaccessible, according to operational best practices, we need to offload the damaged Pod and rebuild replicas on healthy nodes. However, performing such operational tasks based on *StatefulSets* isn't easy. [Similar discussions](https://github.com/kubernetes/kubernetes/issues/83224) can be observed in the Kubernetes community.

To solve the limitations mentioned above, starting from version 0.9, KubeBlocks KubeBlocks replaces *StatefulStes* with *InstanceSet* which is a general workload API and is in charge of a set of instances. With *InstanceSet*, KubeBlocks also introduces the *specified instance scaling* feature to improve the availability.

#### Example 1

This example illustrates how to apply an OpsRequest to scale in replicas to 8 and the deleted pods are instances equipped with 4C8G.

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

#### Example 2

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

#### Example 3

This example illustrates how to scale in the specified instance. In this scenario, both `replicas` and `instance replicas` will decrease 1 in amount.

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
>
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis
spec:
  componentSpecs:
  - name: proxy
    replicas: 9
    offlineInstances: ["redis-proxy-proxy-2c4g-2"]
# ...
```

</TabItem>
</Tabs>

### Scalein and scaleout

#### Example 1

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

#### Example 2

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
    message: VolumeSnapshot/mycluster-mysql-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=mycluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
