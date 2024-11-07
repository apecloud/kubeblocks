---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a MySQL cluster

This tutorial shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* Install KubeBlocks  [by kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../../installation/install-with-helm/install-kubeblocks.md).
* Make sure the MySQL Addon is enabled. The MySQL Addon is installed and enabled by KubeBlocks by default. If you disable it when installing KubeBlocks, [enable it](./../../installation/install-with-kbcli/install-addons.md) first.

  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  mysql                          0.9.1           community   Enabled    true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io mysql
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  mysql   Helm                        Enabled   27h
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

  Make sure the `mysql` cluster definition is installed.

  ```bash
  kubectl get clusterdefinition mysql
  >
  NAME             TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mysql                                       Available   85m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mysql
  >
  NAME           CLUSTER-DEFINITION   STATUS      AGE
  mysql-5.7.44   mysql                Available   27h
  mysql-8.0.33   mysql                Available   27h
  mysql-8.4.2    mysql                Available   27h
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

### Create a cluster

KubeBlocks supports creating two types of MySQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with two replicas. To ensure high availability, all replicas are distributed on different nodes by default.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Create a MySQL cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition mysql -n demo
   ```

   If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create mysql --help
   kbcli cluster create mysql -h
   ```

   If you only have one node for deploying a Replication Cluster, set the `--topology-keys` as `null` when creating a Cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```bash
   kbcli cluster create mycluster --cluster-definition mysql --topology-keys null -n demo
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        mysql                mysql-8.0.30      Delete               Running   Jul 05,2024 18:46 UTC+0800
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Create a MySQL cluster.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Replication Cluster.

   If you only have one node for deploying a Replication Cluster, set `spec.affinity.topologyKeys` as `null`. But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: mysql
     clusterVersionRef: mysql-8.0.33
     terminationPolicy: Delete
     affinity:
       podAntiAffinity: Preferred
       topologyKeys:
       - kubernetes.io/hostname
     tolerations:
       - key: kb-data
         operator: Equal
         value: 'true'
         effect: NoSchedule
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       enabledLogs:
       - error
       - slow
       disableExporter: true
       replicas: 2
       serviceAccountName: kb-mycluster
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

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
   | `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
   | `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-mysql-cluster.md#termination-policy). |
   | `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
   | `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
   | `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
   | `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
   | `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition mysql -o json \| jq '.spec.componentDefs[].name'`.   |
   | `spec.componentSpecs.name`            | It specifies the name of the component.     |
   | `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
   | `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

   For the details of different parameters, you can refer to API docs.

   Run the following commands to see the created MySQL cluster object:

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
   mycluster   mysql                mysql-8.0.30      Delete               Running   6m53s
   ```

</TabItem>

</Tabs>

## Connect to a MySQL Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect mycluster  --namespace demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator creates a new Secret called `mycluster-conn-credential` to store the connection credential of the MySQL cluster. This secret contains the following keys:

* `username`: the root username of the MySQL cluster.
* `password`: the password of the root user.
* `port`: the port of the MySQL cluster.
* `host`: the host of the MySQL cluster.
* `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets mycluster-conn-credential -n demo -o jsonpath='{.data.username}' | base64 -d
   >
   root
   ```

   ```bash
   kubectl get secrets mycluster-conn-credential -n demo -o jsonpath='{.data.password}' | base64 -d
   >
   b8wvrwlm
   ```

2. Exec into the Pod `mycluster-mysql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti mycluster-mysql-0 -n demo -- bash

   mysql -u root -p b8wvrwlm
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to a database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   mysql -uroot -pb8wvrwlm
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
