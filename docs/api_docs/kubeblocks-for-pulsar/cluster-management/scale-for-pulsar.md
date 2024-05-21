---
title: Scale for a Pulsar
description: How to scale a Pulsar cluster, horizontal scaling, vertical scaling
keywords: [pulsar, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale for a Pulsar cluster

You can scale a Pulsar cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader, and the leader pod may change after restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list pulsar
```

### Steps

1. Change configuration. There are 2 ways to apply vertical scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>
  
   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl create -f -<< EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vscale
     namespace: demo
   spec:
     clusterRef: pulsar
     type: VerticalScaling
     verticalScaling:
     - componentName: broker
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3
     - componentName: bookies
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3      
   EOF
   ```
  
   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restoration of data.

### Before you start

- It is recommended to keep 3 nodes without scaling for Zookeeper, and other components can scale horizontally for multiple or single components
- The scaling of the Bookies node needs to be cautious. The data copy is related to the EnsembleSize, Write Quorum, and Ack Quorum configurations, scaling may cause data loss. Check [Pulsar official document](https://pulsar.apahe.org/docs/3.0.x/administration-zk-bk/#decommission-bookies-cleanly) for detailed information.

### Steps

1. Change configuration. There are 2 ways to apply horizontal scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

    ```bash
    kubectl create -f -<< EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-horizontalscaling
      namespace: demo
    spec:
      clusterRef: pulsar
      type: HorizontalScaling  
      horizontalScaling:
      - componentName: broker
        replicas: 5
      - componentName: bookies
        replicas: 5
    EOF
    ```

   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   ```bash
   kubectl edit cluster pulsar
   ```

   </TabItem>

   </Tabs>
  
2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kubectl get ops
   >
   NAME                             TYPE               CLUSTER   STATUS    PROGRESS   AGE
   pulsar-horizontalscaling-9lfvc   HorizontalScaling  pulsar    Succeed   3/3        8m49s
   ```

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mycluster-pulsar-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=mysql-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mysql-cluster
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
