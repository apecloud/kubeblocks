---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [ postgresql, create a postgresql cluster, connect to a postgresql cluster ]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a PostgreSQL cluster

This document shows how to create and connect to a PostgreSQL cluster.

## Create a PostgreSQL cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the PostgreSQL add-on is installed and enabled
  with `kubectl get addons.extensions.kubeblocks.io postgresql`.

  ```bash
    $ kubectl get addons.extensions.kubeblocks.io postgresql              
    NAME         TYPE   STATUS    AGE
    postgresql   Helm   Enabled   23m
  ```
* Make sure the `postgresql` cluster definition is installed with `kubectl get clusterdefinitions postgresql`.

  ```bash
  $ kubectl get clusterdefinition postgresql
    NAME         MAIN-COMPONENT-NAME   STATUS      AGE
    postgresql   postgresql            Available   25m
  ```

* View all available versions for creating a cluster.

  ```bash
  $ kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=postgresql
    NAME                 CLUSTER-DEFINITION   STATUS      AGE
    postgresql-14.7.2    postgresql           Available   27m
    postgresql-14.8.0    postgresql           Available   27m
    postgresql-12.15.0   postgresql           Available   27m
    postgresql-12.14.1   postgresql           Available   27m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  $ kubectl create namespace demo
  namespace/demo created
  ```

### Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Below is the command to create a PostgreSQL cluster.

  ```bash
  $ cat <<EOF | kubectl apply -f -
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that define the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that define the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components.
* `spec.componnetSpecs.componentDefRef` is the name of the component definition that defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition postgresql -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` will delete workload resources such as statefulset, deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD, creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=pg-cluster -n demo`.

```bash
$ kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=pg-cluster -n demo
NAME                          READY   STATUS    RESTARTS   AGE
pod/pg-cluster-postgresql-0   5/5     Running   0          6m18s

NAME                                     TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                  AGE
service/pg-cluster-postgresql-headless   ClusterIP   None           <none>        5432/TCP,8008/TCP,6432/TCP,9187/TCP,3501/TCP,50001/TCP   6m19s
service/pg-cluster-postgresql            ClusterIP   10.43.69.111   <none>        5432/TCP,6432/TCP                                        6m19s

NAME                                     READY   AGE
statefulset.apps/pg-cluster-postgresql   1/1     6m20s

NAME                                            TYPE     DATA   AGE
secret/pg-cluster-conn-credential               Opaque   5      6m20s
secret/pg-cluster-postgresql-kbreplicator       Opaque   2      5m39s
secret/pg-cluster-postgresql-kbprobe            Opaque   2      5m38s
secret/pg-cluster-postgresql-kbadmin            Opaque   2      5m38s
secret/pg-cluster-postgresql-kbdataprotection   Opaque   2      5m38s
secret/pg-cluster-postgresql-kbmonitoring       Opaque   2      5m37s

NAME                                                  ROLE                                      AGE
rolebinding.rbac.authorization.k8s.io/kb-pg-cluster   ClusterRole/kubeblocks-cluster-pod-role   6m19s

NAME                   SECRETS   AGE
serviceaccount/kb-pg   0         6m20s

```

Run the following command to see the modified PostgreSQL cluster object:
```bash
$ kubectl get cluster pg-cluster -n demo -o yaml
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

## Connect to a PostgreSQL Cluster

KubeBlocks operator has created a new Secret called `pg-cluster-conn-credential` to store the connection credential of the `pg-cluster` cluster. This secret contains following keys:
* `username`: the root username of the PostgreSQL cluster.
* `password`: the password of root user.
* `port`: the port of the PostgreSQL cluster.
* `host`: the host of the PostgreSQL cluster.
* `endpoint`: the endpoint of the PostgreSQL cluster, it is the same as `host:port`.

We need `username` and `password` to connect to this PostgreSQL cluster from `kubectl exec` command.

```bash
$ kubectl get secrets -n demo pg-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
postgres

$ kubectl get secrets -n demo pg-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
h62rg2kl
```

Now, we can exec into the pod `pg-cluster-postgresql-0` and connect to the database using username and password.

```bash
$ kubectl exec -ti -n demo pg-cluster-postgresql-0 -- bash
root@pg-cluster-postgresql-0:/home/postgres# psql -U postgres -W
Password: h62rg2kl
psql (14.8 (Ubuntu 14.8-1.pgdg22.04+1))
Type "help" for help.

postgres=# \dt
            List of relations
 Schema |     Name     | Type  |  Owner   
--------+--------------+-------+----------
 public | postgres_log | table | postgres
(1 row)
```

You can also port forward the service to connect to the database from your local machine. Running the following command to port forward the service:

```bash
$ kubectl port-forward -n demo svc/pg-cluster-postgresql 5432:5432 
```