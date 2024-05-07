---
title: Scale for MongoDB cluster
description: How to vertically scale a MongoDB cluster
keywords: [mongodb, vertical scaling, vertically scale a mongodb cluster]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for a MongoDB cluster

For MongoDB, vertical scaling is supported.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, a restart is triggered and the primary pod may change after the restarting.

:::

### Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   mongodb              mongodb-5.0   Delete               Running   27m
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
     name: ops-vertical-scaling
   spec:
     clusterName: mycluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mongodb
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
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - componentDefRef: mongodb
       enabledLogs:
       - running
       monitor: false
       name: mongodb
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

2. Validate the volume expansion.

   ```bash
   kubectl describe cluster mycluster -n demo
   >
   ......
   Component Specs:
    Component Def Ref:  mongodb
    Enabled Logs:
      running
    Monitor:   false
    Name:      mongodb
    Replicas:  2
    Resources:
      Limits:
        Cpu:     2
        Memory:  4Gi
      Requests:
        Cpu:     1
        Memory:  2Gi
   ```
