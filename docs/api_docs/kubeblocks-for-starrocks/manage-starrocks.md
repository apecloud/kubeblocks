---
title: Manage StarRocks with KubeBlocks
description: How to manage StarRocks on KubeBlocks
keywords: [starrocks, analytic, data warehouse, control plane]
sidebar_position: 1
sidebar_label: Manage StarRocks with KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Manage StarRocks with KubeBlocks

StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis.

This tutorial illustrates how to create and manage a StarRocks cluster by `kubectl` or a YAML file. You can find the YAML examples and guides in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/release-0.9/examples/starrocks).

## Before you start

* [Install KubeBlocks](./../installation/install-kubeblocks.md).
* View all the database types and versions available for creating a cluster.
  
  Make sure the `starrocks` cluster definition is installed. If the cluster definition is not available, refer to [this doc](./../installation/install-addons.md) to enable it first.

  ```bash
  kubectl get clusterdefinition starrocks
  >
  NAME         TOPOLOGIES   SERVICEREFS    STATUS      AGE
  starrocks                                Available   30m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=starrocks
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

## Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a StarRocks cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: starrocks-ce
  clusterVersionRef: starrocks-ce-3.1.1
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: fe
    componentDefRef: fe
    serviceAccountName: kb-starrocks-cluster
    replicas: 1
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
  - name: be
    componentDefRef: be
    replicas: 2
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: be-storage
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
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
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created StarRocks cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## Scaling

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

#### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
```

#### Scale replicas

***Steps:***

There are two ways to apply horizontal scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: fe
       replicas: 2
   EOF
   ```

2. Check the operation status to validate the horizontal scaling status.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                           TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        mycluster-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

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
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: fe
       componentDefRef: fe
       replicas: 2 # Change the amount
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

#### Scale instances

From v0.9.0, KubeBlocks supports scale in or out of specified instances. For details, refer to [Horizontal Scale](./../../maintenance/scale/horizontal-scale.md#scale-instances).

### Scale vertically

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

#### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION              TERMINATION-POLICY     STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1   Delete                 Running   47m
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
     name: mycluster-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: fe
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
   NAMESPACE   NAME                         TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        mycluster-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

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
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: fe
       componentDefRef: fe
       replicas: 2
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
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
NAME        CLUSTER-DEFINITION    VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster   starrocks             starrocks-ce-3.1.1      Delete               Running   4m29s
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
      name: mycluster-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: be
        volumeClaimTemplates:
        - name: be-storage
          storage: "40Gi"
    EOF
    ```

2. Validate the volume expansion operation.

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                         TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        mycluster-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
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
     clusterDefinitionRef: starrocks-ce
     clusterVersionRef: starrocks-ce-3.1.1
     componentSpecs:
     - name: be
       componentDefRef: be
       volumeClaimTemplates:
       - name: be-storage
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 40Gi # Change the volume storage size.
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
  name: mycluster-stop
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
  clusterDefinitionRef: starrocks-ce
  clusterVersionRef: starrocks-ce-3.1.1
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: fe
    componentDefRef: fe
    serviceAccountName: kb-starrocks-cluster
    replicas: 0
  - name: be
    componentDefRef: be
    replicas: 0
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
  name: mycluster-start
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
spec:
  clusterDefinitionRef: starrocks-ce
  clusterVersionRef: starrocks-ce-3.1.1
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: fe
    componentDefRef: fe
    serviceAccountName: kb-starrocks-cluster
    replicas: 1
  - name: be
    componentDefRef: be
    replicas: 2
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
     name: mycluster-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: be
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
