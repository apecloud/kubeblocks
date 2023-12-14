---
title: Expand volume
description: How to expand the volume of a Pulsar cluster
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list pulsar
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

    **Option 1.** (**Recommended**) Use kbcli

    Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

    :::note

    Expand volume for `journal` first. `ledger` volume expansion must be performed after the `journal` volume expansion.

    :::

    - Expand volume for `journal`.

      ```bash
      kbcli cluster volume-expand pulsar --storage=40Gi --components=bookies -t journal  
      ```

      - `--components` describes the component name for volume expansion.
      - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
      - `--storage` describes the volume storage size.

    - Expand volume for `ledger`.

      ```bash
      kbcli cluster volume-expand pulsar --storage=200Gi --components=bookies -t ledgers  
      ```

    **Option 2.** Create an OpsRequest

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      generateName: pulsar-volume-expand-
    spec:
      clusterRef: pulsar
      type: VolumeExpansion
      volumeExpansion:
      - componentName: bookies
        volumeClaimTemplates:
        - name: ledgers
          storage: "200Gi"
        - name: journal
          storage: "40Gi"      
    EOF
    ```

    **Option 3.** Edit cluster with `kubectl`.

    ```bash
    kubectl edit cluster pulsar
    ```

2. Validate the volume expansion operation.

   ```bash
   kubectl get ops  
   ```

   * STATUS=VolumeExpanding: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe pulsar
    ```
