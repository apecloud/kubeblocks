---
title: Expand volume
description: How to expand the volume of a MongoDB cluster
keywords: [mongodb, expand volume, volume expansion]
sidebar_position: 3
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME             NAMESPACE        CLUSTER-DEFINITION    VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster        demo             mongodb               mongodb-5.0        Delete                    Running        Apr 10,2023 16:20 UTC+0800
```

</TabItem>

</Tabs>

## Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest. Change the value of storage according to your need and run the command below to expand the volume of a cluster.

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
         storage: "40Gi"
   EOF
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

1. Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

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
               storage: 40Gi # Change the volume storage size
     terminationPolicy: Delete
   ```

2. Check whether the corresponding cluster resources change.

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

<TabItem value="kbcli" label="kbcli">

1. Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

    ```bash
    kbcli cluster volume-expand mycluster -n demo --components="mongodb" --volume-claim-templates="data" --storage="30Gi"
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
       kbcli cluster list mycluster
       >
       NAME              NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS          CREATED-TIME
       mycluster         demo             mongodb                   mongodb-5.0        Delete                    Updating        Apr 10,2023 16:27 UTC+0800
       ```

       * STATUS=Updating: it means the volume expansion is in progress.
       * STATUS=Running: it means the volume expansion operation has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>
