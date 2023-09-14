---
title: Scale for a Pulsar
description: How to scale a Pulsar cluster, horizontal scaling, vertical scaling
keywords: [mysql, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for a Pulsar cluster

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

1. Change configuration. There are 3 ways to apply vertical scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
   kbcli cluster vscale pulsar --cpu=3 --memory=10Gi --components=broker,bookies  
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   **Option 2.** Create an OpsRequest
  
   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl create -f -<< EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     generateName: pulsar-vscale-
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
  
   **Option 3.** Edit Pulsar cluster with `kubectl`.

   ```bash
   kubectl edit cluster pulsar
   ```

2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list pulsar
    ```

   - STATUS=VerticalScaling: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
     > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the opsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe pulsar
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restoration of data.

### Before you start

- It is recommended to keep 3 nodes without scaling for Zookeeper, and other components can scale horizontally for multiple or single components
- The scaling of the Bookies node needs to be cautious. The data copy is related to the EnsembleSize, Write Quorum, and Ack Quorum configurations, scaling may cause data loss. Check [Pulsar official document](https://pulsar.apahe.org/docs/3.0.x/administration-zk-bk/#decommission-bookies-cleanly) for detailed information.

### Steps

1. Change configuration. There are 3 ways to apply horizontal scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale pulsar --replicas=5 --components=broker,bookies                  Running        Jan 29,2023 14:29 UTC+0800
   ```

   - `--components` describes the component name ready for horizontal scaling.
   - `--replicas` describes the replicas with the specified components.

   **Option 2.** Create an OpsRequest

   Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

    ```bash
    kubectl create -f -<< EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      generateName: pulsar-horizontalscaling-
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

   **Option 3.** Edit cluster with `kubectl`.

   ```bash
   kubectl edit cluster pulsar
   ```
  
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
   kbcli cluster describe mysql-cluster
   ```

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mysql-cluster-mysql-scaling-dbqgp: Failed to set default snapshot
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
