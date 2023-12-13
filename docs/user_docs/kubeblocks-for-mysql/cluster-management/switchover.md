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
       failureThreshold: 3
       periodSeconds: 2
       timeoutSeconds: 1
   ```

## Initiate the switchover

You can switch over a follower of an ApeCloud MySQL RaftGroup to the leader role, and the former leader instance to a follower.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

* Initiate a switchover with no leader instance specified.

    ```bash
    kbcli cluster promote mycluster
    ```

* Initiate a switchover with a specified new leader instance.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2'
    ```

* If there are multiple components, you can use `--component` to specify a component.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' --component='apecloud-mysql'
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

The value of `instanceName` decides whether a new leader instance is specified for the switchover.

* Initiate a switchover with no specified leader instance.

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover-jhkgl
  spec:
    clusterRef: mycluster
    type: Switchover
    switchover:
    - componentName: apecloud-mysql
      instanceName: '*'
  >>
  ```

* Initiate a switchover with a specified new leader instance.

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover-jhkgl
  spec:
    clusterRef: mycluster
    type: Switchover
    switchover:
    - componentName: apecloud-mysql
      instanceName: 'mycluster-mysql-2'
  >>
  ```

</TabItem>

</Tabs>

## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

```bash
kbcli cluster list-instances
```



## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
