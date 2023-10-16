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

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to create and connect a cluster by kbcli.
* Install KubeBlocks: You can install KubeBlocks by [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the MongoDB add-on is enabled.
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  mongodb                        Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get clusterdefinitions mongodb
  >
  NAME      MAIN-COMPONENT-NAME   STATUS      AGE
  mongodb   mongodb               Available   118m
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli clusterdefinition list

  kbcli clusterversion list
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  Make sure the `mongodb` cluster definition is installed with `kubectl get clusterdefinitions mongodb`.

  ```bash
  kubectl get clusterdefinitions mongodb
  >
  NAME      MAIN-COMPONENT-NAME   STATUS      AGE
  mongodb   mongodb               Available   118m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  >
  NAME             CLUSTER-DEFINITION   STATUS      AGE
  mongodb-5.0   mongodb              Available   118m
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### Create a cluster

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with a three replicas to support automatic failover. And to ensure high availability, all replicas are distributed on different nodes by default.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Create a Standalone.

```bash
kbcli cluster create mongodb <clustername>
```

Create a ReplicatSet.

```bash
kbcli cluster create mongodb --mode replicaset <clustername>
```

If you only have one node for deploying a ReplicaSet, set the `availability-policy` as `none` when creating a ReplicaSet.

```bash
kbcli cluster create mongodb --mode replicaset --availability-policy none <clustername>
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease cluster availability.
* Run the command below to view the flags for creating a MongoDB cluster and the default values.
  
  ```bash
  kbcli cluster create mongodb -h
  ```

:::

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a MongoDB Standalone.

  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: mongodb-cluster
    namespace: demo
    labels: 
      helm.sh/chart: mongodb-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "5.0"
      app.kubernetes.io/instance: mongodb
  spec:
    clusterVersionRef: mongodb-5.0
    terminationPolicy: Delete  
    affinity:
      podAntiAffinity: Preferred
      topologyKeys:
        - kubernetes.io/hostname
      tenancy: SharedNode
    clusterDefinitionRef: mongodb
    componentSpecs:
      - name: mongodb
        componentDefRef: mongodb      
        monitor: false      
        replicas: 1
        serviceAccountName: kb-mongodb      
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that defines the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that defines the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componentSpecs.componentDefRef` is the name of the component definition that is defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition mongodb -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` will delete workload resources such as statefulset and deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo
```

Run the following command to see the created MongoDB cluster object.

```bash
kubectl get cluster mongodb-cluster -n demo -o yaml
```

<details>

<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mongodb","app.kubernetes.io/version":"5.0","helm.sh/chart":"mongodb-cluster-0.6.0-alpha.36"},"name":"mongodb-cluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"mongodb","clusterVersionRef":"mongodb-5.0","componentSpecs":[{"componentDefRef":"mongodb","monitor":false,"name":"mongodb","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-mongodb","services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T08:59:48Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: mongodb
    app.kubernetes.io/version: 5.0
    clusterdefinition.kubeblocks.io/name: mongodb
    clusterversion.kubeblocks.io/name: mongodb-5.0
    helm.sh/chart: mongodb-cluster-0.6.0-alpha.36
  name: mongodb-cluster
  namespace: demo
  resourceVersion: "16137"
  uid: 6a488eaa-29f2-417f-b248-d10d0512e14a
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
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-mongodb
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
      consensusSetStatus:
        leader:
          accessMode: ReadWrite
          name: primary
          pod: mongodb-cluster-mongodb-0
      phase: Running
      podsReady: true
      podsReadyTime: "2023-07-19T09:00:24Z"
  conditions:
  - lastTransitionTime: "2023-07-19T08:59:49Z"
    message: 'The operator has started the provisioning of Cluster: mongodb-cluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-19T08:59:49Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-19T09:00:24Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-19T09:00:29Z"
    message: 'Cluster: mongodb-cluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

</Tabs>

## Connect to a MongoDB Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mongodb-cluster-conn-credential` to store the connection credential of the MongoDB cluster. This secret contains the following keys:

* `username`: the root username of the MongoDB cluster.
* `password`: the password of the root user.
* `port`: the port of the MongoDB cluster.
* `host`: the host of the MongoDB cluster.
* `endpoint`: the endpoint of the MongoDB cluster and it is the same as `host:port`.

1. Get the `username` and `password` to connect to this MongoDB cluster for the `kubectl exec` command.

```bash
kubectl get secrets -n demo mongodb-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
>
root

kubectl get secrets -n demo mongodb-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
>
svk9xzqs
```

2. Exec into the Pod `mongodb-cluster-mongodb-0` and connect to the database using username and password.

```bash
kubectl exec -ti -n demo mongodb-cluster-mongodb-0 -- bash

root@mongodb-cluster-mongodb-0:/# mongo --username root --password svk9xzqs --authenticationDatabase admin
```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine. 

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/mongodb-cluster-mongodb 27017:27017  
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mongodb-cluster-mongodb-0:/# mongo --username root --password svk9xzqs --authenticationDatabase admin
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
