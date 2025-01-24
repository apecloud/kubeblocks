---
title: Expand volume
description: How to expand the volume of an ApeCloud MySQL cluster
sidebar_position: 3
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

This guide describes how to expand the storage volume size of each pod.

:::note

Volume expansion triggers pod restart. All pods restart in the order of learner -> follower -> leader and the pod roles may change.

:::

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operation tasks may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       Delete               Running   3m40s
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        apecloud-mysql       Delete               Running   Jan 20,2025 16:27 UTC+0800
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
     name: acmysql-volumeexpansion
     namespace: demo
   spec:
     clusterName: mycluster
     type: VolumeExpansion
     volumeExpansion:
     - componentName: mysql
       volumeClaimTemplates:
       - name: data
         storage: 30Gi
   EOF
   ```

2. Verify the volume expansion task.

   ```bash
   kubectl get ops -n demo
   >
   NAME                      TYPE              CLUSTER     STATUS    PROGRESS   AGE
   acmysql-volumeexpansion   VolumeExpansion   mycluster   Succeed   1/1        3m8s
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation task.

3. Check whether the cluster is running and whether the corresponding cluster resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

   `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of a cluster and changing this value triggers the volume expansion of this cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage`.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
   ...
   spec:
     componentSpecs:
       - name: mysql
         volumeClaimTemplates:
           - name: data
             spec:
               storageClassName: "<you-preferred-sc>"
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 30Gi  # Specify a new size and make sure it is larger than the current size
   ```

2. Check whether the cluster is running and whether resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Change the cluster configuration.

   Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

   ```bash
   kbcli cluster volume-expand mycluster --components="mysql" --volume-claim-templates="data" --storage="40Gi" -n demo
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

2. Choose one of the following options to verify the volume expansion task.

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
      mycluster   demo        apecloud-mysql       Delete               Updating   Jan 20,2025 16:27 UTC+0800
      ```

      * STATUS=Updating: it means the volume expansion is in progress.
      * STATUS=Running: it means the volume expansion task has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>
