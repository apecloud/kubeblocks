---
title: Scale for a MySQL cluster
description: How to scale a MySQL cluster, horizontal scaling, vertical scaling
keywords: [mysql, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for a MySQL cluster

You can scale a MySQL cluster in two ways, vertical scaling and horizontal scaling.

:::note

After vertical scaling or horizontal scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature. This feature simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../configuration/configuration.md).

:::

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mycluster
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 19:06 UTC+0800
```

### Steps

1. Change configuration.

    Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

    ```bash
    kbcli cluster vscale mycluster \
    --components="mysql" \
    --memory="4Gi" --cpu="2" \
    ```

    - `--components` describes the component name ready for vertical scaling.
    - `--memory` describes the requested and limited size of the component memory.
    - `--cpu` describes the requested and limited size of the component CPU.

2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list mycluster
    >
    NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     CREATED-TIME
    mycluster   default     mysql                mysql-8.0.33   Delete               Updating   Jul 05,2024 19:11 UTC+0800
    ```

    - STATUS=Updating: it means the vertical scaling is in progress.
    - STATUS=Running: it means the vertical scaling operation has been applied.
    - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.

       To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mycluster
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 18:46 UTC+0800
```

### Scale replicas

#### Steps

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

### Scale instances

From v0.9.0, KubeBlocks supports scale in or out of specified instances. For details, refer to [Horizontal Scale](./../../maintaince/scale/horizontal-scale.md#scale-instances).

### Handle the snapshot exception

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
