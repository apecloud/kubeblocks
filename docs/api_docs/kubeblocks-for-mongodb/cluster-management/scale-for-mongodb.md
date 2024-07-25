---
title: Scale for MongoDB cluster
description: How to vertically scale a MongoDB cluster
keywords: [mongodb, vertical scaling, vertically scale a mongodb cluster]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale for a MongoDB cluster

You can scale a MongoDB cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (e.g. CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

:::note

During the vertical scaling process, a restart is triggered and the primary pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
```

### Steps

There are two ways to apply vertical scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mongodb
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.components.resources` in the YAML file. 

   `spec.components.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   ......
   spec:
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - componentDefRef: mongodb
       enabledLogs:
       - running
       disableExporter: true
       name: mongodb
       replicas: 2
       resources:
         limits:
           cpu: "2"
           memory: 4Gi
         requests:
           cpu: "1"
           memory: 2Gi
   ```

2. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

</Tabs>

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY     STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete                 Running   47m
```

### Scale replicas

#### Steps

There are two ways to apply horizontal scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mongo-horizontalscaling
     namespace: default
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mongodb
       replicas: 4
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

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: mongo
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - name: mongo
       componentDefRef: mongo
       replicas: 4 # Change the amount
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

### Scale instances

From v0.9.0, KubeBlocks supports scale in or out of specified instances. For details, refer to [Horizontal Scale](./../../maintenance/scale/horizontal-scale.md#scale-instances).

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2024-04-25T17:40:26Z"
    message: VolumeSnapshot/mycluster-mongodb-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***Reason***

This exception occurs because the `VolumeSnapshotClass` is not configured. This exception can be fixed after configuring `VolumeSnapshotClass`, but the horizontal scaling cannot continue to run. It is because the wrong backup (volumesnapshot is generated by backup) and volumesnapshot generated before still exist. First delete these two wrong resources and then KubeBlocks re-generates new resources.

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
