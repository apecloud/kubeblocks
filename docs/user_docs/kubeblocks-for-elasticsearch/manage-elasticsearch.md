---
title: Manage Elasticsearch with KubeBlocks
description: How to manage Elasticsearch on KubeBlocks
keywords: [elasticsearch]
sidebar_position: 1
sidebar_label: Manage Elasticsearch with KubeBlocks
---

# Manage Elasticsearch with KubeBlocks

Elasticsearch is a distributed, RESTful search and analytics engine that is capable of solving an ever-growing number of use cases. As the heart of the Elastic Stack, Elasticsearch stores your data centrally, allowing you to search it quickly, tune relevancy, perform sophisticated analytics, and easily scale.

KubeBlocks supports the management of Elasticsearch.

## Before you start

- [Install kbcli](./../installation/install-with-kbcli/install-kbcli.md).
- [Install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
- [Install and enable the elasticsearch addon](./../overview/supported-addons.md#use-addons).

## Create a cluster

***Steps***

1. Execute the following command to create a cluster. You can change the `cluster-definition` value as any other database supported.

   ```bash
   kbcli cluster create elasticsearch elasticsearch
   ```

:::note

View more flags for creating a cluster to create a cluster with customized specifications.
  
```bash
kbcli cluster create --help
```

:::

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS            CREATED-TIME
   elasticsearch   default     elasticsearch        elasticsearch-8.8.2   Delete               Creating          Jul 05,2024 16:51 UTC+0800   
   ```

3. Check the cluster information.

   ```bash
   kbcli cluster describe elasticsearch
   >
   Name: elasticsearch	 Created Time: Jul 05,2024 16:51 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION               STATUS    TERMINATION-POLICY   
   default     elasticsearch        elasticsearch-8.8.2   Running   Delete               

   Endpoints:
   COMPONENT       MODE        INTERNAL                                                     EXTERNAL   
   elasticsearch   ReadWrite   elasticsearch-elasticsearch.default.svc.cluster.local:9200   <none>     
                            elasticsearch-elasticsearch.default.svc.cluster.local:9300              
                            elasticsearch-elasticsearch.default.svc.cluster.local:9114              
    coordinating    ReadWrite   elasticsearch-coordinating.default.svc.cluster.local:9200    <none>     
                            elasticsearch-coordinating.default.svc.cluster.local:9300               
    ingest          ReadWrite   elasticsearch-ingest.default.svc.cluster.local:9200          <none>     
                            elasticsearch-ingest.default.svc.cluster.local:9300                     
    data            ReadWrite   elasticsearch-data.default.svc.cluster.local:9200            <none>     
                            elasticsearch-data.default.svc.cluster.local:9300                       
    master          ReadWrite   elasticsearch-master.default.svc.cluster.local:9200          <none>     
                            elasticsearch-master.default.svc.cluster.local:9300                     

    Topology:
    COMPONENT       INSTANCE                        ROLE     STATUS    AZ       NODE     CREATED-TIME                 
    master          elasticsearch-master-0          <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    data            elasticsearch-data-0            <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    ingest          elasticsearch-ingest-0          <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    elasticsearch   elasticsearch-elasticsearch-0   <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    coordinating    elasticsearch-coordinating-0    <none>   Running   <none>   <none>   Jul 05,2024 16:51 UTC+0800   

    Resources Allocation:
    COMPONENT       DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS     
    elasticsearch   false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    coordinating    false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    ingest          false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    data            false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   
    master          false       1 / 1                1Gi / 1Gi               data:20Gi      csi-hostpath-sc   

    Images:
    COMPONENT       TYPE            IMAGE                                   
    elasticsearch   elasticsearch   docker.io/bitnami/elasticsearch:8.8.2   
    coordinating    coordinating    docker.io/bitnami/elasticsearch:8.8.2   
    ingest          ingest          docker.io/bitnami/elasticsearch:8.8.2   
    data            data            docker.io/bitnami/elasticsearch:8.8.2   
    master          master          docker.io/bitnami/elasticsearch:8.8.2   

    Data Protection:
    BACKUP-REPO   AUTO-BACKUP   BACKUP-SCHEDULE   BACKUP-METHOD   BACKUP-RETENTION   

    Show cluster events: kbcli cluster list-events -n default elasticsearch
   ```

## Connect to an Elasticsearch cluster

Elasticsearch provides the HTTP protocol for client access on port 9200. You can visit the cluster by the local host.

```bash
curl http://127.0.0.1:9200/_cat/nodes?v
```

## Monitor the Elasticsearch cluster

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
   kubectl get cluster elasticsearch -o yaml
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
   kbcli cluster update elasticssearch --disable-exporter=false
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

The scaling function for vector databases is also supported.

### Scale horizontally

Horizontal scaling changes the amount of pods. For example, you can scale out replicas from three to five. 

From v0.9.0, besides replicas, KubeBlocks also supports scaling in and out instances, refer to [Horizontal Scale](./../../api_docs/maintenance/scale/horizontal-scale.md) in API docs for more details and examples.

Use the following command to perform horizontal scaling.

```bash
kbcli cluster hscale elasticsearch --replicas=2 --components=elasticsearch
```

- `--components` describes the component name ready for horizontal scaling.
- `--replicas` describes the replica amount of the specified components. Edit the amount based on your demands to scale in or out replicas.

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops elasticsearch-horizontalscaling-xpdwz -n demo
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe elasticsearch
```

### Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale elasticsearch --cpu=2 --memory=3Gi --components=elasticsearch 
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster vscale` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops elasticsearch-verticalscaling-rpw2l
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe elasticsearch
```

## Volume Expansion

***Steps:***

```bash
kbcli cluster volume-expand elasticsearch --storage=40Gi --components=elasticsearch -t data
```

The volume expansion may take a few minutes.

The `kbcli cluster volume-expand` command prints a command to help check the progress of scaling operations.

```bash
kbcli cluster describe-ops elasticsearch-volumeexpansion-5pbd2 -n default
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe elasticsearch
```

## Restart

1. Restart a cluster.

   Configure the values of `components` and `ttlSecondsAfterSucceed` and run the command below to restart a specified cluster.

   ```bash
   kbcli cluster restart elasticsearch --components="elasticsearch" \
   --ttlSecondsAfterSucceed=30
   ```

   - `components` describes the component name that needs to be restarted.
   - `ttlSecondsAfterSucceed` describes the time to live of an OpsRequest job after the restarting succeeds.

2. Validate the restarting.

   Run the command below to check the cluster status to check the restarting status.

   ```bash
   kbcli cluster list elasticsearch
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION          VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   elasticsearch   default     elasticsearch               elasticsearch-8.8.2   Delete               Running   Jul 05,2024 17:51 UTC+0800
   ```

   * STATUS=Updating: it means the cluster restart is in progress.
   * STATUS=Running: it means the cluster has been restarted.
