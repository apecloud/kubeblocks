---
title: Backup and restore for PostgreSQL Standalone
description: Guide for backup and restore for a PostgreSQL Standalone
sidebar_position: 2
sidebar_label: PostgreSQL Standalone 
---

# Backup and restore for PostgreSQL standalone

This section shows how to use `kbcli` to back up and restore a PostgreSQL Standalone.

## Configure snapshot backup function

***Before you start***

 Prepare a clean EKS cluster, and install EBS CSI driver plug-in, with at least one node and the memory of each node is not less than 4GB.

***Steps:***

1. Enable the snapshot-controller add-on.

- When installing KubeBlocks, use:

    ```bash
    kbcli kubeblocks install --set snapshot-controller.enabled=true
    ```
- In case you have KubeBlocks installed already, and the snapshot addon is not enabled, use:

    ```
    kbcli addon enable snapshot-controller
    kbcli kubeblocks config --set dataProtection.enableVolumeSnapshot=true
    ```

2. Verify the installation with the following command.

    ```bash
    kbcli addon list 
    ```

    The `snapshot-controller` status is enabled. See the information below.

    ```
    NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   AUTO-INSTALLABLE-SELECTOR
    snapshot-controller            Helm   Enabled                   true
    ```

    If the output result does not show `snapshot-controller`, refer to [Enable add-ons](../../installation/enable-add-ons.md) to find the environment requirements and then enable the snapshot-controller add-on. It may be caused by failing to meet the installation condition of this add-on.


3. Configure Cloud-based Kubernetes managed service to support the snapshot function.
    Now KubeBlocks supports snapshot function in EKS, ACK, and GKE.
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

4. Enable S3 storage with `kbcli addon enable csi-s3`.

    ```
    kbcli addon enable csi-s3 --set secret.accessKey=<your-accesskey>,secret.secretKey=<your-secretkey>,secret.endpoint=https://s3.cn-northwest-1.amazonaws.com.cn,secret.region=cn-northwest-1,storageClass.singleBucket=demo
    ```
 
| Parameters                | Description                                                               |
|:--------------------------|:--------------------------------------------------------------------------|
| secret.accessKey          | S3 access key.                                                            |
| secret.secretKey          | S3 secret key.                                                            |
| secret.endpoint           | S3 access address. Example: AWS global: https://s3.<region>.amazonaws.com |
| secret.region             | S3 region                                                                 |
| storageClass.singleBucket | S3 bucket                                                                 |
| secret.region             | S3 region                                                                 |
| storageClass.singleBucket | S3 bucket                                                                 |

5. Check StorageClass.

    ```
    kubectl get storageclasses
    NAME                        PROVISIONER           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
    csi-s3                      ru.yandex.s3.csi      Retain          Immediate              false                  14m
    ```
6. Configure the automatically created PVC name and storageclass.

    ```
    kbcli kubeblocks config --set dataProtection.backupPVCName=backup-data --set dataProtection.backupPVCStorageClassName=csi-s3
    ```

7. Configure your backup policy. Please make sure you have a PGSQL cluster, if not, create it with `kbcli cluster create` command . In the following code blocks, `my-pg` is the name of the PGSQL cluster.

    ```
    kbcli cluster list-backup-policy
    NAME                             DEFAULT   CLUSTER   CREATE-TIME
    my-pg-postgresql-backup-policy   true      my-pg     Apr 20,2023 18:13 UTC+0800

    # Edit the backup policy and enable incremental log backups
    kbcli cluster edit-backup-policy my-pg-postgresql-backup-policy
    # Find spec.schedule.incremental.enable, change it from false to true
    #Save and exit
    :wq
    ```

## Test backup function

1. Create a empty snapshot backup.

    ```
    kbcli cluster backup my-pg
    ```
    Check it by `kbcli cluster list-backup`.

2. Create user account.

    ```
    kbcli cluster create-account my-pg --username myuser
    +---------+-------------------------------------------------+
    | RESULT  | MESSAGE                                         |
    +---------+-------------------------------------------------+
    | Success | created user: myuser, with password: oJ3bAiK7pr |
    +---------+-------------------------------------------------+
    # Copy the user password generated: oJ3bAiK7pr 
    
    ```

3. Grant roles to the user created.

    ```
    kbcli cluster grant-role my-pg --username myuser --role READWRITE 
    ```

    Connect to the database.

    ```
    kbcli cluster connect my-pg --as-user myuser
    password:  #Copy the user password generated

    ```


4. Insert test data to test backup.
    


    ```bash
   
    create table if not exists msg(id SERIAL PRIMARY KEY, msg text, time timestamp);
    insert into msg (msg, time) values ('hello', now());
    # wait for 5 minutes and insert another row
    insert into msg (msg, time) values ('hello', now());
    # check 
    select * from msg;
    id |  msg  |            time
    ----+-------+----------------------------
     1 | hello | 2023-04-17 11:56:38.269572
     2 | hello | 2023-04-17 11:56:42.988197
    (2 rows)
    ```
  
5. Configure backup to a specified time.

    Check the `RECOVERABLE-TIME`.

    ```bash
    kbcli cluster describe my-pg
    ...
    Data Protection:
    AUTO-BACKUP   BACKUP-SCHEDULE   TYPE       BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME
    Disabled      0 18 * * 0        snapshot   7d           <none>          Apr 17,2023 19:55:48 UTC+0800 ~ Apr 17,2023 19:57:01 UTC+0800
    ```

    :::note
    The recoverable time refreshes every 5 minutes.
    :::

    Choose any time after the recoverable time.

    ```
    kbcli cluster restore new-cluster --restore-to-time "Apr 17,2023 19:56:40 UTC+0800" --source-cluster my-pg
    ```

6. Check the backup data.

    :::note

    PostGreSQL uses patroni and the kernel process restarts after backup. Wait after 30 minutes before connect to the backup cluster.
    :::

    Connect to the backup cluster.

    ```bash
    kbcli cluster connect new-cluster --as-user myuser
    select * from msg;
    id |  msg  |            time
    ----+-------+----------------------------
     1 | hello | 2023-04-17 11:56:38.269572
    (1 row)
    ```
In this example, data inserted before 19:56:40 is restored.


7. (Caution) Delete the PostgreSQL cluster and clean up the backup.
⚠ Data deleted here is only for test. In real senario, deleting backup is a critallt high risk operation.

   
    :::note

    Expenses incurred when you have snapshots on the cloud. So it is recommended to delete the test cluster.

    :::
  
    Delete a PostgreSQL cluster with the following command.

    ```bash
    kbcli cluster delete my-pg
    kbcli cluster delete new-cluster
    ```

    Delete the specified backup.

    ```bash
    kbcli cluster delete-backup my-pg --name backup-default-my-pg-20230417195547
    ```

    Force delete all backups with `my-pg`.
    ```bash
    kbcli cluster delete-backup my-pg --force
    ```
