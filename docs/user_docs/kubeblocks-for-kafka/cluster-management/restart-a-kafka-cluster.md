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

    ```yaml
    kubectl apply -f - <<EOF
    apiVersion: operations.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: kafka-combine-restart
      namespace: demo
    spec:
      clusterName: mycluster
      type: Restart
      restart:
      - componentName: kafka-combine
    EOF
    ```

kubectl apply -f - <<EOF
apiVersion: operations.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: kafka-combine-restart
  namespace: demo
spec:
  clusterName: mycluster
  type: Restart
  restart:
  - componentName: kafka-combine
EOF

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo
   >
   NAME                         READY   STATUS        RESTARTS   AGE
   mycluster-kafka-combine-0    2/2     Terminating   0          36m
   mycluster-kafka-exporter-0   1/1     Running       0          36m
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart operation is in progress.
   - STATUS=Running: it means the cluster has been restarted.

   ```bash
   kubectl get ops kafka-combine-restart -n demo
   >
   NAME                    TYPE      CLUSTER     STATUS    PROGRESS   AGE
   kafka-combine-restart   Restart   mycluster   Succeed   1/1        63s
   ```

   For the OpsRequest, there are two status types for pods.

   - STATUS=Running: it means the cluster restart operation is in progress.
   - STATUS=Succeed: it means the cluster has been restarted.

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Restart a cluster.
  
   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="kafka-combine" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        kafka                Delete               Running   Jan 21,2025 12:31 UTC+0800
   ```

   * STATUS=Restarting: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

</Tabs>
