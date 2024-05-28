---
title: Scale for a MySQL cluster
description: How to scale a MySQL cluster, horizontal scaling, vertical scaling
keywords: [mysql, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for an ApeCloud MySQL cluster

You can scale a MySQL cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

### Steps

1. Change configuration. There are 3 ways to apply vertical scaling.


   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
   kbcli cluster vscale mysql-cluster \
   --components="mysql" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   

2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list mysql-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS                 CREATED-TIME
    mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    VerticalScaling        Jan 29,2023 14:29 UTC+0800
    ```

   - STATUS=VerticalScaling: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
     > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

    :::note

    Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

    :::

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mysql-cluster
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

### Steps

1. Change configuration. There are 3 ways to apply horizontal scaling.

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale mysql-cluster \
   --components="mysql" --replicas=3
   ```

   - `--components` describes the component name ready for horizontal scaling.
   - `--replicas` describes the replica amount of the specified components.

   

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kbcli cluster list mysql-cluster
   ```

   - STATUS=HorizontalScaling: it means horizontal scaling is in progress.
   - STATUS=Running: it means horizontal scaling has been applied.

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
   kubectl delete backup -l app.kubernetes.io/instance=mysql-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mysql-cluster
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
