---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [postgresql, create a postgresql cluster, connect to a postgresql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a PostgreSQL cluster

This tutorial shows how to create and connect to a PostgreSQL cluster.

## Create a PostgreSQL cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to create and connect a cluster by kbcli.
* Install KubeBlocks: You can install KubeBlocks by [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the PostgreSQL add-on is enabled.
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>
  
  ```bash
  kbcli addon list
  >
  NAME                       TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  postgresql                 Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io postgresql
  >
  NAME         TYPE   STATUS    AGE
  postgresql   Helm   Enabled   23m
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
  
  Make sure the `postgresql` cluster definition is installed with `kubectl get clusterdefinitions postgresql`.

  ```bash
  kubectl get clusterdefinition postgresql
  >
  NAME         MAIN-COMPONENT-NAME   STATUS      AGE
  postgresql   postgresql            Available   25m
  ```

  View all available versions for creating a cluster

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=postgresql
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of PostgreSQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with a Replication Cluster to support automatic failover. And to ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Create a Standalone.

```bash
kbcli cluster create postgresql <clustername>
```

Create a Replication Cluster.

```bash
kbcli cluster create postgresql --mode replication <clustername>
```

If you only have one node for deploying a Replication, set the `availability-policy` as `none` when creating a Replication Cluster.

```bash
kbcli cluster create postgresql --mode replication --availability-policy none <clustername>
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease cluster availability.
* Run the command below to view the flags for creating a PostgreSQL cluster and the default values.
  
  ```bash
  kbcli cluster create postgresql -h
  ```

:::

</TabItem>

<TabItem value="kubectl" label="kubectl">

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Standalone.

  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: pg-cluster
    namespace: demo
    labels: 
      helm.sh/chart: postgresql-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "14.8.0"
      app.kubernetes.io/instance: pg
  spec:
    clusterVersionRef: postgresql-14.8.0
    terminationPolicy: Delete  
    affinity:
      podAntiAffinity: Preferred
      topologyKeys:
        - kubernetes.io/hostname
      tenancy: SharedNode
    clusterDefinitionRef: postgresql
    componentSpecs:
      - name: postgresql
        componentDefRef: postgresql      
        monitor: false      
        replicas: 1
        enabledLogs:
          - running
        serviceAccountName: kb-pg
        switchPolicy:
          type: Noop      
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
* `spec.componentSpecs.componentDefRef` is the name of the component definition that is defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition postgresql -o json | jq '.spec.componentDefs[].name'`.
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` blocks delete operation. `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from the backup storage location.

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=pg-cluster -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=pg-cluster -n demo
```

Run the following command to see the created PostgreSQL cluster object:

```bash
kubectl get cluster pg-cluster -n demo -o yaml
```

<details>

<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"pg","app.kubernetes.io/version":"14.8.0","helm.sh/chart":"postgresql-cluster-0.6.0-alpha.36"},"name":"pg-cluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"postgresql","clusterVersionRef":"postgresql-14.8.0","componentSpecs":[{"componentDefRef":"postgresql","enabledLogs":["running"],"monitor":false,"name":"postgresql","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-pg","services":null,"switchPolicy":{"type":"Noop"},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T07:53:07Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: pg
    app.kubernetes.io/version: 14.8.0
    clusterdefinition.kubeblocks.io/name: postgresql
    clusterversion.kubeblocks.io/name: postgresql-14.8.0
    helm.sh/chart: postgresql-cluster-0.6.0-alpha.36
  name: pg-cluster
  namespace: demo
  resourceVersion: "8618"
  uid: c9f73d21-b79b-4956-aad0-a4e677cb8ba1
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-14.8.0
  componentSpecs:
  - componentDefRef: postgresql
    enabledLogs:
    - running
    monitor: false
    name: postgresql
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-pg
    switchPolicy:
      type: Noop
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
    postgresql:
      phase: Running
      podsReady: true
      podsReadyTime: "2023-07-19T07:53:43Z"
      replicationSetStatus:
        primary:
          pod: pg-cluster-postgresql-0
  conditions:
  - lastTransitionTime: "2023-07-19T07:53:07Z"
    message: 'The operator has started the provisioning of Cluster: pg-cluster'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-19T07:53:07Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-19T07:53:43Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-19T07:53:43Z"
    message: 'Cluster: pg-cluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

</Tabs>

## Connect to a PostgreSQL Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `pg-cluster-conn-credential` to store the connection credential of the `pg-cluster` cluster. This secret contains following keys:

* `username`: the root username of the PostgreSQL cluster.
* `password`: the password of root user.
* `port`: the port of the PostgreSQL cluster.
* `host`: the host of the PostgreSQL cluster.
* `endpoint`: the endpoint of the PostgreSQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo pg-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   postgres

   kubectl get secrets -n demo pg-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   h62rg2kl
   ```

2. Exec into the Pod `pg-cluster-postgresql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo pg-cluster-postgresql-0 -- bash

   root@pg-cluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/pg-cluster-postgresql 5432:5432 
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@pg-cluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
