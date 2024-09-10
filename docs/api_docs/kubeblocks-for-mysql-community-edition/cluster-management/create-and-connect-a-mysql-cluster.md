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

* [Install KubeBlocks](../../../user_docs/installation/install-with-helm/install-kubeblocks.md).
* View all the database types and versions available for creating a cluster.

* Make sure the `mysql` cluster definition is installed. If the cluster definition is not available, refer to [this doc](../../../user_docs/installation/install-with-helm/install-addons.md) to enable it first.

  ```bash
  kubectl get clusterdefinition mysql
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mysql                              Available   27m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mysql
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of MySQL clusters: Standalone and Replication. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with two replicas.

To ensure high availability, all replicas are distributed on different nodes by default. If you only have one node for deploying a Replication Cluster, set `spec.affinity.topologyKeys` as `null`.

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Replication Cluster.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: mysql
  clusterVersionRef: mysql-8.0.33
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
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - slow
    disableExporter: true
    replicas: 2
    serviceAccountName: kb-mysql-cluster
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
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p> <p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p> <p> - `Delete` is based on Halt and deletes PVCs. </p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition mysql -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

For the details of different parameters, you can refer to API docs.

Run the following commands to see the created MySQL cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

<details>
<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  creationTimestamp: "2024-03-28T10:08:23Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    clusterdefinition.kubeblocks.io/name: mysql
    clusterversion.kubeblocks.io/name: mysql-8.0.33
  name: mycluster
  namespace: default
  resourceVersion: "5466612"
  uid: 2f630f27-3de3-4fd0-85eb-5fd7e4150c8c
spec:
  affinity:
    podAntiAffinity: Preferred
  clusterDefinitionRef: mysql
  clusterVersionRef: mysql-8.0.33
  componentSpecs:
  - componentDefRef: mysql
    enabledLogs:
    - error
    - slow
    disableExporter: true
    name: mysql
    replicas: 2
    resources:
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    serviceAccountName: kb-mycluster
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  resources:
    cpu: "0"
    memory: "0"
  storage:
    size: "0"
  terminationPolicy: Delete
status:
  clusterDefGeneration: 2
  components:
    mysql:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-03-29T18:12:43Z"
  conditions:
  - lastTransitionTime: "2024-03-28T10:08:23Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-03-28T10:08:23Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-03-29T18:12:43Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-03-29T18:12:43Z"
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

## Connect to a MySQL Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl">

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator creates a new Secret called `mycluster-conn-credential` to store the connection credential of the MySQL cluster. This secret contains the following keys:

* `username`: the root username of the MySQL cluster.
* `password`: the password of the root user.
* `port`: the port of the MySQL cluster.
* `host`: the host of the MySQL cluster.
* `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root
   ```

   ```bash
   kubectl get secrets mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   b8wvrwlm
   ```

2. Exec into the Pod `mycluster-mysql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti mycluster-mysql-0 -- bash

   mysql -u root -p b8wvrwlm
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to a database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n default
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   mysql -uroot -pb8wvrwlm
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
