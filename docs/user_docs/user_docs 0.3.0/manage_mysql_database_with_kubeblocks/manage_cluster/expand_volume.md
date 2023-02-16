# Expand volume
You can expand the storage volume size of each pod.

> ***Note:*** 
> 
> Volume expansion triggers pod restart, all pods restart in the order of `learner -> follower -> leader` and the leader pod may change after the operation.

***Before you start***

Run the command below to check whether the cluster STATUS is Running. Otherwise, the following operations may fail.
```
kbcli cluster list NAME
```

***Example***

```$ kbcli cluster list mysql-cluster
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

***Steps:***

1. Expand volume.
   
   **Option 1.** Use `kbcli`.

   Configure the values of `--component-names`, `--volume-claim-template-names`, and `--storage` and run the command below to expand the volume.
   ```
   kbcli cluster volume-expand mysql-cluster --component-names="mysql" \
   --volume-claim-template-names="data" --storage="2Gi"
   ```
   - `component-names` describes the component name for volume expansion.
   - `volume-claim-template-names` describes the VolumeClaimTemplate names in components.
   - `storage describes` the volume storage size.
   
   **Option 2.** Create an OpsRequest.

   Run the command below to expand the volume of a cluster.
   ```
   $ kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-volume-expansion
   spec:
     clusterRef: mysql-01
     type: VolumeExpansion
     volumeExpansion:
     - componentName: mysql
       volumeClaimTemplates:
     - name: data
       storage: "2Gi"
   EOF
   ```
  
   **Option 3.** Change the YAML file of the cluster.

   Change the value of `spec.components.volumeClaimTemplates.spec.resources` in the cluster YAML file. `spec.components.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster. 

   ***Example***

   ```
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-01
     namespace: default
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     components:
     - name: mysql
       type: mysql
       replicas: 1
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi # Change the volume storage size.
   terminationPolicy: Halt
   ```
