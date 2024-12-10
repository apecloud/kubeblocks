---
title: High Availability for Redis
description: High availability for a Redis cluster
keywords: [redis, high availability]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# High availability

KubeBlocks integrates [the official Redis Sentinel solution](https://redis.io/docs/management/sentinel/) to realize high availability and adopts Noop as the switch policy.

Redis Sentinel is the high availability solution for a Redis Replication Cluster, which is recommended by Redis and is also the main-stream solution in the community.

In the Redis Replication Cluster provided by KubeBlocks, Sentinel is deployed as an independent component.

## Before you start

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* [Create a Redis Replication Cluster](./../cluster-management/create-and-connect-a-redis-cluster.md#create-a-redis-cluster).
* Check the Switch Policy and the role probe.
  * Check whether the switch policy is `Noop`.

    ```bash
    kubectl get cluster redis-cluster -o yaml
    >
    spec:
      componentSpecs:
      - name: redis
        componentDefRef: redis
        switchPolicy:
          type: Noop
    ```

  * Check whether the following role probe parameters exist to verify the role probe is enabled.

    ```bash
    kubectl get cd redis -o yaml
    >
    probes:
      roleProbe:
        failureThreshold: 2
        periodSeconds: 2
        timeoutSeconds: 1
    ```

## Steps

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

This section takes the cluster `mycluster` in the namespace `demo` as an example.

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

</TabItem>

<TabItem value="kbcli" label="kbcli">

This section takes the cluster `redis-cluster` in the namespace `default` as an example.

1. View the initial status of the Redis cluster.

   ```bash
   kbcli cluster describe redis-cluster
   ```

   ![Redis cluster original status](../../../img/redis-ha-before.png)

   Currently, `redis-cluster-redis-0` is the primary pod and `redis-cluster-redis-1` is the secondary pod.

2. Simulate a primary pod exception.

   ```bash
   # Enter the primary pod
   kubectl exec -it redis-cluster-redis-0  -- bash

   # Execute the debug sleep command to simulate a primary pod exception
   root@redis-redis-0:/# redis-cli debug sleep 30
   ```

3. Open the Redis Sentinel log to view the failover.

   ```bash
   kubectl logs redis-cluster-redis-sentinel-0
   ```

   In the logs, we can view when a high-availability switch occurs.

   ```bash
   1:X 18 Apr 2023 06:13:17.072 # +switch-master redis-cluster-redis-sentinel redis-cluster-redis-0.redis-cluster-redis-headless.default.svc 6379 redis-cluster-redis-1.redis-cluster-redis-headless.default.svc 6379
   1:X 18 Apr 2023 06:13:17.074 * +slave slave redis-cluster-redis-0.redis-cluster-redis-headless.default.svc:6379 redis-cluster-redis-0.redis-cluster-redis-headless.default.svc 6379 @ redis-cluster-redis-sentinel redis-cluster-redis-1.redis-cluster-redis-headless.default.svc 6379
   1:X 18 Apr 2023 06:13:17.077 * Sentinel new configuration saved on disk
   ```

4. Connect to the Redis cluster to view the primary pod information after the exception simulation.

   ```bash
   kbcli cluster connect redis-cluster
   ```

   ```bash
   # View the current primary pod
   127.0.0.1:6379> info replication
   ```

   ![Redis info replication](../../../img/redis-ha-info-replication.png)

   From the output, `redis-cluster-redis-1` has been assigned as the primary's pod.

5. Describe the cluster and check the instance role.

   ```bash
   kbcli cluster describe redis-cluster
   ```

   ![Redis cluster status after HA](./../../../img/redis-ha-after.png)

   After the failover, `redis-cluster-redis-0` becomes the secondary pod and `redis-cluster-redis-1` becomes the primary pod.

</TabItem>

</Tabs>
