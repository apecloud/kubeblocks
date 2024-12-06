---
title: Restart Kafka cluster
description: How to restart a Kafka cluster
keywords: [kafka, restart]
sidebar_position: 4
sidebar_label: Restart
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Restart a Kafka cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

The pod role may change after the cluster restarts.

:::

## Steps

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

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

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Restart a cluster.
  
   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="kafka" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list cluster-name
   >
   NAME    CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     AGE
   kafka   kafka                kafka-3.3.2    Delete               Running    19m
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

</Tabs>
