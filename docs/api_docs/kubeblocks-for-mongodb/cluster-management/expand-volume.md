---
title: Expand volume
description: How to expand the volume of a MongoDB cluster
keywords: [mongodb, expand volume, volume expansion]
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
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
     - componentName: mongodb
       volumeClaimTemplates:
       - name: data
         storage: "1Gi"
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
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - name: mongodb 
       componentDefRef: mongodb
       replicas: 1
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # Change the volume storage size.
     terminationPolicy: Halt
   ```

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
   >
   ......
   Volume Claim Templates:
      Name:  data
      Spec:
        Access Modes:
          ReadWriteOnce
        Resources:
          Requests:
            Storage:   1Gi
   ```
