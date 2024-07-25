---
title: Horizontal Scale
description: How to horizontally scale a cluster
keywords: [horizontal scale, horizontal scaling]
sidebar_position: 1
sidebar_label: Horizontal Scale
---

# Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mycluster
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 18:46 UTC+0800
```

## Scale replicas

### Steps

1. Change configuration.

    Configure the parameters `--components` and `--replicas`, and run the command.

    ```bash
    kbcli cluster hscale mycluster \
    --components="mysql" --replicas=3
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica amount of the specified components.

2. Validate the horizontal scaling operation.

    Check the cluster STATUS to identify the horizontal scaling status.

    ```bash
    kbcli cluster list mycluster
    ```

    - STATUS=Updating: it means horizontal scaling is in progress.
    - STATUS=Running: it means horizontal scaling has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster
    ```

## Scale instances

### Why do you need to scale for specified instances

KubeBlocks generated workloads as *StatefulSets*, which was a double-edged sword. While KubeBlocks could leverage the advantages of a *StatefulSets* to manage stateful applications like databases, it inherited its limitations.

One of these limitations is evident in horizontal scaling scenarios, where *StatefulSets* offload Pods sequentially based on *Ordinal* order, potentially impacting the availability of databases running within.

For example, managing a PostgreSQL database with one primary and two secondary replicas using a *StatefulSet* named `foo-bar`. Over time, Pod `foo-bar-2` becomes the primary node. Now, if we decide to scale down the database due to low read load, according to *StatefulSet* rules, we can only offload Pod `foo-bar-2`, which is currently the primary node. Therefore we can either directly offload `foo-bar-2`, triggering a failover mechanism to elect a new primary pod from `foo-bar-0` and `foo-bar-1`, or use a switchover mechanism to convert `foo-bar-2` into a secondary pod before offloading it. Either way, there will be a period where write is not applicable.

Another issue arises in the same scenario: if the node hosting `foo-bar-1` experiences a hardware failure, causing disk damage and rendering data read-write inaccessible, according to operational best practices, we need to offload `foo-bar-1` and rebuild replicas on healthy nodes. However, performing such operational tasks based on *StatefulSets* isn't easy.

Similar discussions can be observed in the Kubernetes community. Starting from version 0.9, KubeBlocks introduces the *specified instance scaling* feature to address these issues.

### Steps

From v0.9.0, KubeBlocks supports the scale-in subcommand by adding the `offline-instances` option to specify an instance to scale in. But note that `--offlineInstances` should be edited with `replicas` at the same time to realize offload a specified instance.

The example below offloads the instance `mycluster-mysql-1`.

```bash
kbcli cluster hscale mycluster --components mysql --replicas 2 --offline-instances mycluster-mysql-1
```

:::note

- Here are some combinations that are allowed by the current API but not commonly used. It is recommended to fully know the mechanism and changes of running the options to avoid unexpected results.

   | Before Updating | After Updating | Instances After Updating |
   | :-------------  | :------------- | :----------------------- |
   | `replicas=3, offlineInstances=[]` | `replicas=2,offlineInstances=["mycluster-mysql-3"]` | `mycluster-mysql-0`, `mycluster-mysql-1` |
   | `replicas=2, offlineInstances=["mycluster-mysql-1"]` | `replicas=2, offlineInstances=[]` | `mycluster-mysql-0`, `mycluster-mysql-1` |
   | `replicas=2, offlineInstances=["mycluster-mysql-1"]` | `replicas=3, offlineInstances=["mycluster-mysql-1"]` | `mycluster-mysql-0`, `mycluster-mysql-2`, `mycluster-mysql-3` |

- Currently, `kbcli` only supports scale in one specified instance. But if you want to scale in or out several instances, you can refer to [Horizontal Scale](./../../../api_docs/maintenance/scale/horizontal-scale.md) in API docs to use `kubectl` and YAML files to realize your aim.

:::

## Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2024-07-05T04:20:26Z"
    message: VolumeSnapshot/mycluster-mysql-scaling-dbqgp: Failed to set default snapshot
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
