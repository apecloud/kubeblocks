---
title: Scale for a Redis cluster
description: How to scale a Redis cluster, horizontal scaling, vertical scaling
keywords: [redis, horizontal scaling, vertical scaling, scale]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for a Redis cluster

You can scale Redis DB instances in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

### Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list redis-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS         CREATED-TIME
redis-cluster        default          redis                     redis-7.0.6              Delete                    Running        Apr 10,2023 16:21 UTC+0800
```

### Steps

1. Change configuration. 

   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
   kbcli cluster vscale redis-cluster \
   --components="redis" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

2. Validate the vertical scaling.

   Check the cluster status to identify the vertical scaling status.

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    Updating        Apr 10,2023 16:27 UTC+0800
   ```

   - STATUS=Updating: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal.
     > To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.

    :::note

    Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the opsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

    :::

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe redis-cluster
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list redis-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS         CREATED-TIME
redis-cluster        default          redis                     redis-7.0.6              Delete                    Running        Apr 10,2023 16:50 UTC+0800
```

### Steps

1. Change configuration.

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale redis-cluster \
   --components="redis" --replicas=2
   ```

   - `--components` describes the component name ready for horizontal scaling.
   - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    Updating        Apr 10,2023 16:58 UTC+0800
   ```

   - STATUS=Updating: it means horizontal scaling is in progress.
   - STATUS=Running: it means horizontal scaling has been applied.

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-10T18:20:26Z"
    message: VolumeSnapshot/redis-cluster-redis-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=redis-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=redis-cluster
   ```

***Result***

The horizontal scaling continues after the backup and volumesnapshot are deleted and the cluster restores to running status.
