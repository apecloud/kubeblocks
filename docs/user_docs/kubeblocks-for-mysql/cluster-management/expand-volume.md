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

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

    Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

    ```bash
    kbcli cluster volume-expand mysql-cluster --components="mysql" \
    --volume-claim-templates="data" --storage="2Gi"
    ```

    - `--components` describes the component name for volume expansion.
    - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
    - `--storage` describes the volume storage size.


2. Validate the volume expansion operation.

   ```bash
   kbcli cluster list mysql-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS                 CREATED-TIME
   mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30          Delete                    VolumeExpanding        Jan 29,2023 14:35 UTC+0800
   ```

   * STATUS=Updating: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mysql-cluster
    ```
