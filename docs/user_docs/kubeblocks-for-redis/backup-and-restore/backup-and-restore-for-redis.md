---
title: Backup and restore for Redis Standalone
description: Guide for backup and restore for a Redis cluster
keywords: [redis, backup, restore]
sidebar_position: 2
sidebar_label: Redis
---

# Backup and restore for Redis cluster

This section shows how to use `kbcli` to back up and restore a Redis cluster.

***Before you start***

- Prepare a clean EKS cluster, and install ebs csi driver plug-in, with at least one node and the memory of each node is not less than 4GB.
- [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/#kubectl) to ensure that you can connect to the EKS cluster.
- Install `kbcli`. Refer to [Install kbcli and KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) for details.

***Steps:***

1. Install KubeBlocks and the snapshot-controller add-on.

     ```bash
     kbcli kubeblocks install --set snapshot-controller.enabled=true
     ```

     If you have installed KubeBlock without enabling the snapshot-controller, run the command below.

     ```bash
     kbcli kubeblocks upgrade --set snapshot-controller.enabled=true
     ```

     Since your `kubectl` is already connected to the EKS cluster, this command installs the latest version of KubeBlocks in the default namespace `kb-system` in your EKS environment.

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

     If the output result does not show `kb-addon-snapshot-controller`, it means the snapshot-controller add-on is not enabled. It may be caused by failing to meet the installable condition of this add-on. Refer to [Enable add-ons](./../../installation/enable-add-ons.md) to find the environment requirements and then enable the snapshot-controller add-on.

2. Configure EKS to support the snapshot function.

     The backup is realized by the volume snapshot function, you need to configure EKS to support the snapshot function.

     Configure the storage class of the snapshot (the assigned EBS volume is gp3).

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

3. Create a Redis cluster.

     ```bash
     kbcli cluster create redis-cluster --cluster-definition='redis'
     ```

     View your cluster.

     ```bash
     kbcli cluster list
     ```

     For details on creating a cluster, refer to [Create a Redis cluster](./../../kubeblocks-for-redis/cluster-management/create-and-connect-a-redis-cluster.md).
4. Insert test data to test backup.

     Connect to the Redis cluster created in the previous steps and insert a piece of data. See the example below.

     ```bash
     # Connect to database
     kbcli cluster connect redis-cluster
     
     # Insert test data
     set demo demo_msg
     ```
  
5. Create a snapshot backup.

    ```bash
    kbcli cluster backup redis-cluster
    ```

6. Check the backup.

    ```bash
    kbcli cluster list-backups
    ```

7. Restore to a new cluster.

   Copy the backup name to the clipboard, and restore to the new cluster.

   :::note

   You do not need to specify other parameters for creating a cluster. The restoration automatically reads the parameters of the source cluster, including specification, disk size, etc., and creates a new Redis cluster with the same specifications.

   :::

   Execute the following command.

   ```bash
   kbcli cluster restore redis-new-from-snapshot --backup backup-default-redis-cluster-20230411115450
   ```

8. Verify the data restored.

     Execute the following command to verify the data restored.

     ```bash
     kbcli cluster connect redis-new-from-snapshot

     get demo
     ```

9. Delete the Redis cluster and clean up the backup.

   :::note

   Expenses incurred when you have snapshots on the cloud. So it is recommended to delete the test cluster.

   :::
  
   Delete the Redis cluster with the following command.

   ```bash
   kbcli cluster delete redis-cluster
   kbcli cluster delete redis-new-from-snapshot
   ```

   Delete the backup specified.

   ```bash
   kbcli cluster delete-backup redis-cluster --name backup-default-redis-cluster-20230411115450 
   ```

   Delete all backups with `redis-cluster`.

   ```bash
   kbcli cluster delete redis-cluster --force
   ```
