---
title: Create and connect to a MongoDB Cluster
description: How to create and connect to a MongoDB cluster
keywords: [mogodb, create a mongodb cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a MongoDB cluster

This tutorial shows how to create and connect to a MongoDB cluster.

## Create a MongoDB cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md) if you want to create and connect a MySQL cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the MongoDB Addon is enabled. If this addon is not enabled, [enable it](./../../installation/install-addons.md) first.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io mongodb
  >
  NAME      TYPE   VERSION   PROVIDER   STATUS    AGE
  mongodb   Helm                        Enabled   23m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   
  ...
  mongodb                        Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get clusterdefinition mongodb
  >
  NAME      TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mongodb                              Available   23m
  ```

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mongodb
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list

  kbcli clusterversion list
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

KubeBlocks supports creating two types of MongoDB clusters: Standalone and ReplicaSet. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a ReplicaSet, which creates a cluster with two replicas to support automatic failover. To ensure high availability, all replicas are distributed on different nodes by default.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. Create a MongoDB Standalone.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a MongoDB Standalone.

   If you only have one node for deploying a ReplicaSet Cluster, set `spec.affinity.topologyKeys` as `null`. But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: mongodb
     topology: replicaset
     componentSpecs:
       - name: mongodb
         serviceVersion: "6.0.16"
         replicas: 3
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
               storageClassName: ""
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
   EOF
   ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-mongodb-cluster.md#termination-policy). |
   | `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be `mongodb` to create a MongoDB Cluste. |
   | `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. |
   | `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are: [4.0.28,4.2.24,4.4.29,5.0.28,6.0.16,7.0.1]. |
   | `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are [true, false]. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. |
   | `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
   | `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
   | `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

   For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

   KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster by running the command below.

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   Run the following command to view the created MongoDB cluster object.

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
   mycluster   mongodb              mongodb-5.0       Delete               Running   12m
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Create a MongoDB cluster.

   ```bash
   kbcli cluster create mongodb mycluster -n demo
   ```

   The commands above are some common examples to create a cluster with default settings. If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create mongodb --help
   kbcli cluster create mongodb -h
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        mongodb              mongodb-5.0       Delete               Running   Sep 20,2024 10:01 UTC+0800
   ```

</TabItem>

</Tabs>

## Connect to a MongoDB Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the MongoDB cluster. This secret contains the following keys:

* `username`: the root username of the MongoDB cluster.
* `password`: the password of the root user.
* `port`: the port of the MongoDB cluster.
* `host`: the host of the MongoDB cluster.
* `endpoint`: the endpoint of the MongoDB cluster and it is the same as `host:port`.

1. Get the `username` and `password` to connect to this MongoDB cluster for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   266zfqx5
   ```

2. Exec into the Pod `mycluster-mongodb-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-mongodb-0 -- bash

   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/mycluster-mongodb 27017:27017  
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-mongodb-0:/# mongo --username root --password 266zfqx5 --authenticationDatabase admin
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster -n demo
```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../../user_docs/connect_database/overview-of-database-connection.md).
