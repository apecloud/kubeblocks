---
title: Switchover
description: How to switch over an ApeCloud MySQL Cluster
keywords: [apecloud mysql, switch over an apecloud mysql cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---

# Switchover

You can initiate a switchover for an ApeCloud MySQL RaftGroup cluster. Then KubeBlocks switches the instance roles.

## Before you start

* Make sure the cluster is running normally.
  
  ```bash
  kubectl get cluster mycluster -n demo
  ```

* Check whether the following role probe parameters exist to verify whether the role probe is enabled.

   ```bash
   kubectl get cd apecloud-mysql -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 2
       periodSeconds: 1
       timeoutSeconds: 1
   ```

## Initiate the switchover

You can switch a follower of an ApeCloud MySQL RaftGroup over to the leader role, and the former leader instance over to a follower.

The value of `instanceName` decides whether a new leader instance is specified for the switchover.

* Initiate a switchover with no specified leader instance.

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
    - componentName: apecloud-mysql
      instanceName: '*'
  EOF
  ```

* Initiate a switchover with a specified new leader instance.

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
    - componentName: apecloud-mysql
      instanceName: 'mycluster-mysql-2'
  EOF
  ```

## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

```bash
kubectl get pods -n demo
```

## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
