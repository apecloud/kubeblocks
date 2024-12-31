---
title: Create and connect to a Redis Cluster
description: How to create and connect to a Redis cluster
keywords: [redis, create a redis cluster, connect to a redis cluster, cluster, redis sentinel]
sidebar_position: 1
sidebar_label: Create and connect
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create and Connect to a Redis cluster

This tutorial shows how to create and connect to a Redis cluster.

## Create a Redis cluster

### Before you start

* [Install kbcli](./../../installation/install-kbcli.md) if you want to create a Redis cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure the Redis Addon is enabled. The Redis Addon is enabled by KubeBlocks by default. If you disable it when installing KubeBlocks, [enable it](./../../installation/install-addons.md) first.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io redis
  >
  NAME      TYPE   VERSION   PROVIDER   STATUS    AGE
  redis     Helm                        Enabled   61m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                      TYPE   STATUS     EXTRAS         AUTO-INSTALL   
  ...
  redis                     Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get clusterdefinition redis
  >
  NAME    TOPOLOGIES                                              SERVICEREFS   STATUS      AGE
  redis   replication,replication-twemproxy,standalone                          Available   16m
  ```

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME          CLUSTER-DEFINITION   STATUS      AGE
  redis-7.0.6   redis                Available   16m
  redis-7.2.4   redis                Available   16m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list
  >
  NAME               TOPOLOGIES                                              SERVICEREFS   STATUS      AGE
  redis              replication,replication-twemproxy,standalone                          Available   16m

  kbcli clusterversion list
  >
  NAME                 CLUSTER-DEFINITION   STATUS      IS-DEFAULT   CREATED-TIME
  redis-7.0.6          redis                Available   false        Sep 27,2024 11:36 UTC+0800
  redis-7.2.4          redis                Available   false        Sep 27,2024 11:36 UTC+0800
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

KubeBlocks supports creating two types of Redis clusters: Standalone and Replication Cluster. Standalone only supports one replica and can be used in scenarios with lower requirements for availability. For scenarios with high availability requirements, it is recommended to create a Replication Cluster, which supports automatic failover. To ensure high availability, Primary and Secondary are distributed on different nodes by default.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

KubeBlocks implements a `Cluster` CRD to define a cluster. Here is an example of creating a Replication cluster. KubeBlocks also supports creating a Redis cluster in other modes. You can refer to the examples provided in the [GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/redis).

If you only have one node for deploying a Replication Cluster, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  terminationPolicy: Delete
  clusterDef: redis
  topology: replication
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      disableExporter: false
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
            storageClassName: ""
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
    - name: redis-sentinel
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
| `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-a-redis-cluster.md#termination-policy). |
| `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be `redis` to create a Redis Cluster. |
| `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. |
| `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are [7.0.6,7.2.4]. |
| `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are [true, false]. |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component. |
| `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
| `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
| `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
| `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
| `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

Run the following command to see the created Redis cluster object:

```bash
kubectl get cluster mycluster -n demo -o yaml
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Create an Redis cluster.

   ```bash
   kbcli cluster create redis mycluster -n demo
   ```

   If you want to customize your cluster specifications, kbcli provides various options, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create redis --help

   kbcli cluster create redis -h
   ```

   If you only have one node for deploying a cluster with multiple replicas, you can configure the cluster affinity by setting `--pod-anti-afffinity`, `--tolerations`, and `--topology-keys` when creating a cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION   TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   demo        redis                          Delete               Running    Sep 29,2024 09:46 UTC+0800
   ```

</TabItem>

</Tabs>

## Connect to a Redis Cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

You can use `kubectl exec` to exec into a Pod and connect to a database.

KubeBlocks operator has created a new Secret called `mycluster-conn-credential` to store the connection credential of the Redis cluster. This secret contains the following keys:

* `username`: the root username of the Redis cluster.
* `password`: the password of the root user.
* `port`: the port of the Redis cluster.
* `host`: the host of the Redis cluster.
* `endpoint`: the endpoint of the Redis cluster and it is the same as `host:port`.

1. Get the `username` and `password` for the `kubectl exec` command.

   ```bash
   kubectl get secrets -n demo mycluster-redis-account-default -o jsonpath='{.data.username}' | base64 -d
   >
   default

   kubectl get secrets -n demo mycluster-redis-account-default -o jsonpath='{.data.password}' | base64 -d
   >
   5bv7czc4
   ```

2. Exec into the pod `mycluster-redis-0` and connect to the database using username and password.

   ```bash
   kubectl exec -ti -n demo mycluster-redis-0 -- bash

   root@mycluster-redis-0:/# redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

<TabItem value="port-forward" label="port-forward">

You can also port forward the service to connect to the database from your local machine.

1. Run the following command to port forward the service.
  
   ```bash
   kubectl port-forward -n demo svc/mycluster-redis 6379:6379
   ```

2. Open a new terminal and run the following command to connect to the database.

   ```bash
   root@mycluster-redis-0:/# redis-cli -a 5bv7czc4  --user default
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster connect mycluster -n demo
```

</TabItem>

</Tabs>

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
