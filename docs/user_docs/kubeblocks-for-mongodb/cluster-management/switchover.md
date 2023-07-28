---
title: Switch over a MongoDB cluster
description: How to switch over a MongoDB cluster
keywords: [mongodb, switch over a cluster, switchover]
sidebar_position: 7
sidebar_label: Switchover
---

# Switch over a MongoDB cluster

You can initiate a switchover for a PrimarySecondary MongoDB database by executing the kbcli or kubectl command. Then KubeBlocks modifies the instance roles.

## Before you start

* Make sure the cluster is running normally.
* Check whether the following role probe parameters exist to verify whether the role probe is enabled.

   ```bash
   kubectl get cd mongodb -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 3
       periodSeconds: 2
       timeoutSeconds: 1
   ```

## Initiate the switchover

You can switch over a secondary of a MongoDB PrimarySecondary to the primary role, and the former primary instance to a secondary.

<details open>

<summary>kbcli</summary>

* Switchover with no primary instance specified

    ```bash
    kbcli cluster promote mycluster
    ```

* Switchover with a specified new primary instance

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mongodb-2'
    ```

* If there are multiple components, you can use `--component` to specify a component.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mongodb-2' --component='mongodb'
    ```

</details>

<details>
<summary>kubectl</summary>

Different instanceNames decide whether a new primary instance is specified for the switchover.

* Switchover with no specified primary instance

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
    - componentName: mongodb
      instanceName: '*'
  >>
  ```

* Switchover with a specified new primary instance

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
    - componentName: mongodb
      instanceName: 'mycluster-mongodb-2'
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
