---
title: Expand volume
description: How to expand the volume of a MySQL cluster
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mycluster
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 18:46 UTC+0800
```

## Steps

1. Change configuration.

    Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

    ```bash
    kbcli cluster volume-expand mycluster --components="mysql" \
    --volume-claim-templates="data" --storage="2Gi"
    ```

    - `--components` describes the component name for volume expansion.
    - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
    - `--storage` describes the volume storage size.


2. Validate the volume expansion operation.

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 18:46 UTC+0800
   ```

   * STATUS=Updating: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster
    ```
