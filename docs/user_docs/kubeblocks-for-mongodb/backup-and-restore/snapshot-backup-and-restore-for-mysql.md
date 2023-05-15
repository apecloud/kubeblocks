---
title: Snapshot backup and restore for MySQL
description: Guide for backup and restore for MySQL
keywords: [mysql, snapshot, backup, restore]
sidebar_position: 2
sidebar_label: Snapshot backup and restore
---

# Snapshot backup and restore for MySQL

This section shows how to use `kbcli` to back up and restore a MySQL cluster.

***Before you start***

- Prepare a clean EKS cluster, and install ebs csi driver plug-in, with at least one node and the memory of each node is not less than 4GB.
- [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) to ensure that you can connect to the EKS cluster
- [Install `kbcli`](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md#install-kbcli).

   ```bash
   curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
   ```

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

     ```
     NAME                                              READY   STATUS             RESTARTS      AGE
     kubeblocks-5c8b9d76d6-m984n                       1/1     Running            0             9m
     kb-addon-snapshot-controller-6b4f656c99-zgq7g     1/1     Running            0             9m
     ```

     If the output result does not show `kb-addon-snapshot-controller`, it means the snapshot-controller add-on is not enabled. It may be caused by failing to meet the installable condition of this add-on. Refer to [Enable add-ons](../../installation/enable-add-ons.md) to find the environment requirements and then enable the snapshot-controller add-on.

2. Configure EKS to support the snapshot function.

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

3. Create a MySQL cluster.

     ```bash
     kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
     ```

     View your cluster.

     ```bash
     kbcli cluster list
     ```

     For details on creating a cluster, refer to [Create a Redis cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md#create-a-mysql-cluster).

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

   :::note

   You do not need to specify other parameters for creating a cluster. The restoration automatically reads the parameters of the source cluster, including specification, disk size, etc., and creates a new MySQL cluster with the same specifications.

   :::

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
