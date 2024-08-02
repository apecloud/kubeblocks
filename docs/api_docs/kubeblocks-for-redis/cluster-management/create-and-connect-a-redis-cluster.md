---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [redis, create a redis cluster, connect to a redis cluster, cluster, redis sentinel]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and Connect to a Redis cluster

This tutorial shows how to create and connect to a Redis cluster.

## Create a Redis cluster

### Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).

* View all the database types and versions available for creating a cluster.

  Make sure the `redis` cluster definition is installed. If the cluster definition is not available, refer to [this doc](./../../installation/install-addons.md) to enable it first.

  ```bash
  kubectl get clusterdefinition redis
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  redis                              Available   16m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   16m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### Create a cluster

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover.

To ensure high availability, Primary and Secondary are distributed on different nodes by default. If you only have one node for deploying a Replication Cluster, set `spec.affinity.topologyKeys` as `null`.

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Standalone.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: default
spec:
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
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
  - name: redis
    componentDefRef: redis
    replicas: 1
    disableExporter: true
    enabledLogs:
    - running
    serviceAccountName: kb-redis-cluster
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

Run the following command to see the created Redis cluster object:

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
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mycluster","app.kubernetes.io/version":"7.0.6","helm.sh/chart":"redis-cluster-0.6.0-alpha.36"},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"redis","clusterVersionRef":"redis-7.0.6","componentSpecs":[{"componentDefRef":"redis","enabledLogs":["running"],"disableExporter":true,"name":"redis","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-redis-cluster","services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2024-05-11T03:04:27Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: mycluster
    app.kubernetes.io/version: 7.0.6
    clusterdefinition.kubeblocks.io/name: redis
    clusterversion.kubeblocks.io/name: redis-7.0.6
    helm.sh/chart: redis-cluster-0.6.0-alpha.36
  name: mycluster
  namespace: demo
  resourceVersion: "28223"
  uid: 30744d02-61e4-4ccd-8dd7-94c6b14926ee
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  componentSpecs:
  - componentDefRef: redis
    enabledLogs:
    - running
    disableExporter: true
    name: redis
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-redis-cluster
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
    redis:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-05-11T03:04:39Z"
  conditions:
  - lastTransitionTime: "2024-05-11T03:04:27Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-05-11T03:04:27Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-05-11T03:04:39Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-05-11T03:04:39Z"
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

## Connect to a Redis Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the Redis cluster. This secret contains the following keys:

* `username`: the root username of the Redis cluster.
* `password`: the password of the root user.
* `port`: the port of the Redis cluster.
* `host`: the host of the Redis cluster.
* `endpoint`: the endpoint of the Redis cluster and it is the same as `host:port`.

1. Get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   default

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   5bv7czc4
   ```

2. Exec into the pod `mycluster-redis-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-redis-0 -- bash

   root@mycluster-redis-0:/# redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.
  
   ```bash
   kubectl port-forward -n demo svc/mycluster-redis 6379:6379
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-redis-0:/# redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
