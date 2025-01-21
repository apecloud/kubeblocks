---
title: Expand volume
description: How to expand the volume of a kafka cluster
keywords: [kafka, expand volume, volume expansion]
sidebar_position: 4
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl -n demo get cluster mycluster
>
NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
mycluster   kafka                Delete               Running   43m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        kafka                Delete               Running   Jan 21,2025 11:31 UTC+0800
```

</TabItem>

</Tabs>

## Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest. Change the value of storage according to your need and run the command below to expand the volume of a cluster.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: kafka-combined-volumeexpansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: kafka-combine
       volumeClaimTemplates:
       - name: data
         storage: 40Gi
   EOF
   ```

2. Validate the volume expansion operation.

   ```bash
   kubectl get ops -n demo
   >
   NAME                             TYPE              CLUSTER     STATUS    PROGRESS   AGE
   kafka-combined-volumeexpansion   VolumeExpansion   mycluster   Succeed   3/3        6m
   ```

3. Check whether the corresponding cluster resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ...
   Volume Claim Templates:
     Name:  data
     Spec:
       Access Modes:
         ReadWriteOnce
       Resources:
         Requests:
           Storage:   40Gi
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the values of `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` in the YAML file.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo 
   spec:
   ...
     componentSpecs:
       - name: kafka-combine
       ...
         volumeClaimTemplates:
           - name: data
             spec: 
               storageClassName: "<you-preferred-sc>"
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 40Gi # Change this value to specify new size, and make sure it is larger than the current size
           - name: metadata
             spec: 
               storageClassName: "<you-preferred-sc>"
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 40Gi # Change this value to specify new size, and make sure it is larger than the current size
   ```

2. Check whether the corresponding cluster resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Configure the resources according to your needs and run the command to expand the volume.

   ```bash
   kbcli cluster volume-expand mycluster -n demo --storage=30Gi --components=kafka-combine --volume-claim-templates=data 
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

2. Validate the volume expansion operation.
    - View the OpsRequest progress.

      KubeBlocks outputs a command automatically for you to view the details of the OpsRequest progress. The output includes the status of this OpsRequest and PVC. When the status is `Succeed`, this OpsRequest is completed.

      ```bash
      kbcli cluster describe-ops mycluster-volumeexpansion-8257f -n demo
      ```

    - View the cluster status.

      ```bash
      kbcli cluster list mycluster -n demo
      >
      NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS     CREATED-TIME
      mycluster   demo        kafka                Delete               Updating   Jan 21,2025 11:31 UTC+0800
      ```

      * STATUS=Updating: it means the volume expansion is in progress.
      * STATUS=Running: it means the volume expansion operation has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>
