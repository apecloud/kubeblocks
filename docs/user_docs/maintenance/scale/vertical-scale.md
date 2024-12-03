---
title: Vertical Scale
description: How to vertically scale a cluster
keywords: [vertical scaling, vertical scale]
sidebar_position: 2
sidebar_label: Vertical Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Vertical Scale

You can change the resource requirements and limits (CPU and storage) by performing the vertical scale. For example, you can change the resource class from 1C2G to 2C4G.

This tutorial takes MySQL as an example to illustrate how to vertically scale a cluster.

:::note

From v0.9.0, for MySQL and PostgreSQL, after vertical scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature. This feature simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md).

:::

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   mysql                mysql-8.0.33   Delete               Running   4d18h
```

## Steps

There are two ways to apply vertical scaling.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: mysql
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs, you can troubleshoot with `kubectl describe ops -n demo` command to view the events of this operation.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>
  
<TabItem value="Edit the cluster YAML file" label="Edit the cluster YAML file">

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```yaml
   kubectl edit cluster mycluster -n demo
   ```

   Edit the values of `spec.componentSpecs.resources`.

   ```yaml
   >
   ...
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 3
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
   ...
   ```

2. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>
