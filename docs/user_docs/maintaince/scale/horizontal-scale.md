---
title: Horizontal Scale
description: How to horizontally scale a cluster
keywords: [horizontal scale, horizontal scaling]
sidebar_position: 1
sidebar_label: Horizontal Scale
---

# Horizontal scaling

From v0.9.0, the horizontal scale can be divided into two types to better support various scenarios. Both final-state-oriented and procedure-oriented scales support scale in and out, but what distinguishes them is that the final-state-oriented scale only scales replicas and the procedure-oriented one supports scaling both the replicas and instances.

- Final-state-oriented

    Final-state-oriented horizontal scale refers to the operation in which components and instances specify replicas. This type of horizontal scale is performed by simply specifying the replica amount and overwriting the replicas. However, this method does not support specifying an instance to scale in or out.

    It is also recommended to edit the cluster YAML file to perform a horizontal scale since its corresponding OpsRequest option might be deprecated later.

- Procedure-oriented

    The procedure-oriented horizontal scale is designed to add or delete specified instances to meet the frequent scaling in and out demands.

    For the procedure-oriented horizontal scale, an operation will calculate the horizontal scale state based on the status of the pods involved in the OpsRequest operation. In extreme cases, due to the non-atomic nature of the operation, the recorded lastCompReplicas may be inaccurate, leading to incorrect pod count calculations (you can ensure the order on the client side or set `KUBEBLOCKS_RECONCILE_WORKERS=1`).

     1. If the issued OpsRequest attempts to delete an instance created by a running OpsRequest, it will not be allowed and will fail directly.
     2. If there is a final-state-oriented operation in execution, this operation will be terminated.

This tutorial takes a MySQL cluster as an example.

:::note

From v0.9.0, for MySQL and PostgreSQL, after horizontal scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature, which simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md).

:::

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

Before v0.9.0, KubeBlocks generated workloads as *StatefulSets*, which was a double-edged sword. While KubeBlocks could leverage the advantages of a *StatefulSets* to manage stateful applications like databases, it inherited its limitations.

One of these limitations is evident in horizontal scaling scenarios, where *StatefulSets* offload Pods sequentially based on *Ordinal* order, potentially impacting the availability of databases running within.

Another issue arises in the same scenario: if the node hosting Pods experiences a hardware failure, causing disk damage and rendering data read-write inaccessible, according to operational best practices, we need to offload the damaged Pod and rebuild replicas on healthy nodes. However, performing such operational tasks based on *StatefulSets* isn't easy. [Similar discussions](https://github.com/kubernetes/kubernetes/issues/83224) can be observed in the Kubernetes community.

To solve the limitations mentioned above, starting from version 0.9, KubeBlocks KubeBlocks replaces *StatefulStes* with *InstanceSet* which is a general workload API and is in charge of a set of instances. With *InstanceSet*, KubeBlocks also introduces the *specified instance scaling* feature to improve the availability.

### Steps

From v0.9.0, `kbcli` supports the scale-in subcommand by adding the `offline-instances` option to specify an instance to scale in. But note that `--offlineInstances` should be edited with `replicas` at the same time to realize offload a specified instance.

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
