---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [mogodb, create a mongodb cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a MongoDB cluster

This tutorial shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).

* View all the database types and versions available for creating a cluster.
  
  Make sure the `mongodb` cluster definition is installed. If the cluster definition is not available, refer to [this doc](./../../installation/install-addons.md) to enable it first.

  ```bash
  kubectl get clusterdefinition mongodb
  >
  NAME      TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mongodb                              Available   30m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with three replicas to support automatic failover.

To ensure high availability, all replicas are distributed on different nodes by default. If you only have one node for deploying a ReplicaSet Cluster, set `spec.affinity.topologyKeys` as `null`.

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a MongoDB Standalone.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-6.0
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
  - name: mongodb
    componentDefRef: mongodb
    enabledLogs:
    - running
    disableExporter: true
    serviceAccountName: kb-mongo-cluster
    replicas: 1
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

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
| `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p><p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p> <p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. </p> |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition mongodb -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster by running the command below.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
```

Run the following command to view the created MongoDB cluster object.

```bash
kubectl get cluster mycluster -n demo -o yaml
```

<details>

<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mycluster","app.kubernetes.io/version":"5.0.14","helm.sh/chart":"mycluster-0.8.0"},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"mongodb","clusterVersionRef":"mongodb-5.0","componentSpecs":[{"componentDefRef":"mongodb","disableExporter":true,"name":"mongodb","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":null,"services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2024-05-07T10:23:13Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: mycluster
    app.kubernetes.io/version: 5.0.14
    clusterdefinition.kubeblocks.io/name: mongodb
    clusterversion.kubeblocks.io/name: mongodb-5.0
    helm.sh/chart: mongodb-cluster-0.8.0
  name: mycluster
  namespace: demo
  resourceVersion: "560727"
  uid: 3fced3b6-34bf-4d3a-88e2-baf4e2d73b44
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-5.0
  componentSpecs:
  - componentDefRef: mongodb
    disableExporter: true
    name: mongodb
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
status:
  clusterDefGeneration: 2
  components:
    mongodb:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-05-07T10:23:55Z"
  conditions:
  - lastTransitionTime: "2024-05-07T10:23:13Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-05-07T10:23:13Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-05-07T10:23:55Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-05-07T10:23:55Z"
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

## Connect to a MongoDB Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the MongoDB cluster. This secret contains the following keys:

* `username`: the root username of the MongoDB cluster.
* `password`: the password of the root user.
* `port`: the port of the MongoDB cluster.
* `host`: the host of the MongoDB cluster.
* `endpoint`: the endpoint of the MongoDB cluster and it is the same as `host:port`.

1. Get the `username` and `password` to connect to this MongoDB cluster for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   266zfqx5
   ```

2. Exec into the Pod `mycluster-mongodb-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/mycluster-mongodb 27017:27017  
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
