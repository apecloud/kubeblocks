---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [ Redis, create a Redis cluster, connect to a Redis cluster ]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a Redis cluster

This document shows how to create and connect to a Redis cluster.

## Create a Redis cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the Redis add-on is installed and enabled
  with `kubectl get addons.extensions.kubeblocks.io redis`.

  ```bash
    $ kubectl get addons.extensions.kubeblocks.io redis              
  NAME    TYPE   STATUS    AGE
  redis   Helm   Enabled   96m
  ```
* Make sure the `redis` cluster definition is installed with `kubectl get clusterdefinitions redis`.

  ```bash
  $ kubectl get clusterdefinition Redis
  NAME    MAIN-COMPONENT-NAME   STATUS      AGE
  redis   redis                 Available   96m
  ```

* View all available versions for creating a cluster.

  ```bash
  $ kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   96m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  $ kubectl create namespace demo
  namespace/demo created
  ```

### Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Below is the command to create a Redis cluster.

  ```bash
  $ cat <<EOF | kubectl apply -f -
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that define the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that define the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componnetSpecs.componentDefRef` is the name of the component definition that defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition redis -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` will delete workload resources such as statefulset, deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD, creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=redis -n demo`.

```bash
$ kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=redis -n demo
NAME                READY   STATUS    RESTARTS   AGE
pod/redis-redis-0   3/3     Running   0          73s

NAME                           TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                                AGE
service/redis-redis-headless   ClusterIP   None          <none>        6379/TCP,9121/TCP,3501/TCP,50001/TCP   73s
service/redis-redis            ClusterIP   10.43.50.62   <none>        6379/TCP                               73s

NAME                           READY   AGE
statefulset.apps/redis-redis   1/1     73s

NAME                               CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
cluster.apps.kubeblocks.io/redis   redis                redis-7.0.6   Delete               Running   73s

NAME                                  TYPE     DATA   AGE
secret/redis-conn-credential          Opaque   5      73s
secret/redis-redis-kbprobe            Opaque   2      23s
secret/redis-redis-kbmonitoring       Opaque   2      22s
secret/redis-redis-kbadmin            Opaque   2      22s
secret/redis-redis-kbdataprotection   Opaque   2      22s

NAME                                             ROLE                                      AGE
rolebinding.rbac.authorization.k8s.io/kb-redis   ClusterRole/kubeblocks-cluster-pod-role   73s

NAME                      SECRETS   AGE
serviceaccount/kb-redis   0         73s
```

Run the following command to see the modified Redis cluster object:
```bash
$ kubectl get cluster redis -n demo -o yaml
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

## Connect to a Redis Cluster

KubeBlocks operator has created a new Secret called `redis-conn-credential` to store the connection credential of the `redis` cluster. This secret contains following keys:
* `username`: the root username of the Redis cluster.
* `password`: the password of root user.
* `port`: the port of the PostgreSQL cluster.
* `host`: the host of the PostgreSQL cluster.
* `endpoint`: the endpoint of the PostgreSQL cluster, it is the same as `host:port`.

We need `username` and `password` to connect to this Redis cluster from `kubectl exec` command.

```bash
$ kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\username}' | base64 -d
default

$ kubectl get secrets -n demo redis-conn-credential -o jsonpath='{.data.\password}' | base64 -d
p7twmbrd
```

Now, we can exec into the pod `redis-redis-0` and connect to the database using username and password.

```bash
$ kubectl exec -ti -n demo redis-redis-0 -- bash
root@redis-redis-0:/# redis-cli -a p7twmbrd  --user default
127.0.0.1:6379> get *
(nil)
```

You can also port forward the service to connect to the database from your local machine. Running the following command to port forward the service:

```bash
kubectl port-forward -n demo svc/redis-redis 6379:6379
```