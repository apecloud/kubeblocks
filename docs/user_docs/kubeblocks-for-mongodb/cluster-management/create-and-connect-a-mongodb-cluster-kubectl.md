---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [ mongodb, create a mongodb cluster, connect to a mongodb cluster ]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MongoDB cluster

This document shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the MongoDB add-on is installed and enabled
  with `kubectl get addons.extensions.kubeblocks.io mongodb`.

  ```bash
  $ kubectl get addons.extensions.kubeblocks.io mongodb           
  NAME      TYPE   STATUS    AGE
  mongodb   Helm   Enabled   117m
  ```
* Make sure the `mongodb` cluster definition is installed with `kubectl get clusterdefinitions mongodb`.

  ```bash
  $ kubectl get clusterdefinitions mongodb
  NAME      MAIN-COMPONENT-NAME   STATUS      AGE
  mongodb   mongodb               Available   118m
  ```

* View all available versions for creating a cluster.

  ```bash
  $ kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  NAME             CLUSTER-DEFINITION   STATUS      AGE
  mongodb-5.0.14   mongodb              Available   118m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  $ kubectl create namespace demo
  namespace/demo created
  ```

### Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Below is the command to create a MongoDB cluster.

  ```bash
  $ cat <<EOF | kubectl apply -f -
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: mongodb-cluster
    namespace: demo
    labels: 
      helm.sh/chart: mongodb-cluster-0.6.0-alpha.36
      app.kubernetes.io/version: "5.0.14"
      app.kubernetes.io/instance: mongodb
  spec:
    clusterVersionRef: mongodb-5.0.14
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that define the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that define the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componnetSpecs.componentDefRef` is the name of the component definition that defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition mongodb -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` will delete workload resources such as statefulset, deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD, creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo`.

```bash
$ kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mongodb-cluster -n demo
NAME                            READY   STATUS    RESTARTS   AGE
pod/mongodb-cluster-mongodb-0   3/3     Running   0          108s

NAME                                       TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                                 AGE
service/mongodb-cluster-mongodb-headless   ClusterIP   None            <none>        27017/TCP,9216/TCP,3501/TCP,50001/TCP   108s
service/mongodb-cluster-mongodb            ClusterIP   10.43.234.245   <none>        27017/TCP                               108s

NAME                                       READY   AGE
statefulset.apps/mongodb-cluster-mongodb   1/1     108s

NAME                                     TYPE     DATA   AGE
secret/mongodb-cluster-conn-credential   Opaque   8      108s

NAME                                                       ROLE                                      AGE
rolebinding.rbac.authorization.k8s.io/kb-mongodb-cluster   ClusterRole/kubeblocks-cluster-pod-role   108s

NAME                        SECRETS   AGE
serviceaccount/kb-mongodb   0         108s
```

Run the following command to see the modified MongoDB cluster object:
```bash
$ kubectl get cluster mongodb-cluster -n demo -o yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apps.kubeblocks.io/v1alpha1","kind":"Cluster","metadata":{"annotations":{},"labels":{"app.kubernetes.io/instance":"mongodb","app.kubernetes.io/version":"5.0.14","helm.sh/chart":"mongodb-cluster-0.6.0-alpha.36"},"name":"mongodb-cluster","namespace":"demo"},"spec":{"affinity":{"podAntiAffinity":"Preferred","tenancy":"SharedNode","topologyKeys":["kubernetes.io/hostname"]},"clusterDefinitionRef":"mongodb","clusterVersionRef":"mongodb-5.0.14","componentSpecs":[{"componentDefRef":"mongodb","monitor":false,"name":"mongodb","replicas":1,"resources":{"limits":{"cpu":"0.5","memory":"0.5Gi"},"requests":{"cpu":"0.5","memory":"0.5Gi"}},"serviceAccountName":"kb-mongodb","services":null,"volumeClaimTemplates":[{"name":"data","spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"20Gi"}}}}]}],"terminationPolicy":"Delete"}}
  creationTimestamp: "2023-07-19T08:59:48Z"
  finalizers:
  - cluster.kubeblocks.io/finalizer
  generation: 1
  labels:
    app.kubernetes.io/instance: mongodb
    app.kubernetes.io/version: 5.0.14
    clusterdefinition.kubeblocks.io/name: mongodb
    clusterversion.kubeblocks.io/name: mongodb-5.0.14
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
  clusterVersionRef: mongodb-5.0.14
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

## Connect to a MongoDB Cluster

KubeBlocks operator has created a new Secret called `mongodb-cluster-conn-credential` to store the connection credential of the mongodb cluster. This secret contains following keys:
* `username`: the root username of the MongoDB cluster.
* `password`: the password of root user.
* `port`: the port of the MongoDB cluster.
* `host`: the host of the MongoDB cluster.
* `endpoint`: the endpoint of the MongoDB cluster, it is the same as `host:port`.

We need `username` and `password` to connect to this MongoDB cluster from `kubectl exec` command.

```bash
$ kubectl get secrets -n demo mongodb-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
root

$ kubectl get secrets -n demo mongodb-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
svk9xzqs
```

Now, we can exec into the pod `mongodb-cluster-mongodb-0` and connect to the database using username and password.

```bash
$ kubectl exec -ti -n demo mongodb-cluster-mongodb-0 -- bash
root@mongodb-cluster-mongodb-0:/# mongo --username root --password svk9xzqs --authenticationDatabase admin
mongodb-cluster-mongodb:PRIMARY> show dbs
admin   0.000GB
config  0.000GB
local   0.000GB
```

You can also port forward the service to connect to the database from your local machine. Running the following command to port forward the service:

```bash
$ kubectl port-forward -n demo svc/mongodb-cluster-mongodb 27017:27017  
```