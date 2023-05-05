---
title: Scale for MongoDB cluster
description: How to scale a MongoDB cluster, vertical scaling
sidebar_position: 2
sidebar_label: Scale
---

# Scale for MongoDB cluster

You can perform vertical scaling for MongoDB cluster. 

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

### Before you start

Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.
```bash
kbcli cluster list <name>
```

### Steps

1. Change configuration. There are 3 ways to apply vertical scaling.
   
   **Option 1.** (**Recommended**) Use kbcli
   
   1.1 Use `kbcli cluster vscale` and configure the resources required.
   
   ***Example***
   
   ```bash
   # kbcli cluster vscale mongodb-cluster --component-names=mongodb --cpu=500m --memory=500Mi
   Please type the name again(separate with white space when more than one): mongodb-cluster
   OpsRequest mongodb-cluster-verticalscaling-thglk created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-verticalscaling-thglk -n default
   ```
   - `--component-names` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.
  1.2. Validate the scaling with `kbcli cluster describe-ops mongodb-cluster-verticalscaling-thglk -n default`
     :::note
     `thglk` is the OpsRequest number randomly generated in step 1.
  
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
     clusterVersionRef: mongodb-5.0.14
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
    Run the command below to check the cluster status to identify the vertical scaling status.
    ```bash
    kbcli cluster list <name>
    ```

    ***Example***

    ```bash
      kbcli cluster list mongodb-cluster
      NAME              NAMESPACE   CLUSTER-DEFINITION   VERSION          TERMINATION-POLICY   STATUS    CREATED-TIME                 
      mongodb-cluster   default     mongodb              mongodb-5.0.14   WipeOut              Running   Apr 26,2023 11:50 UTC+0800  
    ```
   - STATUS=Running: it means the vertical scaling operation is applied.
   - STATUS=Updating: it means the vertical scaling is in progress.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal. 

   :::note

      To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.
   :::


