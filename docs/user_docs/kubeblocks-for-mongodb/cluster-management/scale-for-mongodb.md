---
title: Scale for MongoDB cluster
description: How to vertically scale a MongoDB cluster
keywords: [mongodb, vertical scaling, vertically scale a mongodb cluster]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale a MongoDB cluster

You can scale a MongoDB cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME             NAMESPACE        CLUSTER-DEFINITION    VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster        demo             mongodb               mongodb-5.0        Delete                    Running        Apr 10,2023 16:20 UTC+0800
```

</TabItem>

</Tabs>

### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mongodb
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ...
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. 

   `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.resources`.

   ```yaml
   ...
   spec:
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - componentDefRef: mongodb
       enabledLogs:
       - running
       disableExporter: true
       name: mongodb
       replicas: 2
       resources: # Change values of resources
         limits:
           cpu: "2"
           memory: 4Gi
         requests:
           cpu: "1"
           memory: 2Gi
   ```

2. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ...
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      mongodb
    Replicas:  1
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

     ```bash
     kbcli cluster vscale mycluster -n demo --components=mongodb --cpu=500m --memory=500Mi
     ```

     - `--components` describes the component name ready for vertical scaling.
     - `--memory` describes the requested and limited size of the component memory.
     - `--cpu` describes the requested and limited size of the component CPU.
  
2. Validate the vertical scaling.

     - View the OpsRequest progress.

         KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

         ```bash
         kbcli cluster describe-ops mycluster-verticalscaling-g67k9 -n demo
         ```

     - Check the cluster status.

         ```bash
         kbcli cluster list mycluster -n demo
         >
         NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    CREATED-TIME                 
         mycluster   demo        mongodb              mongodb-5.0      Delete               Running   Apr 26,2023 11:50 UTC+0800  
         ```

     - STATUS=Updating: it means the vertical scaling is in progress.
     - STATUS=Running: it means the vertical scaling operation has been applied.
     - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal.
         > To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

     ```bash
     kbcli cluster describe mycluster -n demo
     ```

</TabItem>

</Tabs>

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to the [Horizontal Scale tutorial](./../../maintenance/scale/horizontal-scale.md) for more details and examples.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY     STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME          NAMESPACE     CLUSTER-DEFINITION    VERSION          TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster     demo          mongodb               mongodb-5.0      Delete                    Running        April 26,2023 12:00 UTC+0800
```

</TabItem>

</Tabs>

### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means adding two replicas.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mongo-horizontalscaling
     namespace: default
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mongodb
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn`.

   The example below means deleting two replicas.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mongo-horizontalscaling
     namespace: default
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mongodb
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. Check the operation status to validate the horizontal scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>
  
<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.replicas`.

   ```yaml
   ...
   spec:
     clusterDefinitionRef: mongo
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - name: mongo
       componentDefRef: mongo
       replicas: 4 # Change this value
   ...
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Configure the parameters `--components` and `--replicas`, and run the command.

    ```bash
    kbcli cluster hscale mycluster -n demo --components="mongodb" --replicas=2
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

2. Validate the horizontal scaling operation.

     - View the OpsRequest progress.

         KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

         ```bash
         kbcli cluster describe-ops mycluster-horizontalscaling-ffp9p -n demo
         ```

     - View the cluster satus.

         ```bash
         kbcli cluster list mycluster -n demo
         ```

         - STATUS=Updating: it means horizontal scaling is in progress.
         - STATUS=Running: it means horizontal scaling has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-04-08T04:20:26Z"
    message: VolumeSnapshot/mycluster-mongodb-scaling-dbqgp: Failed to set default snapshot
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
    kubectl delete backup -l app.kubernetes.io/instance=mycluster -n demo
   
    kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster -n demo
    ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
