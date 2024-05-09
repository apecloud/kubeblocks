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

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list
>
NAME    NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
ivy85   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
```

### Steps

1. Change configuration. There are 3 ways to apply vertical scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
    kbcli cluster vscale ivy85 --components="broker" --memory="4Gi" --cpu="2" 
   ```

   - `--components` value can be `broker` or `controller`.
     - broker: all nodes in the combined mode, or all the broker node in the separated node.
     - controller: all the corresponding nodes in the separated mode.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   **Option 2.** Create an OpsRequest
  
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
  
   **Option 3.** Change the YAML file of the cluster

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
           cpu: "1000m"
         limits:
           memory: "4Gi"
           cpu: "2000m"
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
  
2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list mysql-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS                 CREATED-TIME
    ivy85                 default          kafka                kafka-3.3.2            Delete                    VerticalScaling        Jan 29,2023 14:29 UTC+0800
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
    kbcli cluster describe ivy85
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restoration of data.

### Before you start

- Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.
- You are not recommended to perform horizontal scaling on the controller node, including the controller node both in combined mode and separated node.
- When scaling in horizontally, you must know the topic partition storage. If the topic has only one replication, data loss may caused when you scale in broker.

  ```bash
  kbcli cluster list
  >
  NAME    NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME                 
  ivy85   default     kafka                kafka-3.3.2   Delete               Running   Jul 19,2023 18:01 UTC+0800   
  ```

### Steps

1. Change configuration. There are 3 ways to apply horizontal scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale mysql-cluster \
   --components="broker" --replicas=3
   ```

 
   - `--components` value can be `broker` or `controller`.
     - broker: all nodes in the combined mode, or all the broker node in the separated node.
     - controller: all the corresponding nodes in the separated mode.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   **Option 2.** Create an OpsRequest

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

   **Option 3.** Change the YAML file of the cluster

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

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kbcli cluster list ivy85
   ```

   - STATUS=HorizontalScaling: it means horizontal scaling is in progress.
   - STATUS=Running: it means horizontal scaling has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe ivy85
    ```

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
