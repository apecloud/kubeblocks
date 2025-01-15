---
title: Manage RabbitMQ with KubeBlocks
description: How to manage RabbitMQ on KubeBlocks
keywords: [rabbitmq, message queue, streaming, broker]
sidebar_position: 1
sidebar_label: Manage RabbitMQ with KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Manage RabbitMQ with KubeBlocks

RabbitMQ is a reliable and mature messaging and streaming broker, which is easy to deploy on cloud environments, on-premises, and on your local machine.

KubeBlocks supports the management of RabbitMQ. This tutorial illustrates how to create and manage a Qdrant cluster by `kubectl` and YAML files. You can find more YAML examples in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/rabbitmq).

## Before you start

- [Install KubeBlocks](./../installation/install-kubeblocks.md).
- [Install and enable the rabbitmq addon](./../installation/install-addons.md).

## Create a cluster

KubeBlocks implements a Cluster CRD to define a cluster. Here is an example of creating a RabbitMQ cluster with three replicas. Pods are distributed on different nodes by default. But if you only have one node for a cluster with three replicas, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  terminationPolicy: Delete
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      serviceVersion: 3.13.7
      serviceAccountName: kb-rabbitmq-cluster
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
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: rabbitmq-cluster-peer-discovery
  namespace: demo
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kb-rabbitmq-cluster
  namespace: demo
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kb-rabbitmq-cluster
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rabbitmq-cluster-peer-discovery
subjects:
  - kind: ServiceAccount
    name: kb-rabbitmq-cluster
    namespace: demo
EOF
```

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](#termination-policy). |
| `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are [3.10.25,3.11.28,3.12.14,3.13.2,3.13.7,3.8.14,3.9.29]. |
| `spec.componentSpecs.serviceAccountName` | It specifies the name of the ServiceAccount required by the running Component. RabbitMQ needs the `peer-discovery` role to create events and get endpoints. This is essential for discovering other RabbitMQ nodes and forming a cluster. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. RabbitMQ prefers ODD numbers like [3, 5, 7]. All data/state is replicated across all replicas. |
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

Run the following command to see the created RabbitMQ cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

## Connect to the cluster

Use the [RabbitMQ tools](https://www.rabbitmq.com/docs/cli) to connect to and manage the RabbitMQ cluster.

## Scale

### Scale vertically

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

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
   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

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
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 3
       resources: # Change the values of resources
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

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to the [Horizontal Scale tutorial](./../maintenance/scale/horizontal-scale.md) for more details and examples.

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

#### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

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
   NAME                     TYPE                CLUSTER     STATUS    PROGRESS   AGE
   ops-horizontal-scaling   HorizontalScaling   mycluster   Succeed   2/2        6m
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

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
     componentSpecs:
     - name: rabbitmq
       componentDefRef: rabbitmq
       replicas: 1 # Change this value
   ...
   ```

2. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```

</TabItem>

</Tabs>

## Volume expansion

Before you start, check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   47m
```

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

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
    NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
    ops-volume-expansion   VolumeExpansion   mycluster   Succeed   1/1        6m
    ```

    If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

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
               storage: 40Gi # Change the volume storage size
   ...
   ```

2. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
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
     - componentName: rabbitmq
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo

   kubectl get ops -n demo
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

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

```bash
kubect edit cluster mycluster -n demo
```

Configure `replicas` as 0 to delete pods.

```bash
kubectl edit cluster mycluster -n demo
```

Edit the value of `replicas`.

```yaml
...
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
      replicas: 0 # Change this value
...
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

```bash
kubectl edit cluster mycluster -n demo
```

Change replicas back to the original amount to start this cluster again.

```bash
kubectl edit cluster mycluster -n demo
```

Edit the value of `replicas`.

```yaml
...
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
      replicas: 3 # Change this value
...
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

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION    VERSION        TERMINATION-POLICY     STATUS    AGE
mycluster                                        Delete                 Running   55m
```

### Steps

Run the command below to delete a specified cluster.

```bash
kubectl delete -n demo cluster mycluster
```

If you want to delete a cluster and its all related resources, you can modify the termination policy to `WipeOut`, then delete the cluster.

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
