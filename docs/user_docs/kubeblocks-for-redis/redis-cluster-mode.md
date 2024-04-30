---
title: Redis Cluster Mode
description: A brief overview of Redis Cluster Mode and its basic operations
keywords: [redis, redis cluster, feature]
sidebar_position: 1
---

# Redis Cluster Mode

While Redis Sentinel clusters provide excellent failover support, they do not inherently provide data sharding. All data remains on a single Redis instance, limited by its memory and performance capacity. Therefore, it may impact horizontal scalability when dealing with large datasets and high read/write operations.

KubeBlocks now supports Redis Cluster Mode, which not only allows for greater memory distribution but also parallel processing, significantly improving performance for data-intensive operations. A brief overview of Redis Cluster Mode and its basic operations will be elaborated in this documentation.

## What is Redis Cluster Mode

Redis Cluster Mode is a distributed deployment mode of the Redis database used to horizontally scale data storage and improve system availability across multiple nodes.

In Redis Cluster Mode, the cluster manages data through a sharding mechanism and provides capabilities for data replication, failover, and traffic routing among shards. This sharding mechanism enables distributing a large amount of data across different nodes, achieving horizontal scalability and load balancing.

Redis Cluster Mode ensures high availability through master-slave replication. Each master node can have one or more slave nodes that replicate the data from the master and provide read services. In case of a master node failure, a slave node can automatically take over the master's role and continue serving, ensuring failover and fault tolerance.

Redis Cluster Mode also provides communication and data migration mechanisms among cluster nodes. When there are changes in the cluster, such as adding nodes, node failures, or node removal, Redis Cluster automatically performs data migration and resharding to maintain data balance and availability.

## Basic Ops

Below is a brief introduction to the basic operations of Redis Cluster Mode.

### Before you start

* Install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
    Make sure your KubeBlocks, kbcli, and add-on are version 0.9 or above.
* Make sure the add-on is enabled.
* View all the database types and versions available for creating a cluster.

  Make sure the `redis` cluster definition is installed with `kubectl get clusterdefinitions redis`.

  ```bash
  kubectl get componentdefinition redis-cluster
  >
  NAME            SERVICE         SERVICE-VERSION   STATUS      AGE
  redis-cluster   redis-cluster   7.0.6             Available   27h
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

### Create clusters

Since Redis Cluster Mode is based on the latest ShardingSpec API, currently it can only be created by Helm or YAML files using kubectl.

:::note

Redis Cluster Mode requires a minimum of three shards, so the number of shards cannot be less than 3.

:::

#### Using Helm

```bash
# Example 1: Creating a Redis Cluster with three shards, each shard having one replica (one master, one replica)
helm install redisc xxxxx(chart remote address) --set mode=cluster --set redisCluster.shardCount=3 --set replicas=2 -n demo

# Example 2: Creating a Redis Cluster with three shards, each shard having one replica (one master, one replica), and enabling NodePort access
helm install redisc xxxxx(chart remote address) --set mode=cluster --set nodePortEnabled=true --set redisCluster.shardCount=3 --set replicas=2 -n demo
```

#### Using YAML

```bash
kubectl apply -f redis-cluster-example.yaml
```

**Example 1: Redis Cluster without NodePort enabled**

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redisc
  namespace: demo
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterVersionRef: redis-7.0.6
  shardingSpecs:
  - name: shard
    # Specify the number of shards, which cannot be less than 3
    shards: 3
    template:
      componentDef: redis-cluster
      name: redis
      replicas: 2
      # Specify resources for a single shard
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: 7.0.6
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 20Gi
  terminationPolicy: Delete
```

**Example 2: Redis Cluster with NodePort enabled**

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redisc
  namespace: demo
spec:
  affinity:
    podAntiAffinity: Preferred
    tenancy: SharedNode
    topologyKeys:
    - kubernetes.io/hostname
  clusterVersionRef: redis-7.0.6
  shardingSpecs:
  - name: shard
    # Specify the number of shards, which cannot be less than 3
    shards: 3
    template:
      componentDef: redis-cluster
      name: redis
      replicas: 2
      # Specify resources for a single shard
      resources:
        limits:
          cpu: 500m
          memory: 512Mi
        requests:
          cpu: 500m
          memory: 512Mi
      serviceVersion: 7.0.6
      # Enable NodePort
      services:
      - name: redis-advertised
        podService: true
        serviceType: NodePort
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 20Gi
  terminationPolicy: Delete
```

Once the cluster is created, wait until the cluster status change to `running`. Then, run `kbcli cluster describe` to view the roles of nodes in the cluster.

```bash
kbcli cluster describe redisc
Name: redisc         Created Time: Apr 23,2024 21:47 UTC+0800
NAMESPACE   CLUSTER-DEFINITION   VERSION       STATUS    TERMINATION-POLICY
default                          redis-7.0.6   Running   Delete

Topology:
COMPONENT   INSTANCE             ROLE        STATUS    AZ       NODE                                  CREATED-TIME
shard-qst   redisc-shard-qst-0   primary     Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
shard-kr2   redisc-shard-kr2-0   primary     Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
shard-kr2   redisc-shard-kr2-1   secondary   Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
shard-qst   redisc-shard-qst-1   secondary   Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
shard-mv6   redisc-shard-mv6-1   secondary   Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
shard-mv6   redisc-shard-mv6-0   primary     Running   <none>   k3d-k3s-default-server-0/172.18.0.3   Apr 23,2024 21:47 UTC+0800
```

### Connect to clusters

**Option 1: Using SDK**

Redis Cluster Mode requires using specific client SDKs for connection. Currently, there are Redis Cluster client SDK implementations available in different popular programming languages. You can choose the appropriate SDK based on your requirements. For example, Jedis or Lettuce for Java, go-redis for Golang, etc.

Note that when using an SDK, you need to configure the addresses of each node in the Redis Cluster. According to whether NodePort is enabled or not, there are two different configurations:

**Example 1: Redis Cluster without NodePort enabled**

Configure each Pod's headless address.

```bash
redisc-shard-qst-0.redisc-shard-qst-headless:6379
redisc-shard-qst-1.redisc-shard-qst-headless:6379
redisc-shard-kr2-0.redisc-shard-kr2-headless:6379
redisc-shard-kr2-1.redisc-shard-kr2-headless:6379
redisc-shard-mv6-0.redisc-shard-mv6-headless:6379
redisc-shard-mv6-1.redisc-shard-mv6-headless:6379
```

**Example 2: Redis Cluster with NodePort enabled**

Configure each Pod's HostIP and the corresponding NodePort.

```bash
# 172.18.0.3 is the host IP of redisc-shard-qst-0 node, and 32269 is the NodePort mapped to the Redis 6379 server port.
172.18.0.3:32269 
172.18.0.4:32051
172.18.0.5:32650
172.18.0.6:30151
172.18.0.7:31603
172.18.0.8:31718
```

**Option 2: Direct connect (for testings only)**

For simple connectivity verification, you can use kubectl commands to log in to a specific Pod in the cluster and check the cluster's status.

```bash
# Log in to a pod
➜  ~ kubectl exec -it redisc-shard-qst-0 -c redis-cluster -- bash
root@redisc-shard-qst-0:/#

# Check cluster topology
root@redisc-shard-qst-0:/# redis-cli -a $REDIS_DEFAULT_PASSWORD cluster nodes
Warning: Using a password with '-a' or '-u' option on the command line interface may not be safe.
f2f729eb9074d42dc58ba544f35be0b6652134c2 172.18.0.3:32650@31171,redisc-shard-mv6-1.redisc-shard-mv6-headless.default.svc slave 04e309001ce12857558ab721b47ce802e15b84f4 0 1713927855356 3 connected
04e309001ce12857558ab721b47ce802e15b84f4 172.18.0.3:32051@32632,redisc-shard-mv6-0.redisc-shard-mv6-headless.default.svc master - 0 1713927854000 3 connected 12288-16383
11da6bb8875ed4b3787888fec56bf9212e664246 172.18.0.3:32663@32115,redisc-shard-qst-1.redisc-shard-qst-headless.default.svc slave 7f062b0b5fdd0d09ea70920a5ad3612a9d217c9d 0 1713927853848 2 connected
7f062b0b5fdd0d09ea70920a5ad3612a9d217c9d 172.18.0.3:30649@30003,redisc-shard-qst-0.redisc-shard-qst-headless.default.svc myself,master - 0 1713927853000 2 connected 6827-10922
a04da6edbf99faff53b2b94d0fc64f378d27b4aa 172.18.0.3:30151@31667,redisc-shard-kr2-0.redisc-shard-kr2-headless.default.svc master - 0 1713927853000 1 connected 1365-5460
d392d129e27ced8603681cd666f705bfad7ae91a 172.18.0.3:31718@30646,redisc-shard-fm6-1.redisc-shard-fm6-headless.default.svc slave 568bb6ced7f186caddb2b2b7ab560e61ff95438c 0 1713927853036 4 connected
b695dbac46efd9ec24ea608358f92dfd749e8e71 172.18.0.3:31603@30000,redisc-shard-kr2-1.redisc-shard-kr2-headless.default.svc slave a04da6edbf99faff53b2b94d0fc64f378d27b4aa 0 1713927854350 1 connected
568bb6ced7f186caddb2b2b7ab560e61ff95438c 172.18.0.3:32269@31457,redisc-shard-fm6-0.redisc-shard-fm6-headless.default.svc master - 0 1713927853340 4 connected 0-1364 5461-6826 10923-12287
```

### Scale shards

:::note

1. Currently, only scaling up or down one shard node at a time is supported. If you need to scale multiple shards, you need to perform the operations sequentially. This will be optimized in future versions.

2. Scaling shards using kbcli or ops will be supported in future versions.

:::

**Option 1: Using `kubectl patch`**

You can use `kubectl patch` to update the shards field and scale the shards.

```bash
# Scale up to 4 shards
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/shards", "value":4}]' -n demo

# Scale down to 3 shards
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/shards", "value":3}]' -n demo
```

**Option 2: Using `kubectl edit`**

You can use `kubectl edit` to directly edit the cluster YAML and modify the value of `spec.shardingSpecs[0].shards`.

```bash
kubectl edit cluster redisc
```

### Scale replicas

The number of replica applies to all shards.

**Option 1: Using `kubectl patch`**

You can use `kubectl patch` to update the replicas field and scale the shard replicas.

```bash
# Scale to 3 replicas (1 master and 2 replicas per shard)
kubectl patch cluster redisc --type='json' -p='[{"op": "replace", "path": "/spec/shardingSpecs/0/template/replicas", "value": 3}]' -n default
```

**Option 2: Using `kubectl edit`**

You can Use `kubectl edit` to directly edit the cluster YAML and modify the value of `spec.shardingSpecs[0].template.replicas`.

```bash
kubectl edit cluster redisc
```

**Option 3: Using OpsRequest**

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: horizontal-scaling-redisc-
spec:
  clusterRef: redisc
  horizontalScaling:
  - componentName: shard
    isSharding: true
    replicas: 3
  type: HorizontalScaling
```

### Resource scaling/reconfiguration

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: vertical-scaling-redisc-
spec:
  clusterRef: redisc
  verticalScaling:
  - componentName: shard
    isSharding: true
    limits: 
      cpu: 2
      memory: 4Gi
    requests:
      cpu: 2
      memory: 4Gi  
  type: VerticalScaling
```

### Expand volume

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: volume-expand-redisc-
spec:
  clusterRef: redisc
  volumeExpansion:
  - componentName: shard
    isSharding: true
    volumeClaimTemplates:
    - name: data
      storage: 200Gi
  type: VolumeExpansion
```

### Restart

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: restart-redisc-
spec:
  clusterRef: redisc
  restart:
  - componentName: shard
    isSharding: true  
  type: Restart
```

### Stop/Start

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: stop-redisc-
spec:
  clusterRef: redisc
  type: Stop
```

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: start-redisc-
spec:
  clusterRef: redisc
  type: Start
```