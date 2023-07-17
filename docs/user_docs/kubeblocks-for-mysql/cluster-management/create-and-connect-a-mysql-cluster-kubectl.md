---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MySQL cluster

This document shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the ApeCloud MySQL addon is installed with `kubectl get addon apecloud-mysql`.
  
  ```bash
  $ kubectl get addon apecloud-mysql
  NAME             TYPE   STATUS    AGE
  apecloud-mysql   Helm   Enabled   61s
  ```

* Make sure the `apecloud-mysql` cluster definition is installed with `kubectl get clusterdefinition apecloud-mysql`.

  ```bash
  $ kubectl get clusterdefinition apecloud-mysql
  NAME             MAIN-COMPONENT-NAME   STATUS      AGE
  apecloud-mysql   mysql                 Available   85m
  ```

* View all available versions for creating a cluster.

  ```bash
  $ kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=apecloud-mysql
  NAME              CLUSTER-DEFINITION   STATUS      AGE
  ac-mysql-8.0.30   apecloud-mysql       Available   11m
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  $ kubectl create namespace demo
  namespace/demo created
  ```

### Create a cluster

KubeBlocks implements a `Cluster` CRD to define a cluster. Below is the command to create a MySQL cluster.

   ```bash
   $ cat <<EOF | kubectl apply -f -
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

* `spec.clusterDefinitionRef` is the name of the cluster definition CRD that define the cluster components.
* `spec.clusterVersionRef` is the name of the cluster version CRD that define the cluster version.
* `spec.componentSpecs` is the list of components that define the cluster components. 
* `spec.componnetSpecs.componentDefRef` is the name of the component definition that defined in the cluster definition, you can get the component definition names with `kubectl get clusterdefinition apecloud-mysql -o json | jq '.spec.componentDefs[].name'`
* `spec.componentSpecs.name` is the name of the component.
* `spec.componentSpecs.replicas` is the number of replicas of the component.
* `spec.componentSpecs.resources` is the resource requirements of the component.
* `spec.componentSpecs.volumeClaimTemplates` is the list of volume claim templates that define the volume claim templates for the component.
* `spec.terminationPolicy` is the policy of the cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`. `DoNotTerminate` will block delete operation. `Halt` will delete workload resources such as statefulset, deployment workloads but keep PVCs. `Delete` is based on Halt and deletes PVCs. `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from backup storage location.

KubeBlocks operator watches for the `Cluster` CRD, creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mysql-cluster -n demo`.

```bash

```

Run the following command to see the modified MySQL cluster object:
```bash
$ kubectl get cluster mysql-cluster -o yaml
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

KubeBlocks operator has created a new Secret called `mysql-cluster-conn-credential` to store the connection credential of the MySQL cluster. This secret contains following keys:
* `username`: the root username of the MySQL cluster.
* `password`: the password of root user.
* `port`: the port of the MySQL cluster.
* `host`: the host of the MySQL cluster.
* `endpoint`: the endpoint of the MySQL cluster, it is the same as `host:port`.

We need `username` and `password` to connect to this MySQL cluster from `kubectl exec` command.

```bash
$ kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
root

$ kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
2gvztbvz
```

Now, we can exec into the pod `mysql-cluster-mysql-0` and connect to the database using username and password.

```bash
$ kubectl exec -ti mysql-cluster-mysql-0 -- bash

[root@mysql-cluster-mysql-0 /]# mysql -uroot -p2gvztbvz
mysql: [Warning] Using a password on the command line interface can be insecure.
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 232
Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision f80d546

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> show databases;
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mydb               |
| mysql              |
| performance_schema |
| sys                |
+--------------------+
5 rows in set (0.02 sec)
```

You can also port forward the service to connect to the database from your local machine. Running the following command to port forward the service:

```bash
$ kubectl port-forward svc/mysql-cluster-mysql 3306:3306 -n demo
```

Open a new terminal and run the following command to connect to the database:

```bash
$ mysql -uroot -p2gvztbvz
```