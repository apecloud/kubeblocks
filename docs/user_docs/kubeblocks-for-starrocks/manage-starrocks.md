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

This tutorial illustrates how to create and manage a StarRocks cluster by `kbcli`, `kubectl` or a YAML file. You can find the YAML examples and guides in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/release-0.9/examples/starrocks).

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md) if you want to manage the StarRocks cluster with `kbcli`.
- Install KubeBlocks [by kbcli](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../installation/install-with-helm/install-kubeblocks.md).
- Install and enable the starrocks Addon [by kbcli](./../installation/install-with-kbcli/install-addons.md) or [by Helm](./../installation/install-with-helm/install-addons.md).
- To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

## Create a cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

***Steps***

1. Execute the following command to create a StarRocks cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks -n demo
   ```

   You can also create a cluster with specified CPU, memory and storage values.

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks --set cpu=1,memory=2Gi,storage=10Gi -n demo
   ```

:::note

If you want to customize your cluster specifications, `kbcli` provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

```bash
kbcli cluster create --help
kbcli cluster create -h
```

:::

2. Check whether the cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800   
   ```

3. Check the cluster information.

    ```bash
    kbcli cluster describe mycluster -n demo
    >
    Name: mycluster	 Created Time: Jul 17,2024 19:06 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
    demo        starrocks            starrocks-3.1.1   Running   Delete

    Endpoints:
    COMPONENT   MODE        INTERNAL                                      EXTERNAL
    fe          ReadWrite   mycluster-fe.default.svc.cluster.local:9030   <none>

    Topology:
    COMPONENT   INSTANCE         ROLE     STATUS    AZ       NODE                    CREATED-TIME
    be          mycluster-be-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800
    fe          mycluster-fe-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800

    Resources Allocation:
    COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    fe          false       1 / 1                1Gi / 1Gi               data:20Gi      standard
    be          false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT   TYPE   IMAGE
    fe          fe     apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/fe-ubuntu:2.5.4
    be          be     apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/fe-ubuntu:2.5.4

    Show cluster events: kbcli cluster list-events -n demo mycluster
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
  terminationPolicy: Delete
  topology: shared-nothing
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
    - name: fe
      componentDef: starrocks-ce-fe
      serviceVersion: 3.3.0
      replicas: 1
      resources:
        limits:
          cpu: "1"
          memory: "1Gi"
        requests:
          cpu: "1"
          memory: "1Gi"
      volumeClaimTemplates:
        - name: data # ref clusterDefinition components.containers.volumeMounts.name
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
    - name: be
      componentDef: starrocks-ce-be
      serviceVersion: 3.3.0
      replicas: 1
      resources:
        limits:
          cpu: "1"
          memory: "1Gi"
        requests:
          cpu: "1"
          memory: "1Gi"
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: "20Gi"
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](#termination-policy). |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition starrocks -o json \| jq '.spec.componentDefs[].name'`.   |
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

</TabItem>

</Tabs>

## Scale

### Scale vertically

#### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS        CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running       Jul 17,2024 19:06 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   starrocks            starrocks-3.1.1   Delete               Running   4m29s
```

</TabItem>

</Tabs>

#### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Set the `--cpu` and `--memory` values according to your needs and run the following command to perform vertical scaling.

    ```bash
    kbcli cluster vscale mycluster -n demo --cpu=2 --memory=20Gi --components=be
    ```

    Please wait a few seconds until the scaling process is over.

2. Validate the vertical scaling operation.

    - View the OpsRequest progress.

       KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

       ```bash
       kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n demo
       ```

    - Check the cluster status.

       ```bash
       kbcli cluster list mycluster -n demo
       >
       NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS     CREATED-TIME
       mycluster   demo        starrocks            starrocks-3.1.1     Delete               Updating   Jul 17,2024 19:06 UTC+0800
       ```

       - STATUS=Updating: it means the vertical scaling is in progress.
       - STATUS=Running: it means the vertical scaling operation has been applied.
       - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
          > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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
   kubectl edit cluster mycluster -n demo
   >
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

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to the [Horizontal Scale tutorial](./../maintenance/scale/horizontal-scale.md) for more details and examples.

#### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS        CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running       Jul 17,2024 19:06 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   starrocks            starrocks-3.1.1   Delete               Running   4m29s
```

</TabItem>

</Tabs>

#### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the parameters `--components` and `--replicas`, and run the command.

    ```bash
    kbcli cluster hscale mycluster --replicas=3 --components=be -n demo
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

    Please wait a few seconds until the scaling process is over.

2. Validate the vertical scaling.

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

<TabItem value="OpsRequest" label="OpsRequest">

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means adding two replicas for the component `fe`.

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
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn`.

   The example below means deleting two replicas for the component `fe`.

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
       scaleIn:
         replicaChanges: 2
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
   >
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

## Volume expansion

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS        CREATED-TIME
mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running       Jul 17,2024 19:06 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   starrocks            starrocks-3.1.1   Delete               Running   4m29s
```

</TabItem>

</Tabs>

### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Set the `--storage` value according to your need and run the command to expand the volume.

    ```bash
    kbcli cluster volume-expand mycluster -n demo --storage=40Gi --components=be
    ```

    The volume expansion may take a few minutes.

2. Validate the volume expansion operation.

    - View the OpsRequest progress.

       KubeBlocks outputs a command automatically for you to view the details of the OpsRequest progress. The output includes the status of this OpsRequest and PVC. When the status is `Succeed`, this OpsRequest is completed.

       ```bash
       kbcli cluster describe-ops mycluster-volumeexpansion-5pbd2 -n demo
       ```

    - View the cluster status.

       ```bash
       kbcli cluster list mycluster -n demo
       >
       NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
       mycluster   demo        starrocks            starrocks-3.1.1   Delete               Running   Jul 17,2024 19:06 UTC+0800
       ```

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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

    If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

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

## Restart

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="starrocks" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        starrocks               starrocks-3.1.1    Delete               Running   Jul 17,2024 19:06 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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

</TabItem>

</Tabs>

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

### Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster stop mycluster -n demo
    ```

    </TabItem>

    <TabItem value="OpsRequest" label="OpsRequest">

    Apply an OpsRequest to restart a cluster.

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

    <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

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

2. Check the status of the cluster to see whether it is stopped.

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

### Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster start mycluster -n demo
   ```

   </TabItem>

   <TabItem value="OpsRequest" label="OpsRequest">

   Apply an OpsRequest to start a cluster.

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

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

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

2. Check the status of the cluster to see whether it is running again.

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

## Delete a cluster

### Termination policy

:::note

The termination policy determines how a cluster is deleted.

:::

| **terminationPolicy** | **Deleting Operation**                           |
|:----------------------|:-------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` blocks delete operation.        |
| `Halt`                | `Halt` deletes Cluster resources like Pods and Services but retains Persistent Volume Claims (PVCs), allowing for data preservation while stopping other operations. Halt policy is deprecated in v0.9.1 and will have same meaning as DoNotTerminate. |
| `Delete`              | `Delete` extends the Halt policy by also removing PVCs, leading to a thorough cleanup while removing all persistent data.   |
| `WipeOut`             | `WipeOut` deletes all Cluster resources, including volume snapshots and backups in external storage. This results in complete data removal and should be used cautiously, especially in non-production environments, to avoid irreversible data loss.   |

To check the termination policy, execute the following command.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo                                               Delete               Running   Sep 30,2024 13:03 UTC+0800 
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   starrocks            starrocks-3.1.1   Delete               Running   34m
```

</TabItem>

</Tabs>

### Steps

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

</Tabs>
