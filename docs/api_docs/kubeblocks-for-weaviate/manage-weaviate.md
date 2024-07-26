---
title: Manage Weaviate with KubeBlocks
description: How to manage Weaviate on KubeBlocks
keywords: [weaviate, vector database, control plane]
sidebar_position: 1
sidebar_label: Manage Weaviate with KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Manage Weaviate with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. Weaviate is an open-source vector database that simplifies the development of AI applications. Built-in vector and hybrid search, easy-to-connect machine learning models, and a focus on data privacy enable developers of all levels to build, iterate, and scale AI capabilities faster.

This tutorial illustrates how to create and manage a Weaviate cluster by `kubectl` or a YAML file. You can find the YAML examples and guides in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/release-0.9/examples/weaviate).

## Before you start

* [Install KubeBlocks](./../installation/install-kubeblocks.md).
* View all the database types and versions available for creating a cluster.
  
  Make sure the `weaviate` cluster definition is installed. If the cluster definition is not available, refer to [this doc](./../installation/install-addons.md) to enable it first.

  ```bash
  kubectl get clusterdefinition weaviate
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  weaviate                                Available   30m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=weaviate
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

## Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Weaviate cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: default
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - "kubernetes.io/hostname"
  clusterDefinitionRef: weaviate
  clusterVersionRef: weaviate-1.18.0
  componentSpecs:
  - componentDefRef: weaviate
    disableExporter: true
    name: weaviate
    replicas: 1
    resources:
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    serviceAccountName: kb-mycluster
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  resources:
    cpu: "0"
    memory: "0"
  storage:
    size: "0"
  terminationPolicy: Delete
status: {}
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
| `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p><p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition apecloud-mysql -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created Weaviate cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## Connect to a Weaviate cluster

Weaviate provides the HTTP protocol for client access on port 8080. You can visit the cluster by the local host.

```bash
curl http://localhost:8080/v1/meta | jq
```

## Scaling

Scaling function for vector databases is also supported.

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

#### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY     STATUS    AGE
mycluster   weaviate             weaviate-1.18.0   Delete                 Running   47m
```

#### Scale replicas

***Steps***

There are two ways to apply horizontal scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: weaviate
       replicas: 1
   EOF
   ```

2. Check the operation status to validate the horizontal scaling status.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the horizontal scaling operation, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>
  
<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```yaml
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: weaviate
     clusterVersionRef: weaviate-1.18.0
     componentSpecs:
     - name: weaviate
       componentDefRef: weaviate
       replicas: 2 # Change the amount
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

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

#### Scale instances

From v0.9.0, KubeBlocks supports scale in or out of specified instances. For details, refer to [Horizontal Scale](./../../maintenance/scale/horizontal-scale.md#scale-instances).

#### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2024-04-25T17:40:26Z"
    message: VolumeSnapshot/mycluster-weaviate-scaling-dbqgp: Failed to set default snapshot
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
   kubectl delete backup -l app.kubernetes.io/instance=mycluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.

### Scale vertically

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

#### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY     STATUS    AGE
mycluster   weaviate             weaviate-1.18.0   Delete                 Running   47m
```

#### Steps

There are two ways to apply vertical scaling.

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
     - componentName: weaviate
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
    ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: weaviate
     clusterVersionRef: weaviate-1.18.0
     componentSpecs:
     - name: weaviate
       componentDefRef: weaviate
       replicas: 1
       resources: # Change the values of resources.
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

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>
</Tabs>

## Volume Expanding

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION              TERMINATION-POLICY   STATUS    AGE
mycluster   weaviate             weaviate-1.18.0      Delete               Running   4m29s
```

### Steps

There are two ways to apply volume expansion.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```yaml
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: weaviate
        volumeClaimTemplates:
        - name: data
          storage: "40Gi"
    EOF
    ```

2. Validate the volume expansion operation.

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
    ```

    If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```yaml
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: weaviate
     clusterVersionRef: weaviate-1.18.0
     componentSpecs:
     - name: weaviate
       componentDefRef: weaviate
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # Change the volume storage size.
     terminationPolicy: Delete
   ```

2. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

### Stop a cluster

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

Run the command below to stop a cluster.

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-stop
  namespace: demo
spec:
  clusterName: mycluster
  type: Stop
EOF
```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

Configure replicas as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
    namespace: demo
spec:
  clusterDefinitionRef: weaviate
  clusterVersionRef: weaviate-1.18.0
  terminationPolicy: Delete
  componentSpecs:
  - name: weaviate
    componentDefRef: weaviate
    disableExporter: true  
    replicas: 0
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
```

</TabItem>

</Tabs>

### Start a cluster
  
<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

Run the command below to start a cluster.

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-start
  namespace: demo
spec:
  clusterName: mycluster
  type: Start
EOF 
```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
    namespace: demo
spec:
  clusterDefinitionRef: weaviate
  clusterVersionRef: weaviate-1.18.0
  terminationPolicy: Delete
  componentSpecs:
  - name: weaviate
    componentDefRef: weaviate
    disableExporter: true  
    replicas: 1
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
```

</TabItem>

</Tabs>

## Restart

1. Restart a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: weaviate
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo

   kubectl get ops ops-restart -n demo
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
