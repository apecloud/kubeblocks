---
title: Manage RabbitMQ with KubeBlocks
description: How to manage RabbitMQ on KubeBlocks
keywords: [rabbitmq, message queue, streaming, broker]
sidebar_position: 1
sidebar_label: Manage RabbitMQ with KubeBlocks
---

# Manage RabbitMQ with KubeBlocks

RabbitMQ is a reliable and mature messaging and streaming broker, which is easy to deploy on cloud environments, on-premises, and on your local machine.

KubeBlocks supports the management of RabbitMQ.

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md).
- Install KubeBlocks [by kbcli](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by helm](./../installation/install-with-helm/install-kubeblocks.md).
- Install and enable the rabbitmq addon [by kbcli](./../installation/install-with-kbcli/install-addons.md) or [by Helm](./../installation/install-with-helm/install-addons.md).

## Create a cluster

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

***Steps***

1. Execute the following command to create a RabbitMQ cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition=rabbitmq --namespace=demo
   ```

   You can also create a cluster with specified CPU, memory and storage values.

   ```bash
   kbcli cluster create mycluster --cluster-definition=rabbitmq --set cpu=1,memory=2Gi,storage=10Gi,mode=clustermode --namespace=demo
   ```

:::note

View more flags for creating a cluster to create a cluster with customized specifications.
  
```bash
kbcli cluster create --help
```

:::

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list --namespace=demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   demo        rabbitmq             rabbitmq-3.13.2   Delete               Running    Sep 13,2024 11:06 UTC+0800   
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks implements a Cluster CRD to define a cluster. Here is an example of creating a RabbitMQ cluster with three replicas. Pods are distributed on different nodes by default. But if you only have one node for a cluster with three replicas, set `spec.affinity.topologyKeys` as `null`.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 3
      serviceAccountName: kb-mycluster
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      volumeClaimTemplates:
        - name: data # ref clusterDefinition components.containers.volumeMounts.name
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
      services:
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p><p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition qdrant -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to see the created RabbitMQ cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

</Tabs>

## Connect to the cluster

Use the [RabbitMQ tools](https://www.rabbitmq.com/docs/cli) to connect to and manage the RabbitMQ cluster.

## Scale

### Scale vertically

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Before you start, check whether the cluster status is Running. Otherwise, the following operations may fail.

```bash
kbcli cluster list rabbitmq -n demo
>
NAME       NAMESPACE    CLUSTER-DEFINITION     VERSION           TERMINATION-POLICY    STATUS    AGE
rabbitmq   demo         rabbitmq               rabbitmq-3.13.2   Delete                Running   47m
```

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale mycluster --cpu=2 --memory=2Gi --components=rabbitmq -n demo
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster vscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n demo
```

Validate the vertical scale operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### Option 1. Apply an OpsRequest

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
     - componentName: rabbitmq
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

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

#### Option 2. Edit the cluster YAML file

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 3
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

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Before you start, check whether the cluster status is Running. Otherwise, the following operations may fail.

```bash
kbcli cluster list rabbitmq -n demo
>
NAME       NAMESPACE    CLUSTER-DEFINITION     VERSION           TERMINATION-POLICY    STATUS    AGE
rabbitmq   demo         rabbitmq               rabbitmq-3.13.2   Delete                Running   47m
```

Use the following command to perform horizontal scaling.

```bash
kbcli cluster hscale mycluster --replicas=3 --components=rabbitmq -n demo
```

- `--components` describes the component name ready for horizontal scaling.
- `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-horizontalscaling-smx8b -n demo
```

Validate the horizontal scale operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### Option 1. Apply an OpsRequest

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

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
     - componentName: rabbitmq
       scaleIn:
         replicaChanges: 2
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn` and change the value in `replicaChanges`.

2. Check the operation status to validate the horizontal scaling status.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

#### Option 2. Edit the cluster YAML file

1. Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stands for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 1 # Change the amount
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
    terminationPolicy: Delete
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

## Volume expansion

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Before you start, check whether the cluster status is Running. Otherwise, the following operations may fail.

```bash
kbcli cluster list rabbitmq -n demo
>
NAME       NAMESPACE    CLUSTER-DEFINITION     VERSION           TERMINATION-POLICY    STATUS    AGE
rabbitmq   demo         rabbitmq               rabbitmq-3.13.2   Delete                Running   47m
```

Use the following command to perform volume expansion.

```bash
kbcli cluster volume-expand mycluster --storage=40Gi --components=rabbitmq -n demo
```

The volume expansion may take a few minutes.

The `kbcli cluster volume-expand` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-volumeexpansion-smx8b -n demo
```

Validate the volume expansion operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

### Option 1. Apply an OpsRequest

1. Change the value of storage according to your need and run the command below to expand the volume of a cluster.

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
      - componentName: rabbitmq
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

    If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

### Option 2. Edit the cluster YAML file

1. Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
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

## Restart

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster --components="rabbitmq" \
   --ttlSecondsAfterSucceed=30 -n demo
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION            TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        rabbitmq               rabbitmq-3.13.2    Delete               Running   Sep 13,2024 12:06 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
     - componentName: rabbitmq
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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the name of your cluster and run the command below to stop this cluster.

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

2. Check the status of the cluster to see whether it is stopped.

    ```bash
    kbcli cluster list -n demo
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

#### Option 1. Apply an OpsRequest

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

#### Option 2. Edit the cluster YAML file

Configure `replicas` as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 0
      serviceAccountName: kb-mycluster
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      volumeClaimTemplates:
        - name: data # ref clusterDefinition components.containers.volumeMounts.name
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
      services:
```

</TabItem>

</Tabs>

### Start a cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the name of your cluster and run the command below to start this cluster.

   ```bash
   kbcli cluster start mycluster -n demo
   ```

2. Check the status of the cluster to see whether it is running again.

    ```bash
    kbcli cluster list -n demo
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

#### Option 1. Apply an OpsRequest

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

#### Option 2. Edit the cluster YAML file

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
  labels:
    helm.sh/chart: rabbitmq-cluster-0.9.0
    app.kubernetes.io/version: "3.13.2"
    app.kubernetes.io/instance: mycluster
spec:
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
      - kubernetes.io/hostname
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.2
      replicas: 3
      serviceAccountName: kb-mycluster
      resources:
        limits:
          cpu: "0.5"
          memory: "0.5Gi"
        requests:
          cpu: "0.5"
          memory: "0.5Gi"
      volumeClaimTemplates:
        - name: data # ref clusterDefinition components.containers.volumeMounts.name
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
      services:
```

</TabItem>

</Tabs>

## Monitor

The monitoring function of RabbitMQ is the same as other engines. For details, refer to related docs:

- [Monitor databases by kbcli](./../observability/monitor-database.md)
- [Monitor databases by kubectl](./../../api_docs/observability/monitor-database.md)
