---
title: Switch over a PostgreSQL cluster
description: How to switch over a PostgreSQL cluster
keywords: [postgresql, switch over a cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---


# Switch over a PostgreSQL cluster

You can initiate a switchover for a PostgreSQL Replication Cluster by executing the kbcli or kubectl command. Then KubeBlocks modifies the instance roles.

## Before you start

* Make sure the cluster is running normally.
* Check whether the following role probe parameters exist to verify whether the role probe is enabled.

   ```bash
   kubectl get cd postgresql -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 2
       periodSeconds: 1
       timeoutSeconds: 1
   ```

## Initiate the switchover

You can switch over a secondary of a PostgreSQL Replication Cluster to the primary role, and the former primary instance to a secondary.

* Switchover with no primary instance specified

    ```bash
    kbcli cluster promote pg-cluster
    ```

* Switchover with a specified new primary instance

    ```bash
    kbcli cluster promote pg-cluster --instance='pg-cluster-postgresql-2'
    ```

* If there are multiple components, you can use `--components` to specify a component.

    ```bash
    kbcli cluster promote pg-cluster --instance='pg-cluster-postgresql-2' --components='postgresql'
    ```


## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

```bash
kbcli cluster list-instances
```

## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
