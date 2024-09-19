---
title: Manage StarRocks with KubeBlocks
description: How to manage StarRocks on KubeBlocks
keywords: [starrocks, analytic, data warehouse, control plane]
sidebar_position: 1
sidebar_label: Manage StarRocks with KubeBlocks
---

# Manage StarRocks with KubeBlocks

StarRocks is a next-gen, high-performance analytical data warehouse that enables real-time, multi-dimensional, and highly concurrent data analysis.

KubeBlocks supports the management of StarRocks.

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md).
- [Install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
- [Install and enable the starrocks addon](./../installation/install-with-kbcli/install-addons.md).

## Create a cluster

***Steps***

1. Execute the following command to create a StarRocks cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks
   ```

   You can also create a cluster with specified CPU, memory and storage values.

   ```bash
   kbcli cluster create mycluster --cluster-definition=starrocks --set cpu=1,memory=2Gi,storage=10Gi
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
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS     CREATED-TIME
   mycluster   default     starrocks            starrocks-3.1.1   Delete               Running    Jul 17,2024 19:06 UTC+0800   
   ```

2. Check the cluster information.

   ```bash
    kbcli cluster describe mycluster
    >
    Name: mycluster	 Created Time: Jul 17,2024 19:06 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
    default     starrocks            starrocks-3.1.1   Running   Delete

    Endpoints:
    COMPONENT   MODE        INTERNAL                                      EXTERNAL
    fe          ReadWrite   mycluster-fe.default.svc.cluster.local:9030   <none>

    Topology:
    COMPONENT   INSTANCE         ROLE     STATUS    AZ       NODE                    CREATED-TIME
    be          mycluster-be-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800
    fe          mycluster-fe-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 17,2024 19:06 UTC+0800

    Resources Allocation:
    COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    fe          false       1 / 1                1Gi / 1Gi               data:20Gi      standard
    be          false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT   TYPE   IMAGE
    fe          fe     docker.io/starrocks/fe-ubuntu:2.5.4
    be          be     docker.io/starrocks/be-ubuntu:2.5.4

    Show cluster events: kbcli cluster list-events -n default mycluster
   ```

## Scale

### Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale mycluster --cpu=2 --memory=20Gi --components=be
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster vscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-verticalscaling-smx8b -n default
```

Validate the vertical scale operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster
```

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

Use the following command to perform horizontal scaling.

```bash
kbcli cluster hscale mycluster --replicas=3 --components=be
```

- `--components` describes the component name ready for horizontal scaling.
- `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-horizontalscaling-smx8b -n default
```

Validate the horizontal scale operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster
```

## Volume expansion

Use the following command to perform volume expansion.

```bash
kbcli cluster volume-expand mycluster --storage=40Gi --components=be
```

The volume expansion may take a few minutes.

The `kbcli cluster volume-expand` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops mycluster-volumeexpansion-smx8b -n default
```

Validate the volume expansion operation. When the cluster is running again, the operation is completed.

```bash
kbcli cluster describe mycluster
```

## Restart

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart mycluster --components="starrocks" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION     VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     starrocks               starrocks-3.1.1    Delete               Running   Jul 17,2024 19:06 UTC+0800
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
