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
kbcli cluster list <name>
```

***Example***

```bash
kbcli cluster list redis-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
redis-cluster        default          redis                     redis-7.0.6            Delete                    Running        Apr 10,2023 19:00 UTC+0800
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

   **Option 1**. (**Recommended**) Use kbcli

   Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

   ```bash
   kbcli cluster volume-expand redis-cluster --components="redis" \
   --volume-claim-templates="data" --storage="2Gi"
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

    **Option 2**. Create an OpsRequest

    Run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
    spec:
      clusterRef: redis-cluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: redis
        volumeClaimTemplates:
        - name: data
          storage: "2Gi"
    EOF
    ```

    **Option 3**. Change the YAML file of the cluster

    Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

    `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: redis-cluster
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
   kbcli cluster list <name>
   ```

   ***Example***

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    VolumeExpanding        Apr 10,2023 16:27 UTC+0800
   ```

   - STATUS=VolumeExpanding: it means the volume expansion is in progress.
   - STATUS=Running: it means the volume expansion operation has been applied.
