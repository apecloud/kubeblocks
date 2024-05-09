---
title: Expand volume
description: How to expand the volume of a PostgreSQL cluster
keywords: [postgresql, expand volume]
sidebar_position: 3
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

You can expand the storage volume size of each pod.

:::note

Volume expansion triggers a concurrent restart and the leader pod may change after the operation.

:::

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
mycluster   postgresql           postgresql-14.8.0   Delete               Running   17m
```

## Steps

1. Change configuration. There are 2 ways to apply volume expansion.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Run the command below to expand the volume of a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volume-expansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: postgresql
       volumeClaimTemplates:
       - name: data
         storage: "30Gi"
   EOF
   ```

   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: postgresql
     clusterVersionRef: postgresql-14.8.0
     componentSpecs:
     - name: postgresql
       componentDefRef: postgresql
       replicas: 1
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 30Gi # Change the volume storage size.
     terminationPolicy: Delete
   ```

   </TabItem>

   </Tabs>

2. Validate the volume expansion.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Volume Claim Templates:
      Name:  data
      Spec:
        Access Modes:
          ReadWriteOnce
        Resources:
          Requests:
            Storage:   30Gi
   ```
