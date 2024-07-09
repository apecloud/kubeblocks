---
title: Expand volume
description: How to expand the volume of a kafka cluster
keywords: [kafka, expand volume, volume expansion]
sidebar_position: 4
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

## Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list kafka-cluster
```

## Steps

1. Use `kbcli cluster volume-expand` command, configure the resources required and enter the cluster name again to expand the volume.

   ```bash
   kbcli cluster volume-expand --storage=30G --components=kafka --volume-claim-templates=data kafka-cluster
   ```

   - `--components` describes the component name for volume expansion.
   - `--volume-claim-templates` describes the VolumeClaimTemplate names in components.
   - `--storage` describes the volume storage size.

2. Validate the volume expansion.

   ```bash
   kbcli cluster list kafka-cluster
   >
   NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                  TERMINATION-POLICY        STATUS          CREATED-TIME
   kafka-cluster        default          redis                     kafka-3.3.2              Delete                    Updating        May 11,2023 15:27 UTC+0800
   ```
