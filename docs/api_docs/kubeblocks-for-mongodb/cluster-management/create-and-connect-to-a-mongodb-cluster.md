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

* Install KubeBlocks: You can install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the MongoDB add-on is enabled.

  ```bash
  kubectl get addons.extensions.kubeblocks.io mongodb
  >
  NAME      TYPE   VERSION   PROVIDER   STATUS    AGE
  mongodb   Helm                        Enabled   26m
  ```

* View all the database types and versions available for creating a cluster.
  
  Make sure the `mongodb` cluster definition is installed with `kubectl get clusterdefinitions postgresql`.

  ```bash
  kubectl get clusterdefinition mongodb
  >
  NAME      TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mongodb                              Available   30m
  ```

  View all available versions for creating a cluster

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with three replicas to support automatic failover. To ensure high availability, all replicas are distributed on different nodes by default.

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
    tenancy: SharedNode
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
    monitorEnabled: false
    serviceAccountName: kb-mongo-cluster
    replicas: 3
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

:::note

If you only have one node for deploying a RaftGroup Cluster, set `spec.affinity.topologyKeys` as `null`.

:::

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo
```

Run the following command to see the created MongoDB cluster object.

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
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mycluster","app.kubernetes.io/version":"5.0.14","helm.sh/chart":"mongodb-cluster-0.8.0"},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"mongodb","clusterVersionRef":"mongodb-5.0","componentSpecs":[{"componentDefRef":"mongodb","monitor":false,"name":"mongodb","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":null,"services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
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
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: mongodb
  clusterVersionRef: mongodb-5.0
  componentSpecs:
  - componentDefRef: mongodb
    monitor: false
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

KubeBlocks operator has created a new Secret called `mongodb-cluster-conn-credential` to store the connection credential of the MongoDB cluster. This secret contains the following keys:

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

root@mongodb-cluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
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
   root@mongodb-cluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
