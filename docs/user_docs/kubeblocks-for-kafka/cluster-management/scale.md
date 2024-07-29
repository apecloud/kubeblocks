---
title: Scale for a Kafka cluster
description: How to scale a Kafka cluster, horizontal scaling, vertical scaling
keywords: [kafka, horizontal scaling, vertical scaling]
sidebar_position: 3
sidebar_label: Scale
---

# Scale for a Kafka cluster

You can scale a Kafka cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list
>
NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
kafka-cluster   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
```

### Steps

1. Change configuration.

   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
    kbcli cluster vscale kafka-cluster --components="broker" --memory="4Gi" --cpu="2" 
   ```

   - `--components` value can be `broker` or `controller`.
     - broker: all nodes in the combined mode, or all the broker node in the separated node.
     - controller: all the corresponding nodes in the separated mode.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list kafka-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION       VERSION                TERMINATION-POLICY        STATUS          CREATED-TIME
    kafka-cluster        default          kafka                    kafka-3.3.2            Delete                    Updating        Jan 29,2023 14:29 UTC+0800
    ```

   - STATUS=Updating: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
     > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

    :::note

    Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

    :::

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe kafka-cluster
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five. The scaling process includes the backup and restore of data.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../maintaince/scale/horizontal-scale.md) for more details and examples.

### Before you start

- Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.
- You are not recommended to perform horizontal scaling on the controller node, including the controller node both in combined mode and separated node.
- When scaling in horizontally, you must know the topic partition storage. If the topic has only one replication, data loss may caused when you scale in broker.

  ```bash
  kbcli cluster list
  >
  NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
  kafka-cluster   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
  ```

### Steps

1. Change configuration.

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale kafka-cluster \
   --components="broker" --replicas=3
   ```

   - `--components` describes the component name ready for horizontal scaling.
   - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kbcli cluster list kafka-cluster
   ```

   - STATUS=Updating: it means horizontal scaling is in progress.
   - STATUS=Running: it means horizontal scaling has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe kafka-cluster
    ```

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/kafka-cluster-kafka-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=kafka-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=kafka-cluster

   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
