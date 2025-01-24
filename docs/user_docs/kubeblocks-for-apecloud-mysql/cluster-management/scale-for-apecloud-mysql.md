---
title: How to scale a MySQL cluster, horizontal scaling, vertical scaling
keywords: [mysql, horizontal scaling, vertical scaling]
sidebar_position: 2
sidebar_label: Scale
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Scale an ApeCloud MySQL cluster

This guide describes how to vertically and horizontally scale an ApeCloud MySQL cluster. Vertical scaling changes the CPU and memory of this cluster. Horizontal scaling changes the replica number of this cluster.

:::note

After vertical scaling or horizontal scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature. This feature simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../configuration/configuration.md).

:::

## Vertical scaling

You can perform a vertical scaling task to changes resource requirements and limits (CPU and memory) of a cluster. For example, you can change the resource class from 1C2G to 2C4G by performing a vertical scaling task.

### Before you start

Make sure the cluster status is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       Delete               Running   30m
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

### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest on the cluster `mycluster`. Configure the values of `spec.verticalscaling.requests` and  `spec.verticalscaling.limits` as needed.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-verticalscaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: mysql
       requests:
         cpu: '1'
         memory: 1Gi
       limits:
         cpu: '1'
         memory: 1Gi
   EOF
   ```

2. Check the OpsRequest status to verify the vertical scaling .

   ```bash
   kubectl get ops -n demo
   >
   NAME                      TYPE              CLUSTER     STATUS    PROGRESS   AGE
   acmysql-verticalscaling   VerticalScaling   mycluster   Succeed   1/1        3m54s
   ```

   The OpsRequest statuses are as follow:

   - STATUS=Running: It means the vertical scaling task is in progress.
   - STATUS=Succeed: It means the vertical scaling task has completed and the cluster class is changed as required.
   - STATUS=Failed: It means the vertical scaling task failed. You can troubleshoot it with `kubectl describe ops <opsRequestName> -n demo` to view the details of this OpsRequest.

3. Check whether the cluster is running again and corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the configuration of `spec.componentSpecs.resources` in the YAML file. `spec.componentSpecs.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.resources`.

   ```yaml
   ...
   spec:
     componentSpecs:
       - name: mysql
         replicas: 3
         resources:
           requests:
             cpu: "1"       # Update the values according to your needs
             memory: "2Gi"  # Update the values according to your needs
           limits:
             cpu: "2"       # Update the values according to your needs
             memory: "4Gi"  # Update the values according to your needs
   ...
   ```

2. Check whether the cluster is running again and corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Configure the parameters `--components`, `--memory`, and `--cpu`.

   ```bash
   kbcli cluster vscale mycluster --components="mysql" --memory="4Gi" --cpu="2" -n demo
   ```

   - `--components` describes the component name ready for vertical scaling.
   - `--memory` describes the requested and limited size of the component memory.
   - `--cpu` describes the requested and limited size of the component CPU.

2. Choose one of the following options to validate the vertical scaling operation.

   - View the OpsRequest progress.

     KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

     ```bash
     kbcli cluster describe-ops mycluster-verticalscaling-g67k9 -n demo
     ```

   - Check the cluster status.

     ```bash
     kbcli cluster list mycluster -n demo
     >
     NAME        NAMESPACE   CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS     CREATED-TIME
     mycluster   demo        apecloud-mysql       Delete               Running    Jan 20,2025 16:27 UTC+0800
     ```

     - STATUS=Updating: it means the vertical scaling is in progress.
     - STATUS=Running: it means the vertical scaling operation has been applied.
     - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.

       > To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

## Horizontal scaling

Horizontal scaling changes the number of pods. For example, you can scale out replicas from three to five. The scaling process includes the backup and restore of data.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to the [Horizontal Scale tutorial](./../../maintenance/scale/horizontal-scale.md) for more details and examples.

### Before you start

Check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```bash
kubectl get cluster mycluster -n demo
>
NAME        CLUSTER-DEFINITION   TERMINATION-POLICY   STATUS    AGE
mycluster   apecloud-mysql       Delete               Running   40m
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

### Steps

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. Apply an OpsRequest to a specified cluster. Configure the parameters according to your needs.

   The example below means adding two replicas.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: acmysql-horizontalscaling
     namespace: demo
   spec:
     clusterName: acmysql-cluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mysql
       replicas: 3
   EOF
   ```

   If you want to scale in replicas, replace `scaleOut` with `scaleIn`.

   The example below means decreasing two replicas.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-horizontal-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: HorizontalScaling
     horizontalScaling:
     - componentName: mysql
       scaleIn:
         replicaChanges: 2
   EOF
   ```

2. Check the operation status to validate the horizontal scaling.

   ```bash
   kubectl get ops -n demo
   >
   NAME                        TYPE                CLUSTER     STATUS    PROGRESS   AGE
   acmysql-horizontalscaling   HorizontalScaling   mycluster   Succeed   1/1        2m54s
   ```

   If an error occurs, you can troubleshoot it with `kubectl describe ops -n demo` command to view the events of this operation.

3. After the cluster status is `Running` again, check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

1. Change the value of `spec.componentSpecs.replicas` in the YAML file.

   `spec.componentSpecs.replicas` stands for the pod number and changing this value triggers a horizontal scaling of a cluster.

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Edit the value of `spec.componentSpecs.replicas`.

   ```yaml
   ...
   spec:
   componentSpecs:
     - name: apecloud-mysql
       replicas: 3 # Decrease the value of `replicas` for scale-in, and increase the value for scale-out
   ...
   ```

2. Check whether the corresponding cluster is running and whether resources change.

   Check whether the cluster is running again.

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   Check whether the values of resources change.

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Configure the parameters `--components` and `--replicas`.

    ```bash
    kbcli cluster hscale mycluster --components="mysql" --replicas=3 -n demo
    ```

    - `--components` describes the component name ready for horizontal scaling.
    - `--replicas` describes the replica number of the specified components. Edit the value based on your demands to scale in or out replicas.

2. Choose one of the following options to validate the horizontal scaling operation.

   - View the OpsRequest progress.

     KubeBlocks outputs a command automatically for you to view the OpsRequest progress. The output includes the status of this OpsRequest and Pods. When the status is `Succeed`, this OpsRequest is completed.

     ```bash
     kbcli cluster describe-ops mycluster-horizontalscaling-ffp9p -n demo
     ```

   - View the cluster status.

     ```bash
     kbcli cluster list mycluster -n demo
     ```

     - STATUS=Updating: it means horizontal scaling is in progress.
     - STATUS=Running: it means horizontal scaling has been applied.

3. After the OpsRequest status is `Succeed` or the cluster status is `Running` again, check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster -n demo
    ```

</TabItem>

</Tabs>

### Handle the snapshot exception

If `STATUS=ConditionsError` occurs during the horizontal scaling process, you can find the cause from `cluster.status.condition.message` for troubleshooting.
In the example below, a snapshot exception occurs.

```bash
Status:
  conditions:
  - lastTransitionTime: "2024-09-19T04:20:26Z"
    message: VolumeSnapshot/mycluster-mysql-scaling-dbqgp: Failed to set default snapshot
      class with error cannot find default snapshot class
    reason: ApplyResourcesFailed
    status: "False"
    type: ApplyResources
```

***Reason***

This exception occurs because the `VolumeSnapshotClass` is not configured. This exception can be fixed after configuring `VolumeSnapshotClass`, but the horizontal scaling cannot continue to run. It is because the wrong backup (volumesnapshot is generated by backup) and volumesnapshot generated before still exist. First delete these two wrong resources and then KubeBlocks re-generates new resources.

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
    kubectl delete backup -l app.kubernetes.io/instance=mycluster -n demo

    kubectl delete volumesnapshot -l app.kubernetes.io/instance=mycluster -n demo
    ```

***Result***

The horizontal scaling continues after backup and volumesnapshot are deleted and the cluster restores to running status.
