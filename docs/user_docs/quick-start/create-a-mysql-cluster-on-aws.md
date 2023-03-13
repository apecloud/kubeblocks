---
title: Create a MySQL cluster on AWS
description: How to create a MySQL cluster on AWS
sidebar_position: 3
---

# Create a MySQL cluster on AWS

This guide introduces how to use KubeBlocks to create an ApeCloud MySQL cluster within 5 minutes in the EKS environment.

> ***Caution:*** 
> 
> Running a database cluster on the cloud causes fees. Delete the resources created during the deploying process after operations.

## Before you start

1. EKS environment is required and it includes at least three nodes. Amazon EKS CSI driver should also be installed.
2. `kubectl` is required and can connect to the EKS cluster. For installing `kubectl`, refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.

## Step 1. Install `kbcli`

1. Run the command below to install `kbcli`. `kbcli` can run on macOS and Linux.
   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
   ```
   > ***Note:*** 
   > 
   > Please try again if a time-out exception occurs during installation. It may relate to your network condition.
2. Run the command below to check the version and verify whether `kbcli` is installed successfully.
   ```bash
   kbcli version
   ```

## Step 2. Install KubeBlocks

1. Run the command below to install KubeBlock.
   ```bash
   kbcli kubeblocks install
   ```

    ***Result***

    This command installs the latest version in your Kubernetes environment under the default namespace `kb-system` since your `kubectl` can connect to your Kubernetes clusters.

2. Run the YAML files below to apply for EBS resources and enable backup.
   * Apply for EBS resources:
      ```bash
      kubectl apply -f - <<EOF
      kind: StorageClass
      apiVersion: storage.k8s.io/v1
      metadata:
       name: gp3
       annotations:
       storageclass.kubernetes.io/is-default-class: "true"
      allowVolumeExpansion: true
      provisioner: ebs.csi.aws.com
      volumeBindingMode: WaitForFirstConsumer
      parameters:
       type: gp3
      EOF
      ```
   * Enable backup:
     ```bash
     kubectl apply -f - <<EOF
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
3. Run the command below to verify whether KubeBlocks is installed successfully.
   ```bash
   kubectl get pod -n kb-system
   ```

   ***Result***

   When the following pods are `Running`, it means KubeBlocks is installed successfully.

   ```bash
   NAME                                                     READY   STATUS      RESTARTS   AGE
   kb-addon-alertmanager-webhook-adaptor-5549f94599-fsnmc   2/2     Running     0          84s
   kb-addon-grafana-5ddcd7758f-x4t5g                        3/3     Running     0          84s
   kb-addon-prometheus-alertmanager-0                       2/2     Running     0          84s
   kb-addon-prometheus-server-0                             2/2     Running     0          84s
   kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s
   ```

## Step 3. Create a MySQL Paxos group

> ***Caution:***
> 
> * If there are not three nodes that exceed the values of memory and CPU, creating a MySQL cluster may fail.
> * KubeBlocks applies for a new EBS volume of 10 Gi (the default storage size), which causes extra fees. Delete this EBS volume after your trial.
> * You can adjust the replica amount, pod memory, CPU kernel amount of your EKS cluster, and EBS volume size by using the `--set` option when creating the cluster.

1. Run the command below to create a MySQL cluster. The system assigns a name for this cluster by default. If you want to customize a cluster name, run `kbcli cluster create <name>`.
   For more details on options, refer to [`kbcli` cluster create options description](./../kubeblocks-for-mysql/cluster-management/create-and-connect-a-mysql-cluster.md#create-a-mysql-cluster).

   ```bash
   kbcli cluster create --cluster-definition=apecloud-mysql
   ```

   ***Result***

   A MySQL standalone with 10 Gi of storage is created. 

2. Run the command below to view the created cluster.
   ```bash
   kbcli cluster list
   ```

## Step 4. Connect to the MySQL cluster

1. It takes several minutes to create a cluster. Run `kbcli cluster list` to check the cluster status and when the cluster status is `Running`, the cluster has been created. 
2. Run the command below to connect to the leader pod of MySQL clsuter. (The leader pod is called leaseholder in other databases.)
   ```bash
   kbcli cluster connect maple05
   ```

After connecting to the cluster, you can operate the created MySQL cluster as you do in the MySQL client.

If you want to connect to the MySQL cluster using MYSQL client or your stress test tool, follow the steps below: 
1. Run the command below to get the IP and port of this cluster first. 
   ```bash
   kbcli cluster describe maple05
   ```
2. Find the Endpoints information in the result.
   ```
   Endpoints:
   COMPONENT   MODE        INTERNAL                                       EXTERNAL
   mysql       ReadWrite   tulip89-mysql.default.svc.cluster.local:3306   <none>
   ```

The MySQL cluster provides high availability to ensure RPO=0. When a failure occurs to the MySQL leader pod, other MySQL pods can be elected as the succeeding leader based on the Paxos protocol. The connection address does not change even though the leader pod changes.

## Step 5. Clean up the environment

Run the commands below if you want to delete the created cluster and uninstall `kbcli` and KubeBlocks after your trial.

1. Delete the PostgreSQL cluster.
   ```bash
   kbcli cluster delete orange24
   ```

2. Uninstall KubeBlocks.
   ```bash
   kbcli kubeblocks uninstall
   ```

3. Uninstall `kbcli`.
   ```bash
   sudo rm /usr/local/bin/kbcli
   ```