---
title: Switch over a MySQL cluster
description: How to switch over a MySQL cluster
keywords: [mysql, switch over a cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---

# Switch over a MySQL cluster

You can initiate a switchover for an ApeCloud MySQL Raft Group by executing the kbcli or kubectl command. Then KubeBlocks modifies the instance roles.

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

You can switch over a follower of an ApeCloud MySQL Raft Group to the leader role, and the former leader instance to a follower.

<details open>

<summary>kbcli</summary>

* Switchover with no leader instance specified

    ```bash
    kbcli cluster promote mycluster
    ```

* Switchover with a specified new leader instance

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2'
    ```

* If there are multiple components, you can use `--component` to specify a component.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' --component='apecloud-mysql'
    ```

</details>

<details>
<summary>kubectl</summary>

Different instanceNames decide whether a new leader instance is specified for the switchover.

* Switchover with no specified leader instance

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

* Switchover with a specified new leader instance

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

</details>

## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

```bash
kbcli cluster list-instances
```

## Handle an exception

If an error occurs, execute the command below to troubleshoot the operation.
