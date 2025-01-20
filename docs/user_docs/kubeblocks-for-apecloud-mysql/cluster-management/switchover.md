---
title: Switch over an ApeCloud MySQL cluster
description: How to switch over an ApeCloud MySQL cluster
keywords: [mysql, switch over an apecloud cluster, switchover]
sidebar_position: 6
sidebar_label: Switchover
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Switch over an ApeCloud MySQL cluster

You can initiate a switchover for an ApeCloud MySQL RaftGroup by executing the kbcli or kubectl command. Then KubeBlocks switches the instance roles.

## Before you start

* Make sure the cluster is running normally.
  
   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
   mycluster   apecloud-mysql       Delete               Running   45m
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list mycluster -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        apecloud-mysql       Delete               Running   Jan 20,2025 16:27 UTC+0800
   ```

   </TabItem>

   </Tabs>
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

You can switch over a follower of an ApeCloud MySQL RaftGroup to the leader role, and the former leader instance to a follower.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

The value of `instanceName` decides whether a new leader instance is specified for the switchover.

* Initiate a switchover with no leader instance specified.

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-switchover
     namespace: demo
   spec:
     # Specifies the name of the Cluster resource that this operation is targeting.
     clusterName: mycluster
     type: Switchover
     # Lists Switchover objects, each specifying a Component to perform the switchover operation.
     switchover:
       # Specifies the name of the Component.
     - componentName: mysql
       # Specifies the instance to become the primary or leader during a switchover operation. The value of `instanceName` can be either:
       # - "*" (wildcard value): - Indicates no specific instance is designated as the primary or leader.
       # - A valid instance name (pod name)
       instanceName: '*'
   EOF
   ```

* Initiate a switchover with a specified new leader instance.

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-switchover-specify
     namespace: demo
   spec:
     # Specifies the name of the Cluster resource that this operation is targeting.
     clusterName: mycluster
     type: Switchover
     # Lists Switchover objects, each specifying a Component to perform the switchover operation.
     switchover:
       # Specifies the name of the Component.
     - componentName: mysql
       # Specifies the instance to become the primary or leader during a switchover operation. The value of `instanceName` can be either:
       # - "*" (wildcard value): - Indicates no specific instance is designated as the primary or leader.
       # - A valid instance name (pod name)
       instanceName: acmysql-cluster-mysql-2
   EOF
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

* Initiate a switchover with no leader instance specified.

    ```bash
    kbcli cluster promote mycluster -n demo
    ```

* Initiate a switchover with a specified new leader instance.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' -n demo
    ```

* If there are multiple components, you can use `--components` to specify a component.

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-mysql-2' --components='apecloud-mysql' -n demo
    ```

</TabItem>

</Tabs>

## Verify the switchover

Check the instance status to verify whether the switchover is performed successfully.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get pods -n demo
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster list-instances -n demo
```

</TabItem>

</Tabs>

## Handle an exception

If an error occurs, refer to [Handle an exception](./../../handle-an-exception/handle-a-cluster-exception.md) to troubleshoot the operation.
