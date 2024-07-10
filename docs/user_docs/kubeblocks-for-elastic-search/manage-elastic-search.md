---
title: Manage Elasticsearch with KubeBlocks
description: How to manage Elasticsearch on KubeBlocks
keywords: [elasticsearch]
sidebar_position: 1
sidebar_label: Manage Elasticsearch Databases with KubeBlocks
---
# Manage Elasticsearch with KubeBlocks

Elasticsearch is a distributed, RESTful search and analytics engine that is capable of solving an ever-growing number of use cases. As the heart of the Elastic Stack, Elasticsearch stores your data centrally, allowing you to search it quickly, tune relevancy, perform sophisticated analytics, and easily scale.

KubeBlocks supports the management of Elasticsearch.

Before you start, [install kbcli](./../installation/install-with-kbcli/install-kbcli.md) and [install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).

## Create a cluster

***Steps***

1. Execute the following command to create a Qdrant cluster. You can change the `cluster-definition` value as any other databases supported.

   ```bash
   kbcli cluster create elasticsearch --cluster-definition=elasticsearch
   ```

   If you want to create a Elasticsearch cluster with multiple replicas. Use the following command and set the replica numbers.

   ```bash
   kbcli cluster create elasticsearch --cluster-definition=elasticsearch --set replicas=3
   ```

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   elasticsearch   default     elasticsearch        elasticsearch-8.8.2   Delete               Creating          Jul 05,2024 16:51 UTC+0800   
   ```

3. Check the cluster information.

   ```bash
   kbcli cluster describe elasticsearch
   >
   Name: elasticsearch	 Created Time: Jul 05,2024 16:51 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION               STATUS     TERMINATION-POLICY   
   default     elasticsearch        elasticsearch-8.8.2   Creating   Delete               

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
    master          elasticsearch-master-0          <none>   Pending   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    data            elasticsearch-data-0            <none>   Pending   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    ingest          elasticsearch-ingest-0          <none>   Pending   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    elasticsearch   elasticsearch-elasticsearch-0   <none>   Pending   <none>   <none>   Jul 05,2024 16:51 UTC+0800   
    coordinating    elasticsearch-coordinating-0    <none>   Pending   <none>   <none>   Jul 05,2024 16:51 UTC+0800   

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

## Connect to a Elasticsearch cluster

Elasticsearch provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Qdrant cluster.

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe elasticsearch` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose qdant --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose qdant --enable=true --type=internet` to open a public network reachable address for the database cluster.

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
   kbcli cluster update elasticssearch --disableExporter=false
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

Use the following command to perform horizontal scaling.

```bash
kbcli cluster hscale elasticsearch --replicas=5 --components=elasticsearch
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command print the `opsname`, to check the progress of horizontal scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops elasticsearch-horizontalscaling-xpdwz
>
NAME                             TYPE                CLUSTER   STATUS    PROGRESS   AGE
elasticsearch-horizontalscaling-xpdwz   HorizontalScaling   elasticsearch    Running   0/2        16s
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe elasticsearch
```

### Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale elasticsearch --cpu=0.5 --memory=512Mi --components=elasticsearch 
```

Please wait a few seconds until the scaling process is over.
The `kbcli cluster vscale` command print the `opsname`, to check the progress of scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops elasticsearch-verticalscaling-rpw2l
>
NAME                           TYPE              CLUSTER   STATUS    PROGRESS   AGE
elasticsearch-verticalscaling-rpw2l   VerticalScaling   elasticsearch    Running   1/5        44s
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

The `kbcli cluster volume-expand` command print the `opsname`, to check the progress of volume expanding, you can use the following command with the `opsname`.

```bash
kubectl get ops elasticsearch-volumeexpansion-5pbd2
>
NAME                           TYPE              CLUSTER   STATUS   PROGRESS   AGE
elasticsearch-volumeexpansion-5pbd2   VolumeExpansion   elasticsearch    Running  1/1        67s
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe elasticsearch
```

## Backup and restore

The backup and restore operations for Qdrant are the same as those of other clusters and you can refer to [the backup and restore documents](./../backup-and-restore/introduction.md) for details. Remember to use `--method` parameter.
