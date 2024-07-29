---
title: Manage Weaviate with KubeBlocks
description: How to manage Weaviate on KubeBlocks
keywords: [weaviate, vector database, control plane]
sidebar_position: 1
sidebar_label: Manage Weaviate with KubeBlocks
---

# Manage Weaviate with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. Weaviate is an open-source vector database that simplifies the development of AI applications. Built-in vector and hybrid search, easy-to-connect machine learning models, and a focus on data privacy enable developers of all levels to build, iterate, and scale AI capabilities faster.

KubeBlocks supports the management of Weaviate.

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md).
- [Install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
- [Install and enable the weaviate addon](./../overview/supported-addons.md#use-addons).

## Create a cluster

***Steps***

1. Execute the following command to create a Weaviate cluster. You can change the `cluster-definition` value as any other databases supported.

   ```bash
   kbcli cluster create weaviate --cluster-definition=weaviate
   ```

   If you want to create a Weaviate cluster with multiple replicas. Use the following command and set the replica numbers.

   ```bash
   kbcli cluster create weaviate --cluster-definition=weaviate --set replicas=3
   ```

:::note

View more flags for creating a MySQL cluster to create a cluster with customized specifications.
  
```bash
kbcli cluster create --help
```

:::

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS           CREATED-TIME
   weaviate        default     weaviate             weaviate-1.18.0       Delete               Running          Jul 05,2024 17:42 UTC+0800   
   ```

3. Check the cluster information.

   ```bash
    kbcli cluster describe weaviate
    >
    Name: weaviate	 Created Time: Jul 05,2024 17:42 UTC+0800
    NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
    default     weaviate             weaviate-1.18.0   Running   Delete

    Endpoints:
    COMPONENT   MODE        INTERNAL                                           EXTERNAL
    weaviate    ReadWrite   weaviate-weaviate.default.svc.cluster.local:8080   <none>

    Topology:
    COMPONENT   INSTANCE              ROLE     STATUS    AZ       NODE                    CREATED-TIME
    weaviate    weaviate-weaviate-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 05,2024 17:42 UTC+0800

    Resources Allocation:
    COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
    weaviate    false       1 / 1                1Gi / 1Gi               data:20Gi      standard

    Images:
    COMPONENT   TYPE       IMAGE
    weaviate    weaviate   docker.io/semitechnologies/weaviate:1.19.6

    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   RECOVERABLE-TIME 
   ```

## Connect to a Weaviate cluster

Weaviate provides the HTTP protocol for client access on port 8080. You can visit the cluster by the local host.

```bash
curl http://localhost:8080/v1/meta | jq
```

## Monitor the database

For the testing environment, you can run the command below to open the Grafana monitor web page.

1. View all built-in addons and make sure the monitoring addons are enabled. If the monitoring addons are not enabled, [enable these addons](./../overview/supported-addons.md#use-addons) first.

   ```bash
   # View all addons supported
   kbcli addon list
   ...
   grafana                        Helm   Enabled                   true                                                                                    
   alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
   prometheus                     Helm   Enabled    alertmanager   true 
   ...
   ```

2. Check whether the monitoring function of the cluster is enabled. If the monitoring function is enabled, the output shows `disableExporter: false`.

   ```bash
   kubectl get cluster weaviate -o yaml
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
   ......
   spec:
     ......
     componentSpecs:
     ......
       disableExporter: false
   ```

   If `disableExporter: false` is not shown in the output, it means the monitoring function of this cluster is not enabled and you need to enable it first.

   ```bash
   kbcli cluster update weaviate --disable-exporter=false
   ```

3. View the dashboard list.

   ```bash
   kbcli dashboard list
   >
   NAME                                 NAMESPACE   PORT    CREATED-TIME
   kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
   ```

4. Open and view the web console of a monitoring dashboard. For example,

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

For the production environment, it is highly recommended to build your monitoring system or purchase a third-party monitoring service and you can refer to [the monitoring document](./../observability/monitor-database.md#for-production-environment) for details.

## Scale

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five. The scaling process includes the backup and restore of data.

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../maintaince/scale/horizontal-scale.md) for more details and examples.

Use the following command to perform horizontal scaling.

```bash
kbcli cluster hscale weaviate --replicas=3 --components=weaviate
```

- `--components` describes the component name ready for horizontal scaling.
- `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops weaviate-horizontalscaling-xpdwz -n default
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe weaviate
```

### Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale weaviate --cpu=0.5 --memory=512Mi --components=weaviate 
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster vscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops weaviate-verticalscaling-rpw2l -n default
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe weaviate
```

## Volume Expansion

***Steps:***

```bash
kbcli cluster volume-expand weaviate --storage=40Gi --components=weaviate -t data
```

The volume expansion may take a few minutes.

The `kbcli cluster volume-expand` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops weaviate-volumeexpansion-5pbd2 -n default
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe weaviate
```

## Restart

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart weaviate --components="weaviate" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list weaviate
   >
   NAME       NAMESPACE   CLUSTER-DEFINITION     VERSION            TERMINATION-POLICY   STATUS    CREATED-TIME
   weaviate   default     weaviate               weaviate-1.18.0    Delete               Running   Jul 05,2024 18:42 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.

## Stop/Start a cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

### Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

   ```bash
   kbcli cluster stop weaviate
   ```

2. Check the status of the cluster to see whether it is stopped.

    ```bash
    kbcli cluster list
    ```

### Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.

   ```bash
   kbcli cluster start weaviate
   ```

2. Check the status of the cluster to see whether it is running again.

    ```bash
    kbcli cluster list
    ```
