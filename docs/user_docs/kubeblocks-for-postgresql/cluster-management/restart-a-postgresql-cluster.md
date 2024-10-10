---
title: Restart PostgreSQL cluster
description: How to restart a PostgreSQL cluster
keywords: [postgresql, restart]
sidebar_position: 4
sidebar_label: Restart
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Restart PostgreSQL cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

:::note

The pod role may change after the cluster restarts.

:::

## Steps

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster -n demo --components="postgresql" --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME       NAMESPACE   CLUSTER-DEFINITION          VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo       postgresql                  postgresql-14.8.0   Delete               Running   Sep 28,2024 16:57 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Restart a cluster.

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
     - componentName: postgresql
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo
   >
   NAME                     READY   STATUS            RESTARTS   AGE
   mycluster-postgresql-0   3/4     Terminating       0          5m32s
   mycluster-postgresql-1   4/4     Running           0          6m36s

   kubectl get ops ops-restart -n demo
   >
   NAME          TYPE      CLUSTER     STATUS    PROGRESS   AGE
   ops-restart   Restart   mycluster   Succeed   1/1        3m26s
   ```

   During the restarting process, there are two status types for pods.

   - STATUS=Terminating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

</TabItem>

</Tabs>
