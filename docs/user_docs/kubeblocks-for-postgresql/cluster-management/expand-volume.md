---
title: Expand volume
description: How to expand the volume of a PostgreSQL cluster
keywords: [postgresql, expand volume]
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
kbcli cluster list pg-cluster
>
NAME              NAMESPACE        CLUSTER-DEFINITION    VERSION                  TERMINATION-POLICY        STATUS         CREATED-TIME
pg-cluster        default          postgresql            postgresql-14.7.0        Delete                    Running        Mar 3,2023 10:29 UTC+0800
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

  **Option 1.** (**Recommended**) Use kbcli

  Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

  ```bash
  kbcli cluster volume-expand pg-cluster --components="pg-replication" \
  --volume-claim-templates="data" --storage="2Gi"
  ```

  - `--components` describes the component name for volume expansion.
  - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
  - `--storage` describes the volume storage size.

  **Option 2.** Create an OpsRequest

  Run the command below to expand the volume of a cluster.

  ```bash
  kubectl apply -f - <<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: ops-volume-expansion
  spec:
    clusterRef: pg-cluster
    type: VolumeExpansion
    volumeExpansion:
    - componentName: pg-replication
      volumeClaimTemplates:
      - name: data
        storage: "2Gi"
  EOF
  ```

  **Option 3.** Change the YAML file of the cluster

  Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

  ```yaml
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: pg-cluster
    namespace: default
  spec:
    clusterDefinitionRef: postgresql
    clusterVersionRef: postgresql-14.7.0
    componentSpecs:
    - name: pg-replication
      componentDefRef: postgresql
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

1. Validate the volume expansion.

   ```bash
   kbcli cluster list <name>
   ```

   ***Example***

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME              NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   pg-cluster        default          postgresql                postgresql-14.7.0        Delete                    VolumeExpanding        Apr 10,2023 16:27 UTC+0800
   ```
   
   * STATUS=VolumeExpanding: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.
