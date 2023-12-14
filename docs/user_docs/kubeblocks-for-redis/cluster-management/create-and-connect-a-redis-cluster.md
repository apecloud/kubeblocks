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

KuebBlocks for Redis supports Standalone clusters and Replication Cluster.

But for your better high-availability experience, KubeBlocks creates a Redis Replication Cluster by default.

## Create a Redis cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to create and connect a cluster by kbcli.
* Install KubeBlocks: You can install KubeBlocks by [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the Redis add-on is enabled.
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>
  
  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  redis                     Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io redis
  >            
  NAME    TYPE   STATUS    AGE
  redis   Helm   Enabled   96m
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

  Make sure the `redis` cluster definition is installed with `kubectl get clusterdefinitions redis`.

  ```bash
  kubectl get clusterdefinition Redis
  >
  NAME    MAIN-COMPONENT-NAME   STATUS      AGE
  redis   redis                 Available   96m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   96m
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

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover. And to ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Create a Standalone.

```bash
kbcli cluster create redis <clustername>
```

Create a Replication Cluster.

```bash
kbcli cluster create redis --mode replication <clustername>
```

If you only have one node for deploying a Replication, set the `availability-policy` as `none` when creating a Replication Cluster.

```bash
kbcli cluster create redis --mode replication --availability-policy none <clustername>
```

:::note

* In the production environment, it is not recommended to deploy all replicas on one node, which may decrease cluster availability.
* Run the command below to view the flags for creating a Redis cluster and the default values.
  
  ```bash
  kbcli cluster create redis -h
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
    name: redis
    namespace: demo
    labels: 
      helm.sh/chart: redis-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "7.0.6"
      app.kubernetes.io/instance: redis
  spec:
    clusterVersionRef: redis-7.0.6
    terminationPolicy: Delete  
    affinity:
      podAntiAffinity: Preferred
      topologyKeys:
        - kubernetes.io/hostname
      tenancy: SharedNode
    clusterDefinitionRef: redis  # ref clusterDefinition.name
    componentSpecs:
      - name: redis
        componentDefRef: redis # ref clusterDefinition componentDefs.name      
        monitor: false      
        replicas: 1
        enabledLogs:
          - running
        serviceAccountName: kb-redis
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
* `spec.componentSpecs.componentDefRef` is the name of the component definition that is defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition redis -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=redis -n demo`.

```bash
kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=redis -n demo
```

Run the following command to see the created Redis cluster object:

```bash
kubectl get cluster redis -n demo -o yaml
```

<details>

<summary>Output</summary>

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"redis","app.kubernetes.io/version":"7.0.6","helm.sh/chart":"redis-cluster-0.6.0-alpha.36"},"name":"redis","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"redis","clusterVersionRef":"redis-7.0.6","componentSpecs":[{"componentDefRef":"redis","enabledLogs":["running"],"monitor":false,"name":"redis","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-redis","services":null,"switchPolicy":{"type":"Noop"},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T08:33:48Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: redis
    app.kubernetes.io/version: 7.0.6
    clusterdefinition.kubeblocks.io/name: redis
    clusterversion.kubeblocks.io/name: redis-7.0.6
    helm.sh/chart: redis-cluster-0.6.0-alpha.36
  name: redis
  namespace: demo
  resourceVersion: "12967"
  uid: 25ae9193-60ae-4521-88eb-70ea4c3d97ef
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  componentSpecs:
  - componentDefRef: redis
    enabledLogs:
    - running
    monitor: false
    name: redis
    noCreatePDB: false
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-redis
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
    redis:
      phase: Running
      podsReady: true
      podsReadyTime: "2023-07-19T08:34:34Z"
      replicationSetStatus:
        primary:
          pod: redis-redis-0
  conditions:
  - lastTransitionTime: "2023-07-19T08:33:48Z"
    message: 'The operator has started the provisioning of Cluster: redis'
    observedGeneration: 1
    reason: PreCheckSucceed
    status: "True"
    type: ProvisioningStarted
  - lastTransitionTime: "2023-07-19T08:33:48Z"
    message: Successfully applied for resources
    observedGeneration: 1
    reason: ApplyResourcesSucceed
    status: "True"
    type: ApplyResources
  - lastTransitionTime: "2023-07-19T08:34:34Z"
    message: all pods of components are ready, waiting for the probe detection successful
    reason: AllReplicasReady
    status: "True"
    type: ReplicasReady
  - lastTransitionTime: "2023-07-19T08:34:34Z"
    message: 'Cluster: redis is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

</Tabs>

## Connect to a Redis Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `redis-conn-credential` to store the connection credential of the Redis cluster. This secret contains the following keys:

* `username`: the root username of the Redis cluster.
* `password`: the password of the root user.
* `port`: the port of the Redis cluster.
* `host`: the host of the Redis cluster.
* `endpoint`: the endpoint of the Redis cluster and it is the same as `host:port`.

1. Get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   default

   kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   p7twmbrd
   ```

2. Exec into the pod `redis-redis-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo redis-redis-0 -- bash

   root@redis-redis-0:/# redis-cli -a p7twmbrd  --user default
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.
  
   ```bash
   kubectl port-forward -n demo svc/redis-redis 6379:6379
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@redis-redis-0:/# redis-cli -a p7twmbrd  --user default
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
