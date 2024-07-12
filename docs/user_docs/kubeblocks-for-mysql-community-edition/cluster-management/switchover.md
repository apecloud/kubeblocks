---
title: Switch over a MySQL cluster
description: How to switch over a MySQL cluster
keywords: [mysql, switch over a cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Switch over a MySQL cluster

You can initiate a switchover for an ApeCloud MySQL RaftGroup by executing the kbcli or kubectl command. Then KubeBlocks switches the instance roles.

## Before you start

* Make sure the cluster is running normally.
* Check whether the following role probe parameters exist to verify whether the role probe is enabled.

   ```bash
   kubectl get cd apecloud-mysql -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 2
       periodSeconds: 1
       timeoutSeconds: 1
   ```

## Initiate the switchover

You can switch over a secondary of a MySQL Replication to the primary role, and the former primary instance to a secondary one.

* Initiate a switchover with a specified new leader instance.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-1'
    ```

* If there are multiple components, you can use `--components` to specify a component.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-1' --components='apecloud-mysql'
    ```

## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

```bash
kbcli cluster list-instances
```

## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
