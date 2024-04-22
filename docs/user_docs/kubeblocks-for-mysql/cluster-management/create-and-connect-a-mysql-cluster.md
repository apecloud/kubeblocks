---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MySQL cluster

This tutorial shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install KubeBlocks by Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the ApeCloud MySQL addon is enabled.

  ```bash
  kubectl get addons.extensions.kubeblocks.io apecloud-mysql
  >
  NAME             TYPE   STATUS    AGE
  apecloud-mysql   Helm   Enabled   61s
  ```

* View all the database types and versions available for creating a cluster.
  
  Make sure the `apecloud-mysql` cluster definition is installed with `kubectl get clusterdefinition apecloud-mysql`.

  ```bash
  kubectl get clusterdefinition apecloud-mysql
  >
  NAME             MAIN-COMPONENT-NAME   STATUS      AGE
  apecloud-mysql   mysql                 Available   85m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=apecloud-mysql
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of MySQL clusters: Standalone and RaftGroup Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a RaftGroup Cluster, which creates a cluster with three replicas. To ensure high availability, all replicas are distributed on different nodes by default.

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a RaftGroup Cluster.

   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: demo
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - componentDefRef: mysql
       name: mysql
       replicas: 3
       resources:
         limits:
           cpu: "1"
           memory: 1Gi
         requests:
           cpu: "1"
           memory: 1Gi
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that defines the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that defines the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componentSpecs.componentDefRef` is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition apecloud-mysql -o json | jq '.spec.componentDefs[].name'`.
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` blocks deletion operation. `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location.

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mysql-cluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mysql-cluster -n demo
```

Run the following command to see the created MySQL cluster object:

```bash
kubectl get cluster mysql-cluster -n demo -o yaml
```

<details>
<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"name":"mysql-cluster","namespace":"demo"},"spec":{"clusterDefinitionRef":"apecloud-mysql","clusterVersionRef":"ac-mysql-8.0.30","componentSpecs":[{"componentDefRef":"mysql","name":"mysql","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"1Gi"},"requests":{"cpu":"0.5","memory":"1Gi"}},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-17T09:03:23Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    clusterdefinition.kubeblocks.io/name: apecloud-mysql
    clusterversion.kubeblocks.io/name: ac-mysql-8.0.30
  name: mysql-cluster
  namespace: demo
  resourceVersion: "27158"
  uid: de7c9fa4-7b94-4227-8852-8d76263aa326
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  componentSpecs:
  - componentDefRef: mysql
    monitor: false
    name: mysql
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 1Gi
      requests:
        cpu: "0.5"
        memory: 1Gi
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
    mysql:
      consensusSetStatus:
        leader:
          accessMode: None
          name: ""
          pod: Unknown
      phase: Failed
      podsReady: true
      podsReadyTime: "2023-07-17T09:03:37Z"
  conditions:
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: 'The operator has started the provisioning of Cluster: mysql-cluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-17T09:03:37Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-17T09:03:23Z"
    message: 'pods are unavailable in Components: [mysql], refer to related component
      message in Cluster.status.components'
    reason: ComponentsNotReady
    status: "False"
    type: Ready
  observedGeneration: 1
  phase: Running
```

## Connect to a MySQL Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator creates a new Secret called `mysql-cluster-conn-credential` to store the connection credential of the MySQL cluster. This secret contains the following keys:

* `username`: the root username of the MySQL cluster.
* `password`: the password of the root user.
* `port`: the port of the MySQL cluster.
* `host`: the host of the MySQL cluster.
* `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. Exec into the Pod `mysql-cluster-mysql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mysql-cluster-mysql-0 -- bash

   mysql -uroot -p2gvztbvz
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to a database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward svc/mysql-cluster-mysql 3306:3306 -n demo
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   mysql -uroot -p2gvztbvz
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md). 
