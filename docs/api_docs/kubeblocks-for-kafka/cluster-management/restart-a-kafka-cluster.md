---
title: Restart Kafka cluster
description: How to restart a Kafka cluster
keywords: [kafka, restart]
sidebar_position: 4
sidebar_label: Restart
---


# Restart a Kafka cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

The pod role may change after the cluster restarts.

:::

## Steps

1. Create an OpsRequest to restart a cluster.

  ```bash
  kubectl apply -f - <<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: ops-restart
    namespace: demo
  spec:
    clusterName: mycluster
    type: Restart 
    restart:
    - componentName: broker
  EOF
  ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.
