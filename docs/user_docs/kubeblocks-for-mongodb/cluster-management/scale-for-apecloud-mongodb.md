---
title: Scale for MongoDB cluster
description: How to vertically scale a MongoDB cluster
keywords: [mongodb, vertical sclaing, vertially scale a mongodb cluster]
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
kbcli cluster list mongodb-cluster
```

### Steps

1. Change configuration. There are 3 ways to apply vertical scaling.

   **Option 1.** (**Recommended**) Use kbcli

   1. Use `kbcli cluster vscale` and configure the resources required.

      ***Example***

      ```bash
      kbcli cluster vscale mongodb-cluster --components=mongodb --cpu=500m --memory=500Mi
      >
      OpsRequest mongodb-cluster-verticalscaling-thglk created successfully, you can view the progress:
             kbcli cluster describe-ops mongodb-cluster-verticalscaling-thglk -n default
      ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   2. Validate the scaling with `kbcli cluster describe-ops mongodb-cluster-verticalscaling-thglk -n default`.

     :::note

     `thglk` is the OpsRequest number randomly generated in step 1.

     :::
  
   **Option 2.** Change the YAML file of the cluster

   Change the configuration of `spec.components.resources` in the YAML file. `spec.components.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ***Example***

   ```YAML
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mongodb-cluster
     namespace: default
   spec:
     clusterDefinitionRef: mongodb
     clusterVersionRef: mongodb-5.0
     componentSpecs:
     - name: mongodb
       componentDefRef: mongodb
       replicas: 1
       resources: # Change the values of resources.
         requests:
           memory: "2Gi"
           cpu: "1000m"
         limits:
           memory: "4Gi"
           cpu: "2000m"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Halt
   ```

2. Validate the vertical scaling.

    ```bash
    kbcli cluster list mongodb-cluster
    >
    NAME              NAMESPACE   CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    CREATED-TIME                 
    mongodb-cluster   default     mongodb              mongodb-5.0   WipeOut              Running   Apr 26,2023 11:50 UTC+0800  
    ```

   - STATUS=VerticalScaling: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal.

  :::note

  To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.
  
  :::

:::note

Vertical scaling does not synchronize parameters related to CPU and memory and it is required to manually call the opsRequest of configuration to change parameters accordingly. Refer to [Configuration](./../configuration/configuration.md) for instructions.

:::

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mongodb-cluster
    ```
