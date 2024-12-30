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

* [Install kbcli](./../../installation/install-kbcli.md) if you want to manage the PostgreSQL cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the PostgreSQL Addon is enabled. The PostgreSQL Addon is installed and enabled  by KubeBlocks by default. But if you disable it when installing KubeBlocks, [enable it](./../../installation/install-addons.md) first.
  
  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io postgresql
  >
  NAME         TOPOLOGIES   SERVICEREFS   STATUS      AGE
  postgresql                              Available   30m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                       TYPE   STATUS     EXTRAS         AUTO-INSTALL   
  ...
  postgresql                 Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>
  
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

KubeBlocks supports creating two types of PostgreSQL clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which creates a cluster with a Replication Cluster to support automatic failover. To ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. Create a PostgreSQL cluster.

   KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Replication.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: postgresql
     topology: replication
     componentSpecs:
       - name: postgresql
         serviceVersion: "14.7.2"
         disableExporter: false
         labels:
           # Note: DO NOT REMOVE THIS LABEL
           apps.kubeblocks.postgres.patroni/scope: mycluster-postgresql
         replicas: 2
         resources:
           limits:
             cpu: "0.5"
             memory: "0.5Gi"
           requests:
             cpu: "0.5"
             memory: "0.5Gi"
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
   | `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-a-postgresql-cluster.md#termination-policy). |
   | `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. Note: DO NOT UPDATE THIS FIELD. The value must be `postgresql` to create a PostgreSQL Cluster. |
   | `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. The valid option is [replication]. |
   | `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are: [12.14.0,12.14.1,12.15.0,14.7.2,14.8.0,15.7.0,16.4.0]. |
   | `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are [true, false]. |
   | `spec.componentSpecs.labels` | It specifies Labels to override or add for underlying Pods, PVCs, Account & TLS Secrets, Services owned by Component. |
   | `spec.componentSpecs.labels.apps.kubeblocks.postgres.patroni/scope` | PostgreSQL's `ComponentDefinition` specifies the environment variable `KUBERNETES_SCOPE_LABEL=apps.kubeblocks.postgres.patroni/scope`. This variable defines the label key Patroni uses to tag Kubernetes resources, helping Patroni identify which resources belong to the specified scope (or cluster). **Note**: **DO NOT REMOVE THIS LABEL.** You can update its value to match your cluster name. The value must follow the format `<cluster.metadata.name>-postgresql`. For example, if your cluster name is `mycluster`, the value would be `mycluster-postgresql`. Replace `mycluster` with your actual cluster name as needed.  |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. |
   | `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
   | `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
   | `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

   For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

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

<TabItem value="kbcli" label="kbcli">

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
   kbcli cluster create postgresql mycluster --replicas=2  --topology-keys=null -n demo
   ```

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        postgresql           postgresql-14.8.0   Delete               Running   Sep 28,2024 16:47 UTC+0800
   ```

</TabItem>

</Tabs>

## Connect to a PostgreSQL Cluster

<Tabs>

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
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
   >
   postgres

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
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

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster  --namespace demo
```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
