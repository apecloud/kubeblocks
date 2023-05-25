---
title: Snapshot backup and restore for Redis
description: Guide for backup and restore for Redis
keywords: [redis, snapshot, backup, restore]
sidebar_position: 2
sidebar_label: Snapshot backup and restore
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Snapshot backup and restore for Redis

This section shows how to use `kbcli` to back up and restore a Redis cluster.

***Steps:***

1. Install KubeBlocks and the snapshot-controller add-on.

     ```bash
     kbcli kubeblocks install --set snapshot-controller.enabled=true
     ```

     If you have installed KubeBlock without enabling the snapshot-controller, run the command below.

     ```bash
     kbcli kubeblocks upgrade --set snapshot-controller.enabled=true
     ```

     Since your `kubectl` is already connected to the cluster of cloud Kubernetes service, this command installs the latest version of KubeBlocks in the default namespace `kb-system` in your environment.

     Verify the installation with the following command.

     ```bash
     kubectl get pod -n kb-system
     ```

     The pod with `kubeblocks` and  `kb-addon-snapshot-controller` is shown. See the information below.

     ```bash
     NAME                                              READY   STATUS             RESTARTS      AGE
     kubeblocks-5c8b9d76d6-m984n                       1/1     Running            0             9m
     kb-addon-snapshot-controller-6b4f656c99-zgq7g     1/1     Running            0             9m
     ```

     If the output result does not show `kb-addon-snapshot-controller`, it means the snapshot-controller add-on is not enabled. It may be caused by failing to meet the installable condition of this add-on. Refer to [Enable add-ons](./../../installation/enable-addons.md) to find the environment requirements and then enable the snapshot-controller add-on.

2. Configure cloud managed Kubernetes environment to support the snapshot function. For ACK and GKE, the snapshot function is enabled by default, you can skip this step.
    <Tabs>
    <TabItem value="EKS" label="EKS" default>

     The backup is realized by the volume snapshot function, you need to configure EKS to support the snapshot function.

     - Configure the storage class of the snapshot (the assigned EBS volume is gp3).

       ```bash
       kubectl create -f - <<EOF
       kind: StorageClass
       apiVersion: storage.k8s.io/v1
       metadata:
         name: ebs-sc
         annotations:
           storageclass.kubernetes.io/is-default-class: "true"
       provisioner: ebs.csi.aws.com
       parameters:
         csi.storage.k8s.io/fstype: xfs
         type: gp3
       allowVolumeExpansion: true
       volumeBindingMode: WaitForFirstConsumer
       EOF
       ```

       ```bash
       # Disable the default options if an exception occurs to the default gp2 snapshot
       kubectl patch sc/gp2 -p '{"metadata": {"annotations": {"storageclass.kubernetes.io/is-default-class": "false"}}}'
       ```

     </TabItem>

     <TabItem value="TKE" label="TKE">

     Configure the default volumesnapshot class.

       ```yaml
       kubectl create -f - <<EOF
       apiVersion: snapshot.storage.k8s.io/v1beta1
       kind: VolumeSnapshotClass
       metadata:
         name: cbs-snapclass
         annotations: 
           snapshot.storage.kubernetes.io/is-default-class: "true"
       driver: com.tencent.cloud.csi.cbs
       deletionPolicy: Delete
       EOF
       ```

     </TabItem>
     </Tabs>

3. Create a snapshot backup.

    ```bash
    kbcli cluster backup redis-cluster
    ```

4. Check the backup.

    ```bash
    kbcli cluster list-backups
    ```

5. Restore to a new cluster.

   Copy the backup name to the clipboard, and restore to the new cluster.

   :::note

   You do not need to specify other parameters for creating a cluster. The restoration automatically reads the parameters of the source cluster, including specification, disk size, etc., and creates a new Redis cluster with the same specifications.

   :::

   Execute the following command.

   ```bash
   kbcli cluster restore redis-new-from-snapshot --backup backup-default-redis-cluster-20230411115450
   ```
