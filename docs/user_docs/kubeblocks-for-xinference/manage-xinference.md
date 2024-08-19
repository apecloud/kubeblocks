---
title: Manage Xinference with KubeBlocks
description: How to manage Xinference on KubeBlocks
keywords: [xinference, LLM, AI, control plane]
sidebar_position: 1
sidebar_label: Manage Xinference with KubeBlocks
---

# Manage Xinference with KubeBlocks

Xorbits Inference (Xinference) is an open-source platform to streamline the operation and integration of a wide array of AI models. With Xinference, you’re empowered to run inference using any open-source LLMs, embedding models, and multimodal models either in the cloud or on your premises, and create robust AI-driven applications.

KubeBlocks supports the management of Xinference.

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md).
- [Install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
- [Install and enable the xinference addon](./../overview/supported-addons.md#use-addons).

## Create a cluster

***Steps***

1. Execute the following command to create a Xinference cluster. You can change the `cluster-definition` value as any other database supported.

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference
   ```

   If you want to create a Xinference cluster with multiple replicas. Use the following command and set the replica numbers.

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference --set replicas=3
   ```

   You can also create a cluster with specified CPU, memory and storage values.

   ```bash
   kbcli cluster create mycluster --cluster-definition=xinference --set cpu=1,memory=2Gi,storage=10Gi
   ```

:::note

View more flags for creating a cluster to create a cluster with customized specifications.
  
```bash
kbcli cluster create --help
```

:::

1. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     xinference           xinference-0.11.0   Delete               Running   Jul 17,2024 17:24 UTC+0800   
   ```

2. Check the cluster information.

   ```bash
    kbcli cluster describe mycluster
    >
    Name: mycluster	 Created Time: Jul 17,2024 17:29 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION             STATUS    TERMINATION-POLICY
    default     xinference           xinference-0.11.0   Running   Delete

    Endpoints:
    COMPONENT    MODE        INTERNAL                                              EXTERNAL
    xinference   ReadWrite   mycluster-xinference.default.svc.cluster.local:9997   <none>

    Topology:
    COMPONENT    INSTANCE                 ROLE     STATUS    AZ       NODE                    CREATED-TIME
    xinference   mycluster-xinference-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 17:29 UTC+0800

    Resources Allocation:
    COMPONENT    DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    xinference   false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT    TYPE         IMAGE
    xinference   xinference   docker.io/xprobe/xinference:v0.11.0

    Show cluster events: kbcli cluster list-events -n default mycluster
   ```

## Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale mycluster --cpu=0.5 --memory=512Mi --components=xinference 
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster vscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n default
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe mycluster
```

## Restart

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster --components="xinference" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster
   >
   NAME         NAMESPACE   CLUSTER-DEFINITION     VERSION              TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster    default     xinference             xinference-0.11.0    Delete               Running   Jul 05,2024 18:42 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

### Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

   ```bash
   kbcli cluster stop mycluster
   ```

2. Check the status of the cluster to see whether it is stopped.

    ```bash
    kbcli cluster list
    ```

### Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.

   ```bash
   kbcli cluster start mycluster
   ```

2. Check the status of the cluster to see whether it is running again.

    ```bash
    kbcli cluster list
    ```
