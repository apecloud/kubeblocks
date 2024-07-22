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

Volume expansion triggers a concurrent restart and the pod role may change after the operation.

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

1. Change configuration.

   Configure the values of `--components`, `--volume-claim-templates`, and `--storage`, and run the command below to expand the volume.

   ```bash
   kbcli cluster volume-expand redis-cluster --components="redis" \
   --volume-claim-templates="data" --storage="20Gi"
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

2. Validate the volume expansion.

   ```bash
   kbcli cluster list redis-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   redis-cluster        default          redis                     redis-7.0.6              Delete                    Updating        Apr 10,2023 16:27 UTC+0800
   ```

   - STATUS=Updating: it means the volume expansion is in progress.
   - STATUS=Running: it means the volume expansion operation has been applied.
