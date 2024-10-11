---
title: Create and connect to a PostgreSQL Cluster
description: How to create and connect to a PostgreSQL cluster
keywords: [postgresql, create a postgresql cluster, connect to a postgresql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and connect to a PostgreSQL cluster

This tutorial shows how to create and connect to a PostgreSQL cluster.

## Create a PostgreSQL cluster

### Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to manage the PostgreSQL cluster by `kbcli`.
* Install KubeBlocks [by kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../../installation/install-with-helm/install-kubeblocks.md).
* Make sure the PostgreSQL Addon is enabled. The PostgreSQL Addon is installed and enabled  by KubeBlocks by default. But if you disable it when installing KubeBlocks, [enable it](./../../installation/install-with-kbcli/install-addons.md) first.
  
  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                       TYPE   STATUS     EXTRAS         AUTO-INSTALL   
  ...
  postgresql                 Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io postgresql
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  postgresql                              Available   30m
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
  
  ```bash
  kubectl get clusterdefinition postgresql
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  postgresql                              Available   30m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=postgresql
  >
  NAME                 CLUSTER-DEFINITION   STATUS      AGE
  postgresql-12.14.0   postgresql           Available   30m
  postgresql-12.14.1   postgresql           Available   30m
  postgresql-12.15.0   postgresql           Available   30m
  postgresql-14.7.2    postgresql           Available   30m
  postgresql-14.8.0    postgresql           Available   30m
  postgresql-15.7.0    postgresql           Available   30m
  postgresql-16.4.0    postgresql           Available   30m
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

   ```bash
   kubectl create namespace demo
   ```

### Create a cluster

KubeBlocks supports creating two types of PostgreSQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with a Replication Cluster to support automatic failover. To ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Create a PostgreSQL cluster.

   Here is an example of creating a Standalone.

   ```bash
   kbcli cluster create postgresql mycluster -n demo
   ```

   `kbcli` provides various options for you to customize your cluster specifications, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create postgresql --help
   kbcli cluster create postgresql -h
   ```

   For example, you can create a Replication Cluster with the `--replicas` flag.

   ```bash
   kbcli cluster create postgresql mycluster --replicas=2 -n demo
   ```

   If you only have one node for deploying a Replication Cluster, set the `--topology-keys` as `null` when creating a Replication Cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```bash
   kbcli cluster create postgresql mycluster --replicas=2 --availability-policy='none' -n demo
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        postgresql           postgresql-14.8.0   Delete               Running   Sep 28,2024 16:47 UTC+0800
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Create a PostgreSQL cluster.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Replication.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: postgresql
     clusterVersionRef: postgresql-14.8.0
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
     - name: postgresql
       componentDefRef: postgresql
       enabledLogs:
       - running
       disableExporter: true
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
   | `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
   | `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
   | `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p> <p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. |
   | `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
   | `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
   | `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
   | `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
   | `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition postgresql -o json \| jq '.spec.componentDefs[].name'`.   |
   | `spec.componentSpecs.name`            | It specifies the name of the component.     |
   | `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
   | `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

   KubeBlocks operator watches for the `Cluster` CRD and creates the cluster and all dependent resources. You can get all the resources created by the cluster with `kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo`.

   ```bash
   kubectl get all,secret,rolebinding,serviceaccount -l app.kubernetes.io/instance=mycluster -n demo
   ```

   Run the following command to see the created PostgreSQL cluster object:

   ```bash
   kubectl get cluster mycluster -n demo -o yaml
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
   mycluster   postgresql           postgresql-14.8.0   Delete               Running   9m21s
   ```

</TabItem>

</Tabs>

## Connect to a PostgreSQL Cluster

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster connect mycluster  --namespace demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the `mycluster` cluster. This secret contains following keys:

* `username`: the root username of the PostgreSQL cluster.
* `password`: the password of root user.
* `port`: the port of the PostgreSQL cluster.
* `host`: the host of the PostgreSQL cluster.
* `endpoint`: the endpoint of the PostgreSQL cluster and it is the same as `host:port`.

1. Run the command below to get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   postgres

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   h62rg2kl
   ```

2. Exec into the Pod `mycluster-postgresql-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

   root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.

   ```bash
   kubectl port-forward -n demo svc/mycluster-postgresql 5432:5432 
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
   Password: h62rg2kl
   ```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
