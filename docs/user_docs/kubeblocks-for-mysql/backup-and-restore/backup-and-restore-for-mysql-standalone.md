---
title: Backup and restore for MySQL standalone
description: Guide for backup and restore for an ApeCloud MySQL standalone
sidebar_position: 2
---

# Backup and restore for MySQL standalone 
This section shows how to use `kbcli` to back up and restore a MySQL standalone instance.

***Before you start***

- Prepare a clean EKS cluster, and install ebs csi driver plug-in, with at least one node and the memory of each node is not less than 4GB.
- Install `kubectl` to ensure that you can connect to the EKS cluster 
- Install `kbcli`. Refer to [Install kbcli and KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) for details.
   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```

***Steps:***

1. Install KubeBlocks and enable the snapshot backup controller add-on.
   ```bash
   kbcli kubeblocks install --set snapshot-controller.enabled=true
   ```
   If you have installed KubeBlocks without enabling the snapshot controller, you can run the command below to enable snapshot backup.
   ```bash
   kbcli kubeblocks upgrade --set snapshot-controller.enabled=true
   ```   
   Since your `kubectl` is already connected to the EKS cluster, this command installs the latest version of KubeBlocks in your EKS environment.

   Verify the installation with the following command.
   ```bash
   kubectl get pod
   ```

   The pod with kubeblocks-snapshot-controller is shown. See the information below.
   ```
   NAME                                              READY   STATUS             RESTARTS      AGE
   kubeblocks-5c8b9d76d6-m984n                       1/1     Running            0             9m
   kubeblocks-snapshot-controller-6b4f656c99-zgq7g   1/1     Running            0             9m
   ```
2. Configure EKS to support the snapshot function.
   The backup is realized by the volume snapshot function, you need to configure EKS to support the snapshot function.
    - Configure the storage class of snapshot (the assigned ebs volume is gp3).
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
  
       kubectl patch sc/gp2 -p '{"metadata": {"annotations": {"storageclass.kubernetes.io/is-default-class": "false"}}}'
       ```
    - Configure default snapshot volumesnapshot class
       ```bash
       cat <<"EOF" > snapshot_class.yaml
       apiVersion: snapshot.storage.k8s.io/v1
       kind: VolumeSnapshotClass
       metadata:
         name: csi-aws-vsc
         annotations:
           snapshot.storage.kubernetes.io/is-default-class: "true"
       driver: ebs.csi.aws.com
       deletionPolicy: Delete
       EOF
  
       kubectl create -f snapshot_class.yaml
       ```
3. Create a MySQL cluster. 
   ```bash
   kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
   ```
4. Insert test data to test backup.
   Connect to the MySQL cluster created in the previous steps and insert a piece of data. See the example below.
   ```bash
   kbcli cluster connect mysql-cluster
   
   create database if not exists demo;
   create table if not exists demo.msg(id int NOT NULL AUTO_INCREMENT, msg text, time datetime, PRIMARY KEY (id));
   insert into demo.msg (msg, time) value ("hello", now());
   select * from demo.msg;
   ```
  
5. Create a snapshot backup.
    ```bash
    kbcli cluster backup mysql-cluster
    ```
6. Check the backup.
    ```bash
    kbcli cluster list-backups
    ```
7. Restore to a new cluster.
   Copy the backup name to the clipboard, and restore to the new cluster. 
   > ***Note:*** 
   > 
   > You do not need to specify other parameters for creating an cluster. The restoration automatically reads the parameters of the source cluster, including specification, disk size, etc., and create a new MySQL cluster with the same specifications. 

   Execute the following command.
   ```bash
   kbcli cluster restore mysql-new-from-snapshot --backup backup-default-mysql-cluster-20221124113440
   ```
8. Verify the data restored.
   Execute the following command to verify the data restored.
   ```bash
   kbcli cluster connect mysql-new-from-snapshot
   select * from demo.msg;
   ```
9. Delete the ApeCloud MySQL cluster and clean up the backup.
   > ***Note:***
   > 
   > Expense incurred when you have snapshots on the cloud. So it is recommended to delete the test cluster.
  
   Delete MySQL cluster with the following command.
   ```bash
   kbcli cluster delete mysql-cluster
   kbcli cluster delete mysql-new-from-snapshot
   ```
   Delete the backup specified.

   ```bash
   kbcli cluster delete-backup mysql-cluster --name backup-default-mysql-cluster-20221124113440 
   ```
   Delete all backups with `mysql-cluster`.
   ```bash
   kbcli cluster delete mysql-cluster
   ```
