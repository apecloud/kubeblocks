---
title: Switch over a PostgreSQL cluster
description: How to switch over a PostgreSQL cluster
keywords: [postgresql, switch over a cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---

# Switch over a PostgreSQL cluster

You can initiate a switchover for a PostgreSQL Replication Cluster. Then KubeBlocks modifies the instance roles.

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

You can switch over a secondary Pod of a PostgreSQL PrimarySecondary cluster to the primary one, and the former primary instance to a secondary.

The value of `instanceName` decides whether a new primary instance is specified for the switchover.

* Switchover with no specified primary instance

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
    type: Switchover
    switchover:
    - componentName: postgresql
      instanceName: '*'
  >>
  ```

* Switchover with a specified new primary instance

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
    type: Switchover
    switchover:
    - componentName: postgresql
      instanceName: 'mycluster-postgresql-2'
  >>
  ```

## Verify the switchover

Check the cluster and instance status to verify whether the switchover is performed successfully.

```bash
kubectl get cluster mycluster -n demo

kubectl -n demo get po -L kubeblocks.io/role 
```

## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
