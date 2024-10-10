---
title: Manage Elasticsearch with KubeBlocks
description: How to manage Elasticsearch on KubeBlocks
keywords: [elasticsearch]
sidebar_position: 1
sidebar_label: Manage Elasticsearch with KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Manage Elasticsearch with KubeBlocks

Elasticsearch is a distributed, RESTful search and analytics engine that is capable of solving an ever-growing number of use cases. As the heart of the Elastic Stack, Elasticsearch stores your data centrally, allowing you to search it quickly, tune relevancy, perform sophisticated analytics, and easily scale.

KubeBlocks supports the management of Elasticsearch. This tutorial illustrates how to create and manage an Elasticsearch cluster by `kbcli`, `kubectl` or a YAML file. You can find the YAML examples and guides in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/release-0.9/examples/elasticsearch).

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md) if you want to manage your Elasticsearch cluster with `kbcli`.
- Install KubeBlocks [by kbcli](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../installation/install-with-helm/install-kubeblocks.md).
- Install and enable the elasticsearch Addon [by kbcli](./../installation/install-with-kbcli/install-addons.md) or [by Helm](./../installation/install-with-helm/install-addons.md).
- To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

## Create a cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

***Steps***

1. Execute the following command to create an Elasticsearch cluster.

   ```bash
   kbcli cluster create elasticsearch mycluster -n demo
   ```

:::note

If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.
  
```bash
kbcli cluster create elasticsearch --help

kbcli cluster create elasticsearch -h
```

:::

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   demo                                       Delete               Creating   Sep 27,2024 11:42 UTC+0800  
   ```

3. Check the cluster details.

   ```bash
   kbcli cluster describe elasticsearch -n demo
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating an Elasticsearch cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubeblocks.io/extra-env: '{"mdit-roles":"master,data,ingest,transform","mode":"multi-node"}'
  labels:
    app.kubernetes.io/instance: mycluster
    app.kubernetes.io/version: 8.8.2
    helm.sh/chart: elasticsearch-cluster-0.9.0
  name: mycluster
  namespace: demo
spec:
  affinity:
    podAntiAffinity: Required
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  componentSpecs:
  - componentDef: elasticsearch-8
    disableExporter: true
    name: mdit
    replicas: 3
    resources:
      limits:
        cpu: "1"
        memory: 2Gi
      requests:
        cpu: "1"
        memory: 2Gi
    serviceAccountName: null
    serviceVersion: 8.8.2
    services: null
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p><p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition elasticsearch -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created Elasticsearch cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

</Tabs>

## Connect to the Elasticsearch cluster

Elasticsearch provides the HTTP protocol for client access on port 9200. You can visit the cluster by the local host.

```bash
curl http://127.0.0.1:9200/_cat/nodes?v
```

## Monitor the Elasticsearch cluster

The monitoring function of Elasticsearch is the same as other engines. For details, refer to [the monitoring tutorial](./../observability/monitor-database.md).

## Scale

KubeBlocks supports horizontally and vertially scaling an Elasticsearch cluster.

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo                                               Delete               Running   Sep 27,2024 11:42 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster                                                 Delete               Running   4m29s
```

</TabItem>

</Tabs>

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Set the `--replicas` value according to your needs and perform the horizontal scaling.

    ```bash
    kbcli cluster hscale mycluster --replicas=2 --components=elasticsearch -n demo
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

    Please wait a few seconds until the scaling process is over.

2. Validate the horizontal scaling operation.

   - View the OpsRequest progress.

     KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

     ```bash
     kbcli cluster describe-ops mycluster-horizontalscaling-xpdwz -n demo
     ```

   - View the cluster satus.

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
     mycluster   demo                                               Delete               Updating   Sep 27,2024 10:01 UTC+0800
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

   The example below means adding two replicas.

   ```bash
   kubectl apply -f - <<EOF
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: elasticsearch
       scaleOut:
         replicaChanges: 2
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn`.

   The example below means deleting two replicas.

   ```bash
   kubectl apply -f - <<EOF
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: elasticsearch
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

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: mdit
       componentDefRef: elasticsearch
       replicas: 1 # Change the amount
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

### Scale vertically

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Set the `--cpu` and `--memory` values according to your needs and run the following command to perform vertical scaling.

    ```bash
    kbcli cluster vscale mycluster --cpu=2 --memory=3Gi --components=elasticsearch -n demo
    ```

    Please wait a few seconds until the scaling process is over.

2. Validate the vertical scaling operation.

   - View the OpsRequest progress.

     KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

     ```bash
     kbcli cluster describe-ops mycluster-verticalscaling-rpw2l -n demo
     ```

   - Check the cluster status.

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
     mycluster   demo                                               Delete               Updating   Sep 27,2024 10:01 UTC+0800
     ```

     - STATUS=Updating: it means the vertical scaling is in progress.
     - STATUS=Running: it means the vertical scaling operation has been applied.
     - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.

         To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>
  
<TabItem value="OpsRequest" label="OpsRequest">

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: elasticsearch-verticalscaling
      namespace: demo
    spec:
      clusterName: mycluster
      type: VerticalScaling
      verticalScaling:
      - componentName: mdit
        requests:
          cpu: '1'
          memory: '3Gi'
        limits:
          cpu: '1'
          memory: '3Gi'
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
      - name: mdit
        componentDef: elasticsearch
        serviceAccountName: null
        disableExporter: true
        replicas: 1
        resources:
          limits:
            cpu: '1'
            memory: 4Gi
          requests:
            cpu: '1'
            memory: 4Gi
    ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

## Volume Expansion

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo                                               Delete               Running   Sep 27,2024 11:42 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION                  TERMINATION-POLICY   STATUS    AGE
mycluster                                                 Delete               Running   4m29s
```

</TabItem>

</Tabs>

### Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Set the `--storage` value according to your need and run the command to expand the volume.

    ```bash
    kbcli cluster volume-expand mycluster --storage=40Gi --components=elasticsearch -t data -n demo
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
      kbcli cluster list mycluster
      >
      NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
      mycluster   demo                                               Delete               Updating   Sep 27,2024 10:01 UTC+0800
      ```

      * STATUS=Updating: it means the volume expansion is in progress.
      * STATUS=Running: it means the volume expansion operation has been applied.

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
      name: ops-volume-expansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: elasticsearch
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
     componentSpecs:
     - name: mdit
       componentDefRef: elasticsearch
       replicas: 2
       volumeClaimTemplates:
       - name: data
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

1. Configure the name of your cluster and run the command below to stop this cluster.

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster stop mycluster -n demo
    ```

    </TabItem>

    <TabItem value="OpsRequest" label="OpsRequest">

    Configure replicas as 0 to delete pods.

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

    Edit the cluster YAML file and configure replicas as 0 to delete pods.

    ```yaml
    kubectl edit cluster mycluster -n demo
    >
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
        name: mycluster
        namespace: demo
    spec:
      terminationPolicy: Delete
      componentSpecs:
      - name: mdit
        componentDefRef: elasticsearch
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
    kubectl edit cluster mycluster -n demo
    >
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
        name: mycluster
        namespace: demo
    spec:
      terminationPolicy: Delete
      componentSpecs:
      - name: mdit
        componentDefRef: elasticsearch
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

## Restart

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="elasticsearch" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME            CLUSTER-DEFINITION          VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster                                                         Delete               Running   Jul 05,2024 17:51 UTC+0800
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
     name: ops-restart
     namespace: demo
   spec:
     clusterName: mycluster
     type: Restart 
     restart:
     - componentName: elasticsearch
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
