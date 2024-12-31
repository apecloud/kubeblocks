---
title: Manage Qdrant with KubeBlocks
description: How to manage Qdrant on KubeBlocks
keywords: [qdrant, vector database, control plane]
sidebar_position: 1
sidebar_label: Manage Qdrant with KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Manage Qdrant with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. Qdrant (read: quadrant) is a vector similarity search engine and vector database. It provides a production-ready service with a convenient API to store, search, and manage points—vectors with an additional payload Qdrant is tailored to extended filtering support. It makes it useful for all sorts of neural-network or semantic-based matching, faceted search, and other applications.

KubeBlocks supports the management of Qdrant. This tutorial illustrates how to create and manage a Qdrant cluster by `kbcli`, `kubectl` or a YAML file. You can find the YAML examples in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/qdrant).

## Before you start

- [Install kbcli](./../installation/install-kbcli.md) if you want to manage the Qdrant cluster with `kbcli`.
- [Install KubeBlocks](./../installation/install-kubeblocks.md).
- [Install and enable the Qdrant Addon](./../installation/install-addons.md).
- To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

   ```bash
   kubectl create namespace demo
   ```

## Create a cluster

***Steps***

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Qdrant Replication cluster. Primary and Secondary are distributed on different nodes by default. But if you only have one node for deploying a Replication Cluster, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  terminationPolicy: Delete
  clusterDef: qdrant
  topology: cluster
  componentSpecs:
    - name: qdrant
      serviceVersion: 1.10.0
      replicas: 3
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: ""
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](#termination-policy). |
| `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be `qdrant` to create a Qdrant Cluster. |
| `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. The valid option is [cluster]. |
| `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are [1.10.0,1.5.0,1.7.3,1.8.1,1.8.4]. |
| `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are [true, false]. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. Recommended values are [3,5,7]. |
| `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
| `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
| `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
| `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
| `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created Qdrant cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Execute the following command to create a Qdrant cluster.

   ```bash
   kbcli cluster create qdrant mycluster -n demo
   ```

   If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create qdrant --help

   kbcli cluster create qdrant -h
   ```

   If you only have one node for deploying a cluster with multiple replicas, you can configure the cluster affinity by setting `--pod-anti-afffinity`, `--tolerations`, and `--topology-keys` when creating a cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        qdrant                              Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

3. Check the cluster information.

   ```bash
   kbcli cluster describe mycluster -n demo
   >
   Name: mycluster         Created Time: Aug 15,2023 23:03 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION        STATUS    TERMINATION-POLICY
   demo        qdrant                              Running   Delete

   Endpoints:
   COMPONENT   MODE        INTERNAL                                                 EXTERNAL
   qdrant      ReadWrite   mycluster-qdrant-qdrant.default.svc.cluster.local:6333   <none>
                           mycluster-qdrant-qdrant.default.svc.cluster.local:6334

   Topology:
   COMPONENT   INSTANCE             ROLE     STATUS    AZ       NODE                   CREATED-TIME
   qdrant      mycluster-qdrant-0   <none>   Running   <none>   x-worker3/172.20.0.3   Aug 15,2023 23:03 UTC+0800
   qdrant      mycluster-qdrant-1   <none>   Running   <none>   x-worker2/172.20.0.5   Aug 15,2023 23:03 UTC+0800
   qdrant      mycluster-qdrant-2   <none>   Running   <none>   x-worker/172.20.0.2    Aug 15,2023 23:04 UTC+0800

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
   qdrant      false       1 / 1                1Gi / 1Gi               data:20Gi      standard

   Images:
   COMPONENT   TYPE     IMAGE
   qdrant      qdrant   docker.io/qdrant/qdrant:latest

   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE     BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME
   Disabled      <none>            <none>   7d           <none>          <none>

   Show cluster events: kbcli cluster list-events -n demo mycluster
   ```

</TabItem>

</Tabs>

## Connect to a Qdrant cluster

Qdrant provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Qdrant cluster.

<Tabs>

<TabItem value="Port forward" label="Port forward" default>

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward svc/mycluster-qdrant 6333:6333 -n demo
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   curl http://127.0.0.1:6333/collections
   ```

   Refer to [the official Qdrant documents](https://qdrant.tech/documentation/) for the cluster operations.

</TabItem>

<TabItem value="kbcli" label="kbcli">

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe mycluster -n demo` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose mycluster -n demo --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose mycluster -n demo --enable=true --type=internet` to open a public network reachable address for the database cluster.

</TabItem>

</Tabs>

## Scale

The scaling function for Qdrant is also supported.

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to the [Horizontal Scale tutorial](./../maintenance/scale/horizontal-scale.md) for more details and examples.

#### Before you start

Check whether the cluster status is Running. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

#### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means adding two replicas.

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
     - componentName: qdrant
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
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: qdrant
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. Check the operation status to validate the horizontal scaling status.

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
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       replicas: 2 # Change this value
   ...
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Set the `--replicas` value according to your needs and perform the horizontal scaling.

    ```bash
    kbcli cluster hscale mycluster -n demo --replicas=5 --components=qdrant
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

   - View the cluster status.

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
     mycluster   demo        qdrant                         Delete               Running   Jul 24,2023 11:38 UTC+0800
     ```

     - STATUS=Updating: it means horizontal scaling is in progress.
     - STATUS=Running: it means horizontal scaling has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### Scale vertically

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, you can change the resource class from 1C2G to 2C4G by performing vertical scaling.

#### Before you start

Check whether the cluster status is Running. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

#### Steps

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
     - componentName: qdrant
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
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
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

<TabItem value="kbcli" label="kbcli">

1. Set the `--cpu` and `--memory` values according to your needs and run the following command to perform vertical scaling.

    ```bash
    kbcli cluster vscale mycluster -n demo --cpu=0.5 --memory=512Mi --components=qdrant 
    ```

    Please wait a few seconds until the scaling process is over.

2. Validate the vertical scaling operation.
   - View the OpsRequest progress.

      KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

      ```bash
      kbcli cluster describe-ops mycluster-verticalscaling-rpw2l -n demo
      >
      NAME                              TYPE              CLUSTER      STATUS    PROGRESS   AGE
      mycluster-verticalscaling-rpw2l   VerticalScaling   mycluster    Running   1/5        44s
     ```

   - Check the cluster status.

      ```bash
      kbcli cluster list mycluster -n demo
      >
      NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
      mycluster   demo                                               Delete               Updating   Aug 15,2023 23:03 UTC+0800
      ```

      - STATUS=Updating: it means the vertical scaling is in progress.
      - STATUS=Running: it means the vertical scaling operation has been applied.
      - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.
          >To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## Volume Expansion

### Before you start

Check whether the cluster status is Running. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                         Delete               Running   Aug 15,2023 23:03 UTC+0800
```

</TabItem>

</Tabs>

### Steps

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
      - componentName: qdrant
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

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources`.

   ```yaml
   ...
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.8.1
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # Change the volume storage size
   ...
   ```

2. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Set the `--storage` value according to your need and run the command to expand the volume.

    ```bash
    kbcli cluster volume-expand mycluster -n demo --storage=40Gi --components=qdrant -t data
    ```

    The volume expansion may take a few minutes.

2. Validate the volume expansion operation.

    - View the OpsRequest progress.

       KubeBlocks outputs a command automatically for you to view the details of the OpsRequest progress. The output includes the status of this OpsRequest and PVC. When the status is `Succeed`, this OpsRequest is completed.
   
       ```bash
       kbcli cluster describe-ops mycluster-volumeexpansion-5pbd2 -n demo
       >
       NAME                              TYPE              CLUSTER      STATUS   PROGRESS   AGE
       mycluster-volumeexpansion-5pbd2   VolumeExpansion   mycluster    Running  1/1        67s
       ```

    - View the cluster status.

       ```bash
       kbcli cluster list mycluster -n demo
       >
       NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
       mycluster   demo        qdrant                                 Delete               Updating   Aug 15,2023 23:03 UTC+0800
       ```

      * STATUS=Updating: it means the volume expansion is in progress.
      * STATUS=Running: it means the volume expansion operation has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## Restart

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

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
     - componentName: qdrant
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

<TabItem value="kbcli" label="kbcli">

1. Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="qdrant" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        qdrant                                 Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

</Tabs>

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

### Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

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

    ```bash
    kubectl edit cluster mycluster -n demo
    ```

    Configure the value of `spec.ComponentSpecs.replicas` as 0 to delete pods.

    ```yaml
    ...
    spec:
      clusterDefinitionRef: qdrant
      clusterVersionRef: qdrant-1.8.1
      terminationPolicy: Delete
      componentSpecs:
      - name: qdrant
        componentDefRef: qdrant
        disableExporter: true  
        replicas: 0 # Change this value
    ...
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster stop mycluster -n demo
    ```

    </TabItem>

    </Tabs>

2. Check the status of the cluster to see whether it is stopped.

    <Tabs>

    <TabItem value="kubectl" label="kubectl" default>

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>

### Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.
  
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

    ```bash
    kubectl edit cluster mycluster -n demo
    ```

    Change replicas back to the original amount to start this cluster again.

    ```yaml
    ...
    spec:
      clusterDefinitionRef: qdrant
      clusterVersionRef: qdrant-1.8.1
      terminationPolicy: Delete
      componentSpecs:
      - name: qdrant
        componentDefRef: qdrant
        disableExporter: true  
        replicas: 1 # Change this value
    ...
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster start mycluster -n demo
    ```

    </TabItem>

    </Tabs>

2. Check the status of the cluster to see whether it is running again.

    <Tabs>

    <TabItem value="kubectl" label="kubectl" default>

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster list mycluster -n demo
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
| `DoNotTerminate`      | `DoNotTerminate` prevents deletion of the Cluster. This policy ensures that all resources remain intact.       |
| `Delete`              | `Delete` deletes Cluster resources like Pods, Services, and Persistent Volume Claims (PVCs), leading to a thorough cleanup while removing all persistent data.   |
| `WipeOut`             | `WipeOut` is an aggressive policy that deletes all Cluster resources, including volume snapshots and backups in external storage. This results in complete data removal and should be used cautiously, primarily in non-production environments to avoid irreversible data loss.  |

To check the termination policy, execute the following command.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster   qdrant                              Delete                 Running   47m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION         TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        qdrant                                 Delete               Running   Aug 15,2023 23:03 UTC+0800 
```

</TabItem>

</Tabs>

### Steps

Run the command below to delete a specified cluster.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster delete mycluster -n demo
```

</TabItem>

</Tabs>

## Backup and restore

The backup and restore operations for Qdrant are the same as those of other clusters and you can refer to [the backup and restore documents](./../maintenance/backup-and-restore/introduction.md) for details. Remember to use `--method` parameter.
