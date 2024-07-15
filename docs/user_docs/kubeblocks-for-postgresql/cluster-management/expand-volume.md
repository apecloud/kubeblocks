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

Volume expansion triggers a concurrent restart and the pod role may change after the operation.

:::

## Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list pg-cluster
>
NAME              NAMESPACE        CLUSTER-DEFINITION    VERSION                  TERMINATION-POLICY        STATUS         CREATED-TIME
pg-cluster        default          postgresql            postgresql-14.8.0        Delete                    Running        Mar 3,2023 10:29 UTC+0800
```

## Steps

1. Change configuration.

   Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

   ```bash
   kbcli cluster volume-expand pg-cluster --components="postgresql" \
   --volume-claim-templates="data" --storage="20Gi"
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

2. Validate the volume expansion.

   ```bash
   kbcli cluster list pg-cluster
   >
   NAME              NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   pg-cluster        default          postgresql                postgresql-14.8.0        Delete                    Updating        Apr 10,2023 16:27 UTC+0800
   ```

   * STATUS=Updating: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.
