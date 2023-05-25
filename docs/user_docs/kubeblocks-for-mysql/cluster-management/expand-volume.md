---
title: Expand volume
description: How to expand the volume of a MySQL cluster
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

:::note

Volume expansion triggers pod restart, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the operation.

:::

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

    **Option 1.** (**Recommended**) Use kbcli

    Configure the values of `--components`, `--volume-claim-template-names`, and `--storage`, and run the command below to expand the volume.

    ```bash
    kbcli cluster volume-expand mysql-cluster --components="mysql" \
    --volume-claim-template-names="data" --storage="2Gi"
    ```

    - `--components` describes the component name for volume expansion.
    - `--volume-claim-template-names` describes the VolumeClaimTemplate names in components.
    - `--storage` describes the volume storage size.

    **Option 2.** Create an OpsRequest

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
    spec:
      clusterRef: mysql-cluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: mysql
        volumeClaimTemplates:
        - name: data
          storage: "2Gi"
    EOF
    ```

    **Option 3.** Change the YAML file of the cluster

    Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

    `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mysql-cluster
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

2. Validate the volume expansion operation.

   ```bash
   kbcli cluster list mysql-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30          Delete                    VolumeExpanding        Jan 29,2023 14:35 UTC+0800
   ```

   * STATUS=VolumeExpanding: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.
