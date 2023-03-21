---
title: Create a PostgreSQL cluster on AWS
description: How to create a PostgreSQL cluster on AWS
sidebar_position: 4
sidebar_label: PostgreSQL cluster on AWS
---

# Create a PostgreSQL cluster on AWS

This guide introduces how to use KubeBlocks to create a PostgreSQL Standalone within 5 minutes in the EKS environment.

:::caution

Running a database cluster on the cloud causes fees. Delete the resources created during the deploying process after operations.

:::

## Before you start

1. EKS environment is required and it includes at least three replicas.
2. `kubectl` is required and can connect to the EKS cluster. For installing `kubectl`, refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.
   
## Step 1. Install kbcli

1. Run the command below to install `kbcli`. `kbcli` can run on macOS and Linux.
    ```bash
    curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh |bash
    ```

    :::note

    Please try again if a time-out exception occurs during installation. It may relate to your network condition.

    :::
   
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
2. Run the command below to verify whether KubeBlocks is installed successfully.
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

## Step 3. Create a PostgreSQL Standalone

:::caution

* If there are not three replicas that exceed the values of memory and CPU, creating a PostgreSQL cluster may fail.
* KubeBlocks applies for a new EBS volume of 10 Gi (the default storage size), which causes extra fees. Delete this EBS volume after your trial.
* You can adjust the pod memory, CPU kernel amount of your EKS cluster, and EBS volume size by using the `--set` option when creating the cluster.

:::

1. Run the command below to create a PostgreSQL Standalone. The cluster name can be customized and `pg-cluster` below is an example. The system assigns a name for this cluster by default. If you want to customize a cluster name, run `kbcli cluster create <name>`.
    For more details on command options, refer to [`kbcli` cluster create options description](./../kubeblocks-for-postgresql/cluster-management/create-and-connect-a-postgresql-cluster.md#create-a-postgresql-cluster).

    ```bash
    kbcli cluster create --cluster-definition=postgresql
    ```

    ***Result***
   
    A PostgreSQL Standalone with 10 Gi of storage is created.
2. Run the command below to view the created cluster.
   ```bash
   kbcli cluster list
   ```

## Step 4. Connect to the PostgreSQL Standalone

1. It takes several minutes to create a cluster. Run `kbcli cluster list` to check the cluster status and when the cluster status is `Running`, the Standaloner has been created.
2. Run the command below to connect to the PostgreSQL Standalone. 
    ```bash
    kbcli cluster connect orange24
    ```
    After connecting to the cluster, you can operate the created PostgreSQL Standalone as you do in the PostgreSQL client.
If you want to connect to the PostgreSQL cluster using the PostgreSQL client or your stress test tool, follow the steps below:
1. Run the command below to get the IP and port of this cluster first
    ```bash
    kbcli cluster describe orange24
    ```
2. Find the Endpoints information in the result.
    ```bash
    Endpoints:
    COMPONENT       MODE        INTERNAL                                                EXTERNAL
    pg-replicaset   ReadWrite   orange24-pg-replicaset.default.svc.cluster.local:5432   <none>
    ```

## Step 5. Clean up the environment

Run the commands below if you want to delete the created cluster and uninstall `kbcli` and KubeBlocks after your trial.

1. Delete the PostgreSQL Standalone.
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