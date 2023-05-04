---
title: Scale for a MySQL cluster
description: How to scale a MySQL cluster, horizontal scaling, vertical scaling
keywords: [mysql, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

# Scale for an ApeCloud MySQL cluster

You can scale a MySQL cluster in two ways, vertical scaling and horizontal scaling.

## Vertical scaling

You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

:::note

During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.

:::

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list <name>
```

***Example***

```bash
kbcli cluster list mysql-cluster
>
NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

### Steps

1. Change configuration. There are 3 ways to apply vertical scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

   ***Example***

   ```bash
   kbcli cluster vscale mysql-cluster \
   --components="mysql" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

   You can also vertically scale a cluster with specified class type.

   1. List all classes with `kbcli class list` command and choose the one you need, or check [class type](./../cluster-type/cluster-types.md) document for reference.

   ```bash
   kbcli class list --cluster-definition apecloud-mysql  
   ```

   :::note
  
   If there is no suitable class listed, you can [customize your own class](./../cluster-type/customize-class-type.md) template and apply the class here.

   Creating clusters that does not meet the constraints is invalid and system creates the cluster with the minimum CPU value specified.

   :::

   2. Use `--set` option with `kbcli cluster vscale` command.

   ```bash
   kbcli cluster vscale mysql-clsuter --components="mysql" --cluster-definition apecloud-mysql --set class=general-2c2g
   ```

   **Option 2.** Create an OpsRequest
  
   Run the command below to apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
   spec:
     clusterRef: mysql-cluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: mysql
       requests:
         memory: "2Gi"
         cpu: "1000m"
       limits:
         memory: "4Gi"
         cpu: "2000m"
   EOF
   ```
  
   **Option 3.** Change the YAML file of the cluster

   Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ***Example***

   ```YAML
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: default
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
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

    Check the cluster status to identify the vertical scaling status.

    ```bash
    kbcli cluster list <name>
    ```

    ***Example***

    ```bash
    kbcli cluster list mysql-cluster
    >
    NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS          CREATED-TIME
    mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Updating        Jan 29,2023 14:29 UTC+0800
    ```

   - STATUS=VerticalScaling: it means the vertical scaling is in progress.
   - STATUS=Running: it means the vertical scaling operation has been applied.
   - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal.
     > To solve the problem, you can check manually to see whether resources are sufficient. If AutoScaling is supported, the system recovers when there are enough resources, otherwise, you can create enough resources and check the result with kubectl describe command.

## Horizontal scaling

Horizontal scaling changes the amount of pods. For example, you can apply horizontal scaling to scale up from three pods to five pods. The scaling process includes the backup and restoration of data.

### Before you start

- Refer to [Backup and restore for MySQL](./../backup-and-restore/backup-and-restore-for-mysql-standalone.md) to make sure the EKS environment is configured properly since the horizontal scaling relies on the backup function.
- Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

  ```bash
  kbcli cluster list <name>
  ```

  ***Example***

  ```bash
  kbcli cluster list mysql-cluster
  >
  NAME                 NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
  mysql-cluster        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
  ```

### Steps

1. Change configuration. There are 3 ways to apply horizontal scaling.

   **Option 1.** (**Recommended**) Use kbcli

   Configure the parameters `--components` and `--replicas`, and run the command.

   ***Example***

   ```bash
   kbcli cluster hscale mysql-cluster \
   --components="mysql" --replicas=3
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--replicas` describes the replicas with the specified components.

   **Option 2.** Create an OpsRequest

   Run the command below to apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
   spec:
     clusterRef: mysql-cluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mysql
       replicas: 3
   EOF
   ```

   **Option 3.** Change the YAML file of the cluster

   Change the configuration of `spec.componentSpecs.replicas` in the YAML file. `spec.componentSpecs.replicas` stand for the pod amount and changing this value triggers a horizontal scaling of a cluster.

   ***Example***

   ```YAML
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
    apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mysql-cluster
     namespace: default
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 1 # Change the pod amount.
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

2. Validate the horizontal scaling operation.

   Check the cluster STATUS to identify the horizontal scaling status.

   ```bash
   kbcli cluster list mysql-cluster
   ```

   - STATUS=HorizontalScaling: it means horizontal scaling is in progress.
   - STATUS=Running: it means horizontal scaling has been applied.

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions: 
  - lastTransitionTime: "2023-02-08T04:20:26Z"
    message: VolumeSnapshot/mysql-cluster-mysql-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***Reason***

This exception occurs because the `VolumeSnapshotClass` is not configured. This exception can be fixed after configuring `VolumeSnapshotClass`, but the horizontal scaling cannot continue to run. It is because the wrong backup (volumesnapshot is generated by backup) and volumesnapshot generated before still exist. Delete these two wrong resources and then KubeBlocks re-generates new resources.

***Steps:***

1. Configure the VolumeSnapshotClass by running the command below.

   ```bash
   kubectl create -f - <<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshotClass
   metadata:
     name: csi-aws-vsc
     annotations:
       snapshot.storage.kubernetes.io/is-default-class: "true"
   driver: ebs.csi.aws.com
   deletionPolicy: Delete
   EOF
   ```

2. Delete the wrong backup (volumesnapshot is generated by backup) and volumesnapshot resources.

   ```bash
   kubectl delete backup -l app.kubernetes.io/instance=mysql-cluster
   
   kubectl delete volumesnapshot -l app.kubernetes.io/instance=mysql-cluster
   ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
