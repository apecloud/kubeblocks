---
title: Scale for a PostgreSQL cluster
description: How to vertically scale a PostgreSQL cluster
keywords: [postgresql, vertical scale]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale for a PostgreSQL cluster

Currently, only vertical scaling for PostgreSQL is supported.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, a concurrent restart is triggered and the leader pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
mycluster   postgresql           postgresql-14.8.0   Delete               Running   36m
```

### Steps

1. Change configuration. There are 2 ways to apply vertical scaling.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>
  
   Run the command below to apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scalin
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: postgresql
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```
  
   </TabItem>

   <TabItem value="Edit Cluster YAML File" label="Edit Cluster YAML File">

   Change the configuration of `spec.components.resources` in the YAML file. `spec.components.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ***Example***

   ```YAML
   ......
   spec:
     affinity:
       podAntiAffinity: Preferred
       tenancy: SharedNode
       topologyKeys:
       - kubernetes.io/hostname
     clusterDefinitionRef: postgresql
     clusterVersionRef: postgresql-14.8.0
     componentSpecs:
     - componentDefRef: postgresql
       enabledLogs:
       - running
       monitor: false
       name: postgresql
       replicas: 2
       resources:
         limits:
           cpu: "2"
           memory: 4Gi
         requests:
           cpu: "1"
           memory: 2Gi
   ```

   </TabItem>

   </Tabs>

2. Check the operation status to validate the vertical scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   If an error occurs to the vertical scaling operation, you can troubleshoot with `kubectl describe` command to view the events of this operation.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  postgresql
    Enabled Logs:
      running
    Monitor:   false
    Name:      postgresql
    Replicas:  2
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the OpsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::
