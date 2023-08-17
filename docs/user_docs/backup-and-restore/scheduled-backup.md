---
title: Scheduled backup
description: How to back up databases by schedule
keywords: [backup and restore, schedule, scheduled backup]
sidebar_position: 5
sidebar_label: Scheduled backup and restore
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scheduled backup

You can customize your backup schedule by modifying relevant parameters.

:::caution

The backup created by kbcli or kubectl is saved permanently. If you want to delete the backup, you can delete it manually.

:::

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy
>
spec:
  ...
  schedule:
    datafile:
      # UTC time zone, the example below stands for 2 A.M. every Monday
      cronExpression: "0 18 * * 0"
      # Enable this funciton
      enable: true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # Enable automatic backup
    enabled: true
    # UTC time zone, the example below stands for 2 A.M. every Monday
    cronExpression: 0 18 * * *
    # It specifies the backup method. Here is an example of backupTool. If your storage suports snapshot, you can change it to snapshot
    method: backupTool
    # Disable PITR. If enabled, automatic backup is enabled accordingly
    pitrEnabled: false
    # Retention period for a backup set
    retentionPeriod: 1d
```

</TabItem>

</Tabs>
