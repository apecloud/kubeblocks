---
title: Create a PostgreSQL cluster on AWS
description: How to create a PostgreSQL cluster on AWS
sidebar_position: 4
---

# Create a PostgreSQL cluster on AWS

This guide introduces how to use KubeBlocks to create a PostgreSQL cluster based on the Paxos consensus protocol within 5 minutes in the EKS environment.

> ***Caution:***
>
> Running a database cluster on the cloud causes fees. Delete the resources created during the deploying process after operations.

## Before you start

1. EKS environment is required and it includes at least three replicas.
2. `kubectl` is required and can connect to the EKS cluster. For installing `kubectl`, refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.
   
## Step 1. Install `kbcli`

1. Run the command below to install `kbcli`. `kbcli` can run on macOS and Linux.
   ```bash
   curl -fsSL https://kubeblocks.io/installer/install_cli.sh |bash
   ```
   > ***Note:***
   >
   > Please try again if a time-out exception occurs during installation. It may relate to your network condition.
2. Run the command below to check the version and verify whether `kbcli` is installed successfully.
   ```bash
   kbcli version
   ```
3. Run the command below to uninstall `kbcli` if you want to delete `kbcli` after your trial.
   ```bash
   sudo rm /usr/local/bin/kbcli
   ```

## Step 2. Install KubeBlocks

1. Run the command below to install KubeBlock.
   ```bash
   kbcli kubeblocks install --set snapshot-controller.enabled=true --set loadbalancer.enabled=true
   ```
   * `--set snapshot-controller.enabled=true` option enables KubeBlocks to use EBS snapshot for backup and restore and this option is required for the deployment on AWS.
   * If you want the node outside the Kubernetes cluster (the node should be within the same VPC) to visit the database cluster created by KubeBlocks, use `--set loadbalancer.enabled=true` option as the above command does. 
   * For more details on installation, refer to [Install/Uninstall kbcli and KubeBlocks](../install_kbcli_kubeblocks/install_and_unistall_kbcli_and_kubeblocks.md).
   
    ***Result***
    This command installs the latest version in your Kubernetes environment since your kubectl can connect to your Kubernetes clusters.
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
   kubectl get pod
   ```
   ***Result***
   Four pods starting with `kubeblocks` are displayed. For example,
   ```bash
   NAME                                                  READY   STATUS    RESTARTS   AGE
   kubeblocks-7d4c6fd684-9hjh7                           1/1     Running   0          3m33s
   kubeblocks-grafana-b765d544f-wj6c6                    3/3     Running   0          3m33s
   kubeblocks-prometheus-alertmanager-7c558865f5-hsfn5   2/2     Running   0          3m33s
   kubeblocks-prometheus-server-5c89c8bc89-mwrx7         2/2     Running   0          3m33s
   ```
4. Run the command below to uninstall KubeBlocks if you want to delete KubeBlocks after your trial.
   ```bash
   kbcli kubeblocks uninstall
   ```

## Step 3. Create a PostgreSQL standalone

> ***Cautions:***
>
> * If there are not three replicas that exceed the values of memory and CPU, creating a MySQL cluster may fail.
> * KubeBlocks applies for a new EBS volume of 10 Gi (the default storage size), which causes extra fees. Delete this EBS volume after your trial.
> * You can adjust the replica amount, pod memory, CPU kernel amount of your EKS cluster, and EBS volume size by using the `--set` option when creating the cluster.

1. Run the command below to create a PostgreSQL cluster. The cluster name can be customized and `pg-cluster` below is an example. The system assigns a name for this cluster by default. If you want to customize a cluster name, run `kbcli cluster create <name>`.
   For more details on command options, refer to [`kbcli` cluster create options description](../manage_mysql_database_with_kubeblocks/manage_cluster/create_and_connect_a_mysql_cluster.md#create-a-mysql-cluster).
   ```bash
   kbcli cluster create --cluster-definition=apecloud-mysql --set cpu=2000m,memory=1Gi,storage=10Gi,replicas=3
   ```
   ***Result***
   A PostgreSQL standalone with 10 Gi of storage is created.
2. Run the command below to view the created cluster.
   ```bash
   kbcli cluster list
   ```

## Step 4. Connect to the PostgreSQL cluster

1. It takes several minutes to create a cluster. Run `kbcli cluster list` to check the cluster status and when the cluster status is `Running`, the cluster has been created.
2. Run the command below to connect to the leader pod of PostgreSQL. (The leader pod is called leaseholder in other databases.)
   ```bash
   kbcli cluster connect orange24
   ```
   After connecting to the cluster, you can operate the created PostgreSQL cluster as you do in the PostgreSQL client.
If you want to connect to the PostgreSQL cluster using PostgreSQL client or your stress test tool, follow the steps below:
1. Run the command below to get the IP and port of this cluster first
   ```bash
   kbcli cluster describe orange24
   ```
2. Find the Endpoints information in the result.
   ```bash
   Endpoints:
   COMPONENT          MODE             INTERNAL                  EXTERNAL        
   replicasets        ReadWrite        10.100.197.10:3306        <none>
   ```

## Step 5. Delete the PostgreSQL cluster

Run the command below to delete the PostgreSQL cluster.
```bash
kbcli cluster delete orange24
```