---
title: High Availability for Redis
description: High availability for a Redis cluster
keywords: [redis, high availability]
sidebar_position: 1
---

# High availability

KubeBlocks integrates [the official Redis Sentinel solution](https://redis.io/docs/management/sentinel/) to realize high availability and adopts Noop as the switch policy.

Redis Sentinel is the high availability solution for a Redis Replication Cluster, which is recommended by Redis and is also the main-stream solution in the community.

In the RedisReplication Cluster provided by KubeBlocks, Sentinel is deployed as an independent component.

## Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* [Create a Redis Replication Cluster](./../cluster-management/create-and-connect-a-redis-cluster.md#create-a-redis-cluster).
* Check the role probe.
  * Check whether the following role probe parameters exist to verify the role probe is enabled.

    ```bash
    kubectl get cd redis -n demo -o yaml
    >
    probes:
      roleProbe:
        failureThreshold: 2
        periodSeconds: 2
        timeoutSeconds: 1
    ```

## Steps

1. View the initial status of the Redis cluster.

    ```bash
    kubectl get pods -l kubeblocks.io/role=primary -n demo
    >
    NAME                READY   STATUS    RESTARTS   AGE
    mycluster-redis-0   3/3     Running   0          24m

    kubectl get pods -l kubeblocks.io/role=secondary -n demo
    >
    NAME                READY   STATUS    RESTARTS      AGE
    mycluster-redis-1   3/3     Running   1 (24m ago)   24m
    ```

   Currently, `mycluster-redis-0` is the primary pod and `mycluster-redis-1` is the secondary pod.

   :::note

   To fetch a more complete output, you can modify the `-o` parameter.

   ```bash
   kubectl get pods  -o custom-columns=NAME:.metadata.name,ROLE_LABEL:.metadata.labels."kubeblocks\.io/role"
   ```

   :::

2. Simulate a primary pod exception.

   ```bash
   # Enter the primary pod
   kubectl exec -ti -n demo mycluster-redis-0 -- bash

   # Execute the debug sleep command to simulate a primary pod exception
   root@mycluster-redis-0:/# redis-cli debug sleep 30
   ```

3. Open the Redis Sentinel log to view the failover.

   ```bash
   kubectl logs mycluster-redis-sentinel-0 -n demo
   ```

   In the logs, we can view when a high-availability switch occurs.

   ```bash
   1:X 18 Apr 2023 06:13:17.072 # +switch-master mycluster-redis-sentinel mycluster-redis-0.mycluster-redis-headless.default.svc 6379 mycluster-redis-1.mycluster-redis-headless.default.svc 6379
   1:X 18 Apr 2023 06:13:17.074 * +slave slave mycluster-redis-0.mycluster-redis-headless.default.svc:6379 mycluster-redis-0.mycluster-redis-headless.default.svc 6379 @ mycluster-redis-sentinel mycluster-redis-1.mycluster-redis-headless.default.svc 6379
   1:X 18 Apr 2023 06:13:17.077 * Sentinel new configuration saved on disk
   ```

4. Connect to the Redis cluster to view the primary pod information after the exception simulation.

    ```bash
    127.0.0.1:6379> info replication
    ```

   Now `mycluster-redis-1` has been assigned as the primary's pod.

5. Describe the cluster and check the instance role.

   ```bash
   kubectl get pods -l kubeblocks.io/role=primary -n demo
   kubectl get pods -l kubeblocks.io/role=secondary -n demo
   ```

   After the failover, `mycluster-redis-0` becomes the secondary pod and `mycluster-redis-1` becomes the primary pod.
