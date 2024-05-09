---
title: Scale for a Kafka cluster
description: How to scale a Kafka cluster, horizontal scaling, vertical scaling
keywords: [kafka, horizontal scaling, vertical scaling]
sidebar_position: 3
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale for a Kafka cluster

You can scale a Kafka cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo  
```

### Steps

1. Change configuration. There are 2 ways to apply vertical scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>
  
   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
   spec:
     clusterRef: ivy85
     type: VerticalScaling 
     verticalScaling:
     - componentName: broker
       requests:
         memory: "2Gi"
         cpu: "1000m"
       limits:
         memory: "4Gi"
         cpu: "2000m"
   EOF
   ```
  
   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ***Example***

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: ivy85
     namespace: default
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2
     componentSpecs:
     - name: broker
       componentDefRef: broker
       replicas: 1
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Halt
   ```

   </TabItem>

   </Tabs>
  
2. Validate the volume expansion.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  kafka
    Enabled Logs:
      running
    Monitor:   false
    Name:      kafka
    Replicas:  2
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restoration of data.

### Before you start

- Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.
- You are not recommended to perform horizontal scaling on the controller node, including the controller node both in combined mode and separated node.
- When scaling in horizontally, you must know the topic partition storage. If the topic has only one replication, data loss may caused when you scale in broker.

  ```bash
  kubectl get cluster mycluster -n demo  
  ```

### Steps

1. Change configuration. There are 3 ways to apply horizontal scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
   spec:
     clusterRef: ivy85
     type: HorizontalScaling
     horizontalScaling:
     - componentName: broker
       replicas: 3
   EOF
   ```

   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ***Example***

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
    apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: ivy85
     namespace: default
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2 
     componentSpecs:
     - name: broker
       componentDefRef: broker
       replicas: 1 # Change the pod amount.
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
    terminationPolicy: Halt
   ```

   </TabItem>

   </Tabs>

2. Validate the horizontal scaling operation.

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/ivy85-kafka-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***Reason***

This exception occurs because the `VolumeSnapshotClass` is not configured. This exception can be fixed after configuring `VolumeSnapshotClass`, but the horizontal scaling cannot continue to run. It is because the wrong backup (volumesnapshot is generated by backup) and volumesnapshot generated before still exist. Delete these two wrong resources and then KubeBlocks re-generates new resources.

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
   kubectl delete backup -l app.kubernetes.io/instance=ivy85
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=ivy85

   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
