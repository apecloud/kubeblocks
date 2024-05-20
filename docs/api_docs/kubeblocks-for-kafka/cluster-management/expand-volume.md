---
title: Expand volume
description: How to expand the volume of a kafka cluster
keywords: [kafka, expand volume, volume expansion]
sidebar_position: 4
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
```

## Steps

1. Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volumeexpansion
      namespace: demo
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: broker
        volumeClaimTemplates:
        - name: data
          storage: 30Gi
    EOF
    ```

    </TabItem>

    <TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

    Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

    `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mycluster
      namespace: demo 
    spec:
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - name: kafka 
        componentDefRef: kafka
        volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 40Gi # Change the volume storage size.
      terminationPolicy: Halt
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
   >
   ......
   Volume Claim Templates:
     Name:  data
     Spec:
       Access Modes:
         ReadWriteOnce
       Resources:
         Requests:
           Storage:   40Gi
   ```
