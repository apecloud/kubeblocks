---
title: Manage Vector Databases with KubeBlocks
description: How to manage vector databases on KubeBlocks
keywords: [qdrant, milvus,weaviate]
sidebar_position: 1
sidebar_label: Manage Vector Databases with KubeBlocks
---
# Manage Vector Databases with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. KubeBlocks supports the management of vector databases, such as Qdrant, Milvus, and Weaviate.

In this chapter, we take Qdrant as an example to show how to manage vector databases with KubeBlocks.

## Before you start

* Install KubeBlocks: You can install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the PostgreSQL add-on is enabled.

  ```bash
  kubectl get addons.extensions.kubeblocks.io qdrant
  >
  NAME         TYPE   VERSION   PROVIDER   STATUS    AGE
  qdrant       Helm                        Enabled   30m
  ```

* View all the database types and versions available for creating a cluster.
  
  Make sure the `postgresql` cluster definition is installed with `kubectl get clusterdefinitions postgresql`.

  ```bash
  kubectl get clusterdefinition qdrant
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  qdrant                                  Available   30m
  ```

  View all available versions for creating a cluster

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=qdrant
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

## Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Qdrant Replication cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: qdrant
  clusterVersionRef: qdrant-1.8.1
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
  - name: qdrant
    componentDefRef: qdrant
    monitor: false
    serviceAccountName: kb-qdrant-cluster
    replicas: 2
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
EOF
```

* `kubeblocks.io/extra-env` in `metadata.annotations` defines the topology mode of a MySQL cluster. If you want to create a Standalone cluster, you can change the value to `standalone`.
* `spec.clusterVersionRef` is the name of the cluster version CRD that defines the cluster version.
* * `spec.terminationPolicy` is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` blocks deletion operation. `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componentSpecs.componentDefRef` is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition apecloud-mysql -o json | jq '.spec.componentDefs[].name'`.
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.

If you only have one node for deploying a Replication Cluster, set `spec.affinity.topologyKeys` as `null`.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: qdrant
  clusterVersionRef: qdrant-1.8.1
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys: null
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: qdrant
    componentDefRef: qdrant
    monitor: false
    serviceAccountName: kb-qdrant-cluster
    replicas: 2
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
EOF
```

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created PostgreSQL cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

<details>

<summary>Output</summary>

</details>

## Connect to a vector database cluster

Qdrant provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Qdrant cluster.

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe qdrant` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose qdant --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose qdant --enable=true --type=internet` to open a public network reachable address for the database cluster.

## Monitor the vector database

TBD

## Scaling

Scaling function for vector databases is also supported.

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale pods up from three to five. The scaling process includes the backup and restore of data.

#### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   qdrant               qdrant-1.5.0   Halt                 Running   47m
```

#### Steps

1. Change configuration. There are two ways to apply horizontal scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

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
       replicas: 1
   EOF
   ```

   </TabItem>
  
   <TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

   Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.5.0
     componentSpecs:
     - name: qdrant
       componentDefRef: qdrant
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
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

#### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2024-04-25T17:40:26Z"
    message: VolumeSnapshot/mycluster-qdrant-scaling-dbqgp: Failed to set default snapshot
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

### Scale vertically

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource class from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

#### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   qdrant               qdrant-1.5.0   Halt                 Running   47m
```

#### Steps

1. Change configuration. There are two ways to apply vertical scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>
  
   Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

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
     - componentName: mysql
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

   </TabItem>
  
   <TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

   Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: qdrant
     clusterVersionRef: qdrant-1.5.0
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
     terminationPolicy: Halt
   ```

   </TabItem>

   </Tabs>

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

:::note

Vertical scaling does not synchronize the configuration related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

## Volume Expanding

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   4m29s
```

### Steps

1. Change configuration. There are two ways to apply volume expansion.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
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
      - componentName: mysql
        volumeClaimTemplates:
        - name: data
          storage: "40Gi"
    EOF
    ```

    </TabItem>

    <TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

    Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

    `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mycluster
      namespace: demo
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      componentSpecs:
      - name: mysql
        componentDefRef: mysql
        replicas: 1
        volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi # Change the volume storage size.
      terminationPolicy: Halt
    ```

    </TabItem>

    </Tabs>

2. Validate the volume expansion operation.

    ```bash
    kubectl get ops -n demo
    >
    NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
    ```

3. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

## Backup and restore

The backup and restore operations for qdrant are the same with those of other clusters.

You can refer to the [backup and restore docs](./../maintenance/backup-and-restore/introduction.md) for details.
