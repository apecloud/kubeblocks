---
title: Scale for MongoDB cluster
description: How to vertically scale a MongoDB cluster
keywords: [mongodb, vertical scaling, vertically scale a mongodb cluster]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for a MongoDB cluster

You can scale a MongoDB cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, a restart is triggered and the primary pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mongodb-cluster
```

### Steps

1. Change configuration.

     Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

     ```bash
     kbcli cluster vscale mongodb-cluster --components=mongodb --cpu=500m --memory=500Mi
     ```

     - `--components` describes the component name ready for vertical scaling.
     - `--memory` describes the requested and limited size of the component memory.
     - `--cpu` describes the requested and limited size of the component CPU.
  
2. Validate the vertical scaling.

     ```bash
     kbcli cluster list mongodb-cluster
     >
     NAME              NAMESPACE   CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    CREATED-TIME                 
     mongodb-cluster   default     mongodb              mongodb-5.0      WipeOut              Running   Apr 26,2023 11:50 UTC+0800  
     ```

     - STATUS=Updating: it means the vertical scaling is in progress.
     - STATUS=Running: it means the vertical scaling operation has been applied.
     - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal.
         To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

3. Check whether the corresponding resources change.

     ```bash
     kbcli cluster describe mongodb-cluster
     ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from one to two. The scaling process includes the backup and restore of data.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mongodb-cluster
>
NAME                NAMESPACE        CLUSTER-DEFINITION    VERSION          TERMINATION-POLICY        STATUS         CREATED-TIME
mongodb-cluster     default          mongodb               mongodb-5.0      Delete                    Running        April 26,2023 12:00 UTC+0800
```

### Steps

1. Change configuration.

    Configure the parameters `--components` and `--replicas`, and run the command.

    ```bash
    kbcli cluster hscale mongodb-cluster \
    --components="mongodb" --replicas=2
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica amount of the specified components.

2. Validate the horizontal scaling operation.

    Check the cluster STATUS to identify the horizontal scaling status.

    ```bash
    kbcli cluster list mongodb-cluster
    ```

    - STATUS=Updating: it means horizontal scaling is in progress.
    - STATUS=Running: it means horizontal scaling has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mongodb-cluster
    ```

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-08T04:20:26Z"
    message: VolumeSnapshot/mongodb-cluster-mongodb-scaling-dbqgp: Failed to set default snapshot
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
    kubectl delete backup -l app.kubernetes.io/instance=mongodb-cluster
   
    kubectl delete volumesnapshot -l app.kubernetes.io/instance=mongodb-cluster
    ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
