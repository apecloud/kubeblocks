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

This tutorial shows how to create and connect to an ApeCloud MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md) if you want to create and connect a MySQL cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Check whether the ApeCloud MySQL Addon is enabled. The ApeCloud MySQL Addon is enabled by KubeBlocks by default. If you disable it when installing KubeBlocks,[enable it](./../../installation/install-addons.md) first.
  
  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io apecloud-mysql
  >
  NAME             TYPE   VERSION   PROVIDER   STATUS    AGE
  apecloud-mysql   Helm                        Enabled   61m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">
  
  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  apecloud-mysql                 0.9.0           apecloud    Enabled    true
  ...
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>
  
  Make sure the `apecloud-mysql` cluster definition is installed.

  ```bash
  kubectl get clusterdefinition apecloud-mysql
  >
  NAME             TOPOLOGIES   SERVICEREFS   STATUS      AGE
  apecloud-mysql                              Available   85m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=apecloud-mysql
  >
  NAME                CLUSTER-DEFINITION   STATUS      AGE
  ac-mysql-8.0.30     apecloud-mysql       Available   85m
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
  ```

### Create a cluster

KubeBlocks supports creating two types of ApeCloud MySQL clusters: Standalone and RaftGroup Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a RaftGroup Cluster, which creates a cluster with three replicas. To ensure high availability, all replicas are distributed on different nodes by default.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. Create an ApeCloud MySQL cluster.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a RaftGroup Cluster.

   If you only have one node for deploying a RaftGroup Cluster, set `spec.affinity.topologyKeys` as `null`. But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     componentSpecs:
     - name: mysql
       componentDef: apecloud-mysql
       affinity:
         podAntiAffinity: Preferred
         topologyKeys:
         - kubernetes.io/hostname
         tenancy: SharedNode
       tolerations:
       - key: kb-data
         operator: Equal
         value: 'true'
         effect: NoSchedule
       enabledLogs:
       - error
       - general
       - slow
       disableExporter: true
       serviceAccountName: kb-acmysql-cluster
       replicas: 2
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
   | `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-mysql-cluster.md#termination-policy). |
   | `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
   | `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
   | `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
   | `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
   | `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition apecloud-mysql -o json \| jq '.spec.componentDefs[].name'`.   |
   | `spec.componentSpecs.name`            | It specifies the name of the component.     |
   | `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
   | `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

   KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster by running the command below.

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   Run the following command to view the details of the created ApeCloud MySQL cluster:

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

1. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
   mycluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   12m
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
   kbcli cluster create mycluster --cluster-definition apecloud-mysql --namespace demo
   ```

   Create a RaftGroup Cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition apecloud-mysql --set replicas=3 --namespace demo
   ```

   If you only have one node for deploying a RaftGroup Cluster, set the `topology-keys` as `null` when creating a RaftGroup Cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```bash
   kbcli cluster create mycluster --cluster-definition apecloud-mysql --set replicas=3 --topology-keys null --namespace demo
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Sep 19,2024 16:01 UTC+0800
   ```

</TabItem>

</Tabs>

## Connect to an ApeCloud MySQL Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator creates a new Secret called `mycluster-conn-credential` to store the connection credential of the ApeCloud MySQL cluster. This secret contains the following keys:

* `username`: the root username of the MySQL cluster.
* `password`: the password of the root user.
* `port`: the port of the MySQL cluster.
* `host`: the host of the MySQL cluster.
* `endpoint`: the endpoint of the MySQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
   >
   2gvztbvz
   ```

2. Exec into the Pod `mycluster-mysql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash

   mysql -uroot -p2gvztbvz
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
