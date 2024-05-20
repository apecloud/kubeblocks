---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [postgresql, create a postgresql cluster, connect to a postgresql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a PostgreSQL cluster

This tutorial shows how to create and connect to a PostgreSQL cluster.

## Create a PostgreSQL cluster

### Before you start

* Install KubeBlocks: You can install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the PostgreSQL add-on is enabled.

  ```bash
  kubectl get addons.extensions.kubeblocks.io postgresql
  >
  NAME         TYPE   VERSION   PROVIDER   STATUS    AGE
  postgresql   Helm                        Enabled   30m
  ```

* View all the database types and versions available for creating a cluster.
  
  Make sure the `postgresql` cluster definition is installed with `kubectl get clusterdefinitions postgresql`.

  ```bash
  kubectl get clusterdefinition postgresql
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  postgresql                              Available   30m
  ```

  View all available versions for creating a cluster

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=postgresql
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of PostgreSQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with a Replication Cluster to support automatic failover. And to ensure high availability, Primary and Secondary are distributed on different nodes by default.

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Standalone.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-12.14.0
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
  - name: postgresql
    componentDefRef: postgresql
    enabledLogs:
    - running
    monitor: false
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
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-12.14.0
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
  - name: postgresql
    componentDefRef: postgresql
    enabledLogs:
    - running
    monitor: false
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

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubeblocks.io/reconcile: "2024-05-17T03:17:08.617771416Z"
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":null},"clusterDefinitionRef":"postgresql","clusterVersionRef":"postgresql-12.14.0","componentSpecs":[{"componentDefRef":"postgresql","enabledLogs":["running"],"monitor":false,"name":"postgresql","replicas":2,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete","tolerations":[{"effect":"NoSchedule","key":"kb-data","operator":"Equal","value":"true"}]}}
  creationTimestamp: "2024-05-17T02:36:28Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    clusterdefinition.kubeblocks.io/name: postgresql
    clusterversion.kubeblocks.io/name: postgresql-12.14.0
  name: mycluster
  namespace: demo
  resourceVersion: "1019716"
  uid: 8123622e-2fd4-4d13-8836-61c4e2dc8df4
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-12.14.0
  componentSpecs:
  - componentDefRef: postgresql
    enabledLogs:
    - running
    monitor: false
    name: postgresql
    replicas: 2
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
  tolerations:
  - effect: NoSchedule
    key: kb-data
    operator: Equal
    value: "true"
status:
  clusterDefGeneration: 2
  components:
    postgresql:
      phase: Running
      podsReady: true
      podsReadyTime: "2024-05-17T02:37:50Z"
  conditions:
  - lastTransitionTime: "2024-05-17T02:36:28Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2024-05-17T02:36:28Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2024-05-17T02:37:50Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2024-05-17T02:37:50Z"
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

## Connect to a PostgreSQL Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the `pg-cluster` cluster. This secret contains following keys:

* `username`: the root username of the PostgreSQL cluster.
* `password`: the password of root user.
* `port`: the port of the PostgreSQL cluster.
* `host`: the host of the PostgreSQL cluster.
* `endpoint`: the endpoint of the PostgreSQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   postgres

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   h62rg2kl
   ```

2. Exec into the Pod `mycluster-postgresql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

   root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/mycluster-postgresql 5432:5432 
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
