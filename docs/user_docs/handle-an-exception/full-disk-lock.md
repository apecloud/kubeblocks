---
title: Full disk lock
description: How to configure disk lock when it is about to be full
sidebar_position: 2
sidebar_label: Full disk lock
---

# Full disk lock

The full disk lock function of KubeBlocks ensures the stability and availability of a database. This function triggers a disk lock when the disk usage reaches a set threshold, thereby pausing write operations and only allowing read operations. Such a mechanism prevents a database from being affected by disk space exhaustion.

## Lock/unlock mechanism

When the space water level of any configured volume exceeds the defined threshold, the instance is locked (read-only). Meanwhile, the system sends a related warning event, including the specific threshold and space usage information of each volume.

When the space water level of all configured volumes falls below the defined threshold, the instance is unlocked (read and write). Meanwhile, the system sends a related warning event, including the specific threshold and space usage information of each volume.

:::note

1. The full disk lock function currently supports global (ClusterDefinition) enabling or disabling and does not support Cluster dimension control. Dynamically enabling or disabling this function may affect the existing Cluster instances that use this ClusterDefinition and cause them to restart. Please operate with caution.

2. The full disk locking function relies on the read permission (get & list) of the two system resource nodes and nodes/stats. If you create an instance via kbcli, make sure to grant the controller administrative rights to the ClusterRoleBinding.

3. Supported engines for KubeBlocks v0.6.0: ApeCloud MySQL, PostgreSQL, MongoDB.

:::

## Enable full disk lock

- For MySQL database, the read/write user cannot write to the disk when the disk usage reaches the `highwatermark` value, while the superuser can still write.
- For PostgreSQL and MongoDB, both the read/write user and the superuser cannot write when the disk usage reaches `highwatermark`.
- `90` is the default value setting for the high watermark at the component level which means the disk is locked when the disk usage reaches 90%, while `85` is used for the volumes which overwrites the component's threshold value.

In the cluster definition, add the following content to enable the full disk lock function. You can set the value according to your need.

```bash
volumeProtectionSpec:
  highWatermark: 90
  volumes:
  - highWatermark: 85
    name: data
```

:::note

The recommended value of `highWatermark` is 90.

:::

## Disable full disk lock

Delete `volumeProtectionSpec` from the cluster definition file.
