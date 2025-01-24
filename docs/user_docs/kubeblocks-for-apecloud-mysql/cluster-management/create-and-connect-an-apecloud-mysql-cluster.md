---
title: Create and connect to an ApeCloud MySQL Cluster
description: How to create and connect to an ApeCloud MySQL cluster
keywords: [mysql, create an apecloud mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to an ApeCloud MySQL cluster

This guide describe how to create and connect to an ApeCloud MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) if you want to create and connect a MySQL cluster by `kubectl`.
* [Install kbcli](./../../installation/install-kbcli.md) if you want to create and connect a MySQL cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Check whether the ApeCloud MySQL Addon is enabled. The ApeCloud MySQL Addon is enabled by KubeBlocks by default. If you disable it when installing KubeBlocks, [enable it](./../../installation/install-addons.md) first.
  
  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io apecloud-mysql
  >
  NAME             TYPE   VERSION         PROVIDER   STATUS    AGE
  apecloud-mysql   Helm   1.0.0-alpha.0   apecloud   Enabled   10m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">
  
  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  apecloud-mysql                 1.0.0-alpha.0   apecloud    Enabled    true
  ...
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating an ApeCloud MySQL cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>
  
  Make sure the `apecloud-mysql` compoennt definition is installed.

  ```bash
  kubectl get componentdefinition | grep apecloud-mysql
  >
  NAME                                SERVICE            SERVICE-VERSION   STATUS      AGE
  apecloud-mysql-1.0.0                mysql              8.0.30            Available   13m
  ```

  View the available component version.

  ```bash
  kubectl get componentversions | grep apecloud-mysql
  >
  NAME               VERSIONS                           STATUS      AGE
  apecloud-mysql     8.0.30                             Available   15m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  Make sure the `apecloud-mysql` compoennt definition is installed.

  ```bash
  kbcli componentdefinition list | grep apecloud-mysql
  >
  NAME                                      SERVICE            SERVICE-VERSION   PROVIDER        STATUS      CREATED-TIME
  apecloud-mysql-1.0.0-alpha.0              mysql              8.0.30            kubeblocks.io   Available   Jan 20,2025 10:41 UTC+0800
  ```

  View the available component version.

  ```bash
  kbcli componentversion list | grep apecloud-mysql
  >
  NAME               VERSIONS                           STATUS      AGE
  apecloud-mysql     8.0.30                             Available   17m
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports two types of ApeCloud MySQL clusters: Standalone and RaftGroup Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability.

For scenarios with high availability requirements, it is recommended to create a RaftGroup Cluster, which creates a cluster with three replicas. To ensure high availability, all replicas are distributed on different nodes by default.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. Create an ApeCloud MySQL cluster.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a RaftGroup Cluster.

   If you only have one node for deploying a RaftGroup Cluster, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: apecloud-mysql
     topology: apecloud-mysql
     componentSpecs:
       - name: mysql
         serviceVersion: "8.0.30"
         disableExporter: false
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
               storageClassName: "<your-preferred-sc>"
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
   EOF
   ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, and `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-mysql-cluster.md#termination-policy). |
   | `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be `apecloud-mysql` to create an ApeCloud MySQL Cluster. |
   | `spec.topology` | It specifies the name of the ClusterTopology to be used when creating a Cluster. |
   | `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. The valid option for ApeCloud MySQL is 8.0.30. |
   | `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are `true` and `false`. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. ApeCloud-MySQL prefers ODD numbers like 1, 3, 5, 7. |
   | `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
   | `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
   | `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

   :::note

   For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

   :::

   KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster by running the command below.

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   Run the following command to view the details of the created ApeCloud MySQL cluster:

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
   mycluster   apecloud-mysql       Delete               Running   40s
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Create an ApeCloud MySQL cluster.

   Below are some common examples to create a cluster with default settings. If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.
  
   ```bash
   kbcli cluster create apecloud-mysql --help

   kbcli cluster create apecloud-mysql -h
   ```

   Create a Standalone.

   ```bash
   kbcli cluster create apecloud-mysql mycluster --mode='standalone' --namespace demo
   ```

   Create a RaftGroup Cluster.

   ```bash
   kbcli cluster create apecloud-mysql mycluster --mode='raftGroup' --namespace demo
   ```

   If you only have one node for deploying a RaftGroup Cluster, you can configure the cluster affinity by setting `--pod-anti-affinity`, `--tolerations`, and `--topology-keys` when creating a RaftGroup Cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability. For example,

   ```bash
   kbcli cluster create apecloud-mysql mycluster \
       --mode='raftGroup' \
       --pod-anti-affinity='Preferred' \
       --tolerations='node-role.kubeblocks.io/data-plane:NoSchedule' \
       --topology-keys='null' \
       --namespace demo
   ```

2. View the cluster status. If the status is `Running`, this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        apecloud-mysql       Delete               Running   Jan 20,2025 16:27 UTC+0800
   ```

</TabItem>

</Tabs>

## Connect to an ApeCloud MySQL Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

1. Retrive the Secret created by KubeBlocks operator.

   ```bash
   kubectl get secrets -n demo
   >
   NAME                                       TYPE     DATA   AGE
   mycluster-mysql-account-kbadmin            Opaque   2      13s
   mycluster-mysql-account-kbdataprotection   Opaque   2      13s
   mycluster-mysql-account-kbmonitoring       Opaque   2      13s
   mycluster-mysql-account-kbprobe            Opaque   2      13s
   mycluster-mysql-account-kbreplicator       Opaque   2      13s
   mycluster-mysql-account-root               Opaque   2      12s
   ```

   This Secret stores the connection credential of the ApeCloud MySQL cluster. This Secret contains the following keys:

   * `username`: the root username of the MySQL cluster.
   * `password`: the password of the root user.
   * `port`: the port of the MySQL cluster.
   * `host`: the host of the MySQL cluster.
   * `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

2. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-mysql-account-root -o jsonpath='{.data.username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-mysql-account-root -o jsonpath='{.data.password}' | base64 -d
   >
   2gvztbvz
   ```

3. Exec into the Pod `mycluster-mysql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash

   mysql -uroot -p2gvztbvz
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to a database from your local machine.

1. Retrive the Secret created by KubeBlocks operator.

   ```bash
   kubectl get secrets -n demo
   >
   NAME                                       TYPE     DATA   AGE
   mycluster-mysql-account-kbadmin            Opaque   2      13s
   mycluster-mysql-account-kbdataprotection   Opaque   2      13s
   mycluster-mysql-account-kbmonitoring       Opaque   2      13s
   mycluster-mysql-account-kbprobe            Opaque   2      13s
   mycluster-mysql-account-kbreplicator       Opaque   2      13s
   mycluster-mysql-account-root               Opaque   2      12s
   ```

   This Secret stores the connection credential of the ApeCloud MySQL cluster. This Secret contains the following keys:

   * `username`: the root username of the MySQL cluster.
   * `password`: the password of the root user.
   * `port`: the port of the MySQL cluster.
   * `host`: the host of the MySQL cluster.
   * `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

2. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-mysql-account-root -o jsonpath='{.data.username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-mysql-account-root -o jsonpath='{.data.password}' | base64 -d
   >
   2gvztbvz
   ```

3. Run the following command to port forward the service.

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo
   ```

4. Open a new terminal and run the following command to connect to the database.

   ```bash
   mysql -uroot -p2gvztbvz
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster  -n demo
```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
