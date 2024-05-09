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

For your better high-availability experience, KubeBlocks creates a Redis Replication Cluster by default.

## Create a Redis cluster

### Before you start

* [Install KubeBlocks](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure the Redis add-on is enabled.

  ```bash
  kubectl get addons.extensions.kubeblocks.io redis
  >            
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  redis   Helm                        Enabled   16d
  ```

* View all the database types and versions available for creating a cluster.

  Make sure the `redis` cluster definition is installed with `kubectl get clusterdefinitions redis`.

  ```bash
  kubectl get clusterdefinition Redis
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  redis                              Available   16d
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   96m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### Create a cluster

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover. And to ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<<<<<<< HEAD
<TabItem value="kbcli" label="kbcli" default>

Create a Standalone.

```bash
kbcli cluster create redis --mode standalone <clustername>
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

=======
>>>>>>> 25dfea9eb (docs: update redis create docs)
<TabItem value="kubectl" label="kubectl">

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Standalone.

  ```bash
  cat <<EOF | kubectl apply -f -
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: mycluster
    namespace: demo
    labels: 
      helm.sh/chart: redis-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "7.0.6"
      app.kubernetes.io/instance: mycluster
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

For the details of different parameters, you can refer to API docs.

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
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mycluster","app.kubernetes.io/version":"7.0.6","helm.sh/chart":"redis-cluster-0.6.0-alpha.36"},"name":"mycluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"redis","clusterVersionRef":"redis-7.0.6","componentSpecs":[{"componentDefRef":"redis","enabledLogs":["running"],"monitor":false,"name":"redis","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-redis","services":null,"switchPolicy":{"type":"Noop"},"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T08:33:48Z"
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
          pod: mycluster-redis-0
  conditions:
  - lastTransitionTime: "2023-07-19T08:33:48Z"
    message: 'The operator has started the provisioning of Cluster: mycluster'
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
    message: 'Cluster: mycluster is ready, current phase is Running'
    reason: ClusterReady
    status: "True"
    type: Ready
  observedGeneration: 1
  phase: Running
```

</details>

</TabItem>

<TabItem value="helm" label="Helm">

This tutorial takes creating a Redis cluster from the addon repository cloned from the [KubeBlocks addons repository](https://github.com/apecloud/kubeblocks-addons/tree/main) as an example.

1. Clone the KubeBlocks addon repository.

    ```bash
    git clone https://github.com/apecloud/kubeblocks-addons.git
    ```

 2. (Optional) If you want to create a cluster with custom specifications, you can view the values available for configuring.

    ```bash
    helm show values ./addons/redis
    ```

 3. Create a Redis cluster.

    ```bash
    helm install mycluster ./addons/redis-cluster --namespace=demo
    ```

    If you need to customize the cluster specifications, use `--set` to specify the parameters. But only part of the parameters can be customized and you can view these parameters by running the command in step 2.

    ```bash
    helm install mycluster ./addons/redis-cluster --namespace=demo --set cpu=4,mode=replication
    ```

 4. Verify whether this cluster is created successfully.


    ```bash
    helm list -n demo
    >
    NAME   	   NAMESPACE	 REVISION	  UPDATED                             	STATUS  	CHART              	APP VERSION
    mycluster   demo     	 1       	  2024-04-18 15:23:44.063714 +0800 CST	deployed	redis-cluster-0.9.0	7.0.6
    ```

    ```bash
    kubectl get cluster -n demo
    >
    NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
    mycluster   redis                redis-7.0.6    Delete               Running   5m34s
    ```

</TabItem>

</Tabs>

## Connect to a Redis Cluster

<Tabs>

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

2. Exec into the pod `mycluster-redis-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-redis-0 -- bash

   root@mycluster-redis-0:/# redis-cli -a p7twmbrd  --user default
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
   root@mycluster-redis-0:/# redis-cli -a p7twmbrd  --user default
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
