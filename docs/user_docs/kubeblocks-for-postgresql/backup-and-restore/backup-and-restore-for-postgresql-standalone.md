---
title: Backup and restore for PostgreSQL Standalone
description: Guide for backup and restore for a PostgreSQL Standalone
sidebar_position: 2
sidebar_label: PostgreSQL Standalone 
---

# Backup and restore for PostgreSQL Standalone 
This section shows how to use `kbcli` to back up and restore a PostgreSQL Standalone.

***Before you start***

- Prepare a clean EKS cluster, and install EBS CSI driver plug-in, with at least one node and the memory of each node is not less than 4GB.
- [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) to ensure that you can connect to the EKS cluster.
- Install `kbcli`. Refer to [Install kbcli and KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md) for details.
   ```bash
   curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
   ```

***Steps:***

1. Install KubeBlocks and the snapshot-controller add-on.
    ```bash
    kbcli kubeblocks install --set snapshot-controller.enabled=true
    ```
 
    Since your `kubectl` is already connected to the EKS cluster, this command installs the latest version of KubeBlocks in the default namespace `kb-system` in your EKS environment.

    Verify the installation with the following command.
    ```bash
    kubectl get pod -n kb-system
    ```

    The pod with `kubeblocks` and `kb-addon-snapshot-controller` is shown. See the information below.
    ```
    NAME                                              READY   STATUS             RESTARTS      AGE
    kubeblocks-5c8b9d76d6-m984n                       1/1     Running            0             9m
    kb-addon-snapshot-controller-6b4f656c99-zgq7g     1/1     Running            0             9m
    ```

    If the output result does not show `kb-addon-snapshot-controller`, it means the snapshot-controller add-on is not enabled. It may be caused by failing to meet the installable condition of this add-on. Refer to [Enable add-ons](../../installation/enable-add-ons.md) to find the environment requirements and then enable the snapshot-controller add-on.

2. Configure EKS to support the snapshot function.
    
    The backup is realized by the volume snapshot function, you need to configure EKS to support the snapshot function.
    - Configure the storage class of snapshot (the assigned EBS volume is gp3).
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

3. Create a PostgreSQL Standalone. 
    
    ```bash
    kbcli cluster create pg-cluster --cluster-definition='postgresql'
    ```
4. Insert test data to test backup.
    
    Connect to the PostgreSQL cluster created in the previous steps and insert a piece of data. See the example below.
    ```bash
    kbcli cluster connect pg-cluster
   
    create database if not exists demo;
    create table if not exists demo.msg(id int NOT NULL AUTO_INCREMENT, msg text, time datetime, PRIMARY KEY (id));
    insert into demo.msg (msg, time) value ("hello", now());
    select * from demo.msg;
    ```
  
5. Create a snapshot backup.
    ```bash
    kbcli cluster backup pg-cluster
    ```
6. Check the backup.
    ```bash
    kbcli cluster list-backups
    ```
7. Restore to a new cluster.
    
    Copy the backup name to the clipboard, and restore to the new cluster. 
    
    :::note

    You do not need to specify other parameters for creating a cluster. The restoration automatically reads the parameters of the source cluster, including specification, disk size, etc., and creates a new PostgreSQL cluster with the same specifications. 

    :::

    Execute the following command.
    ```bash
    kbcli cluster restore postgresql-new-from-snapshot --backup backup-default-postgresql-cluster-20221124113440
    ```
8. Verify the data restored.
   
    Execute the following command to verify the data restored.
    ```bash
    kbcli cluster connect postgresql-new-from-snapshot

    select * from demo.msg;
    ```
9. Delete the PostgreSQL cluster and clean up the backup.
   
    :::note

    Expenses incurred when you have snapshots on the cloud. So it is recommended to delete the test cluster.

    :::
  
    Delete a PostgreSQL cluster with the following command.
    ```bash
    kbcli cluster delete pg-cluster
    kbcli cluster delete postgresql-new-from-snapshot
    ```

    Delete the backup specified.

    ```bash
    kbcli cluster delete-backup pg-cluster --name backup-default-pg-cluster-20221124113440 
    ```

    Delete all backups with `pg-cluster`.
    ```bash
    kbcli cluster delete-backup pg-cluster --force
    ```