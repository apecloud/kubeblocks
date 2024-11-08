---
title: Scale for a Pulsar
description: How to scale a Pulsar cluster, horizontal scaling, vertical scaling
keywords: [pulsar, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale for a Pulsar cluster

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
```

</TabItem>

</Tabs>

### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ```bash
   kbcli cluster vscale mycluster --cpu=3 --memory=10Gi --components=broker,bookies -n demo
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

2. Validate the vertical scaling operation.

   - View the OpsRequest progress.

       KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

       ```bash
       kbcli cluster describe-ops mycluster-verticalscaling-njl6s -n demo
       ```

   - Check the cluster status.

       ```bash
       kbcli cluster list mycluster -n demo
       ```

       - STATUS=updating: it means the vertical scaling is in progress.
       - STATUS=Running: it means the vertical scaling operation has been applied.
       - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
          > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

    :::note

    Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

    :::

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl create -f -<< EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vscale
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: pulsar-broker
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3
     - componentName: bookies
       requests:
         memory: "10Gi"
         cpu: 3
       limits:
         memory: "10Gi"
         cpu: 3      
   EOF
   ```

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.components.resources` in the YAML file. 

   `spec.components.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   ......
   spec:
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     clusterDefinitionRef: pulsar
     clusterVersionRef: pulsar-3.0.2
     componentSpecs:
     - componentDefRef: pulsar
       enabledLogs:
       - running
       disableExporter: true
       name: pulsar
       replicas: 1
       resources:
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
   ......
   Component Specs:
    Component Def Ref:  pulsar
    Enabled Logs:
      running
    DisableExporter:   true
    Name:      pulsar
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

</Tabs>

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

### Before you start

- It is recommended to keep 3 nodes without scaling for Zookeeper, and other components can scale horizontally for multiple or single components
- The scaling of the Bookies node needs to be cautious. The data copy is related to the EnsembleSize, Write Quorum, and Ack Quorum configurations, scaling may cause data loss. Check [Pulsar official document](https://pulsar.apahe.org/docs/3.0.x/administration-zk-bk/#decommission-bookies-cleanly) for detailed information.
- Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kubectl" label="kubectl">

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    </Tabs>

### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Change configuration.

   Configure the parameters `--components` and `--replicas`, and run the command.

   ```bash
   kbcli cluster hscale pulsar-cluster --replicas=5 --components=broker,bookies    
   ```

   - `--components` describes the component name ready for horizontal scaling.
   - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

2. Validate the horizontal scaling operation.

   - View the OpsRequest progress.

       KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

       ```bash
       kbcli cluster describe-ops mycluster-horizontalscaling-9lfvc -n demo
       ```

   - View the cluster satus.

       Check the cluster STATUS to identify the horizontal scaling status.

       ```bash
       kbcli cluster list mycluster -n demo
       ```

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

   ```bash
   kbcli cluster describe mycluster -n demo
   ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means adding two replicas.

    ```bash
    kubectl create -f -<< EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-horizontalscaling
      namespace: demo
    spec:
      clusterRef: mycluster
      type: HorizontalScaling  
      horizontalScaling:
      - componentName: pulsar-proxy
        scaleOut:
          replicaChanges: 2
    EOF
    ```

    If you want to scale in replicas, replace `scaleOut` with `scaleIn`.

   The example below means deleting two replicas.

    ```bash
    kubectl create -f -<< EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-horizontalscaling
      namespace: demo
    spec:
      clusterRef: mycluster
      type: HorizontalScaling  
      horizontalScaling:
      - componentName: pulsar-proxy
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

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file.

   `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: pulsar
     clusterVersionRef: pulsar-3.0.2
     componentSpecs:
     - name: pulsar
       componentDefRef: pulsar-proxy
       replicas: 2 # Change the amount
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.

In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mycluster-pulsar-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=mycluster -n demo
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster -n demo
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
