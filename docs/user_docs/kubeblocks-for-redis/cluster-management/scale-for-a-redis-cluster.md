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

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

- During the vertical scaling process, a concurrent restart is triggered and the leader pod may change after the restarting.
- From v0.9.0, after vertical scaling is executed, KubeBlocks will automatically adjust part of the parameters of the database instance to more appropriate values. It is recommended to back up and record your custom parameter settings so that you can restore them if needed.

:::

### Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
```

***Example***

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   redis                redis-7.0.6    Delete               Running   4d18h
```

### Steps

1. Change configuration. There are 2 ways to apply vertical scaling.

   **Option 1.** Create an OpsRequest
  
   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
   spec:
     clusterRef: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: redis
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```
  
   **Option 2.** Change the YAML file of the cluster

   Change the configuration of `spec.componentSpecs.resources` in the YAML file.

   `spec.componentSpecs.resources` controls the requests and limits of resources and changing them triggers a vertical scaling.

   ***Example***

   ```YAML
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: default
   spec:
     clusterDefinitionRef: redis
     clusterVersionRef: redis-7.0.6
     componentSpecs:
     - name: redis
       componentDefRef: redis
       replicas: 1
       resources: # Change values of resources
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Delete
   ```

2. Validate the vertical scaling.

   Check the cluster status to identify the vertical scaling status.

   ```bash
   kubectl get cluster mycluster
   ```

   ***Example***

   ```bash
   kubectl get cluster mycluster
   >
   NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
   mycluster   redis                redis-7.0.6    Delete               Updating  4d18h
   ```

    :::note

    Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the opsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

    :::

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster
    ```

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale up from three pods to five pods. The scaling process includes the backup and restoration of data.

:::note

 From v0.9.0, after horizontal scaling is performed, KubeBlocks will automatically adjust part of the parameters of the database instance to more appropriate values. It is recommended to back up and record your custom parameter settings so that you can restore them if needed.

 :::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
```

***Example***

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   redis                redis-7.0.6    Delete               Running   4d18h
```

### Steps

1. Change configuration. There are 2 ways to apply horizontal scaling.

   **Option 1.** Create an OpsRequest

   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
   spec:
     clusterRef: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: redis
       replicas: 2
   EOF
   ```

   **Option 2.** Change the YAML file of the cluster

   Change the value of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ***Example***

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: default
   spec:
     clusterDefinitionRef: redis
     clusterVersionRef: redis-7.0.6
     componentSpecs:
     - name: redis
       componentDefRef: redis
       replicas: 2 # Change the pod amount.
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
    terminationPolicy: Delete
   ```

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kubectl get cluster mycluster
   ```

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-10T18:20:26Z"
    message: VolumeSnapshot/mycluster-redis-scaling-dbqgp: Failed to set default snapshot
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
