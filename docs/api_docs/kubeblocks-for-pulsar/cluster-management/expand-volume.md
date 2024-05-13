---
title: Expand volume
description: How to expand the volume of a Pulsar cluster
sidebar_position: 3
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list pulsar
```

## Steps

1. Change configuration. There are 2 ways to apply volume expansion.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expand
      namespace: demo
    spec:
      clusterRef: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: bookies
        volumeClaimTemplates:
        - name: ledgers
          storage: "200Gi"
        - name: journal
          storage: "40Gi"      
    EOF
    ```

   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

    ```bash
    kubectl edit cluster mycluster -n demo
    ```

   </TabItem>

   </Tabs>

2. Validate the volume expansion operation.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-volume-expansion   VolumeExpansion   mycluster   Succeed   3/3        6m
   ```

3. Check whether the corresponding cluster resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```
