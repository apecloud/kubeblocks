---
title: Restart a MongoDB cluster
description: How to restart a MongoDB cluster
keywords: [mongodb, restart a cluster]
sidebar_position: 4
sidebar_label: Restart
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Restart MongoDB cluster

You can restart all pods of the cluster. When an exception occurs in a database, you can try to restart it.

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
     - componentName: mongodb
   EOF
   ```

2. Check the pod and operation status to validate the restarting.

   ```bash
   kubectl get pod -n demo
   >
   NAME                  READY   STATUS            RESTARTS   AGE
   mycluster-mongodb-0   3/4     Terminating       0          5m32s

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

1. Restart a cluster with `kbcli cluster restart` command and enter the cluster name again.

   ```bash
   kbcli cluster restart mycluster -n demo
   >
   OpsRequest mongodb-cluster-restart-pzsbj created successfully, you can view the progress:
         kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n demo
   ```

2. Validate the restart operation.

   Check the cluster status to identify the restart status.

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME                   NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   mongodb-cluster        demo             mongodb                   mongodb-5.0        Delete                    Running        Apr 26,2023 12:50 UTC+0800
   ```

   - STATUS=Updating: it means the cluster restart is in progress.
   - STATUS=Running: it means the cluster has been restarted.

</TabItem>

</Tabs>
