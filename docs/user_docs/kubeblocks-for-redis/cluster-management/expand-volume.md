---
title: Expand volume
description: How to expand the volume of a Redis cluster
keywords: [redis, expand volume]
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

:::note

Volume expansion triggers a concurrent restart and the leader pod may change after the operation.

:::

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
```

***Example***

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   redis                redis-7.0.6    Delete               Running   4d18h
```

## Steps

1. Change configuration. There are 2 ways to apply volume expansion.

   **Option 1**. Create an OpsRequest

   Run the command below to expand the volume of a cluster.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volume-expansion
   spec:
     clusterRef: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: redis
       volumeClaimTemplates:
       - name: data
         storage: "2Gi"
   EOF
   ```

   **Option 2**. Change the YAML file of the cluster

   Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: default
   spec:
     clusterDefinitionRef: redis
     clusterVersionRef: redis-7.0.6
     componentSpecs:
     - componentDefRef: redis
       name: redis
       replicas: 2
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # Change the volume storage size.
     terminationPolicy: Delete
   ```

2. Validate the volume expansion.

   ```bash
   kubectl get cluster mycluster
   ```

   ***Example***

   ```bash
   kubectl get cluster mycluster
   >
   NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS            AGE
   mycluster   redis                redis-7.0.6    Delete               VolumeExpanding   4d18h

   ```

   - STATUS=VolumeExpanding: it means the volume expansion is in progress.
   - STATUS=Running: it means the volume expansion operation has been applied.
