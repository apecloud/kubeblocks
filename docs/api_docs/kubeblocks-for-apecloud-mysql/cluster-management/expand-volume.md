---
title: Expand volume
description: How to expand the volume of a MySQL cluster
sidebar_position: 3
sidebar_label: Expand volume
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Expand volume

You can expand the storage volume size of each pod.

:::note

Volume expansion triggers pod restart, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the operation.

:::

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME             NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mycluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        April 25,2024 17:29 UTC+0800
```

## Steps

1. Change configuration. There are two ways to apply volume expansion.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
    spec:
      clusterName: mycluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: mysql
        volumeClaimTemplates:
        - name: data
          storage: "2Gi"
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
      namespace: default
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      componentSpecs:
      - name: mysql
        componentDefRef: mysql
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

    </TabItem>

    </Tabs>

2. Validate the volume expansion operation.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME             NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   mycluster        default          apecloud-mysql            ac-mysql-8.0.30          Delete                    VolumeExpanding        April 25,2024 17:35 UTC+0800
   ```

   * STATUS=VolumeExpanding: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kubectl describe cluster mycluster -n demo
    ```
