---
title: Manage Qdrant with KubeBlocks
description: How to manage Qdrant on KubeBlocks
keywords: [qdrant, vector database, control plane]
sidebar_position: 1
sidebar_label: Manage Qdrant with KubeBlocks
---

# Manage Qdrant with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. Qdrant (read: quadrant) is a vector similarity search engine and vector database. It provides a production-ready service with a convenient API to store, search, and manage points—vectors with an additional payload Qdrant is tailored to extended filtering support. It makes it useful for all sorts of neural-network or semantic-based matching, faceted search, and other applications.

KubeBlocks supports the management of Qdrant.

Before you start, [install kbcli](./../installation/install-with-kbcli/install-kbcli.md) and [install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).

## Create a cluster

***Steps***

1. Execute the following command to create a Qdrant cluster. 

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant
   ```

   If you want to create a Qdrant cluster with multiple replicas. Use the following command and set the replica numbers.

   ```bash
   kbcli cluster create qdrant --cluster-definition=qdrant --set replicas=3
   ```

2. Check whether the cluster is created.

   ```bash
   kbcli cluster list
   >
   NAME     NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
   qdrant   default     qdrant               qdrant-1.5.0   Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

3. Check the cluster information.

   ```bash
   kbcli cluster describe qdrant
   >
   Name: qdrant         Created Time: Aug 15,2023 23:03 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION        STATUS    TERMINATION-POLICY
   default     qdrant               qdrant-1.5.0   Running   Delete

   Endpoints:
   COMPONENT   MODE        INTERNAL                                       EXTERNAL
   qdrant      ReadWrite   qdrant-qdrant.default.svc.cluster.local:6333   <none>
                           qdrant-qdrant.default.svc.cluster.local:6334

   Topology:
   COMPONENT   INSTANCE          ROLE     STATUS    AZ       NODE                   CREATED-TIME
   qdrant      qdrant-qdrant-0   <none>   Running   <none>   x-worker3/172.20.0.3   Aug 15,2023 23:03 UTC+0800
   qdrant      qdrant-qdrant-1   <none>   Running   <none>   x-worker2/172.20.0.5   Aug 15,2023 23:03 UTC+0800
   qdrant      qdrant-qdrant-2   <none>   Running   <none>   x-worker/172.20.0.2    Aug 15,2023 23:04 UTC+0800

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
   qdrant      false       1 / 1                1Gi / 1Gi               data:20Gi      standard

   Images:
   COMPONENT   TYPE     IMAGE
   qdrant      qdrant   docker.io/qdrant/qdrant:latest

   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE     BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME
   Disabled      <none>            <none>   7d           <none>          <none>

   Show cluster events: kbcli cluster list-events -n default qdrant
   ```

## Connect to a Qdrant cluster

Qdrant provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Qdrant cluster.

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe qdrant` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose qdrant --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose qdrant --enable=true --type=internet` to open a public network reachable address for the database cluster.

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
   kubectl get cluster qdrant -o yaml
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
   kbcli cluster update qdrant --disableExporter=false
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
kbcli cluster hscale qdrant --replicas=5 --components=qdrant
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command print the `opsname`, to check the progress of horizontal scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops qdrant-horizontalscaling-xpdwz
>
NAME                             TYPE                CLUSTER   STATUS    PROGRESS   AGE
qdrant-horizontalscaling-xpdwz   HorizontalScaling   qdrant    Running   0/2        16s
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe qdrant
```

### Scale vertically

Use the following command to perform vertical scaling.

```bash
kbcli cluster vscale qdrant --cpu=0.5 --memory=512Mi --components=qdrant 
```

Please wait a few seconds until the scaling process is over.
The `kbcli cluster vscale` command print the `opsname`, to check the progress of scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops qdrant-verticalscaling-rpw2l
>
NAME                           TYPE              CLUSTER   STATUS    PROGRESS   AGE
qdrant-verticalscaling-rpw2l   VerticalScaling   qdrant    Running   1/5        44s
```

To check whether the scaling is done, use the following command.

```bash
kbcli cluster describe qdrant
```

## Volume Expansion

***Steps:***

```bash
kbcli cluster volume-expand qdrant --storage=40Gi --components=qdrant -t data
```

The volume expansion may take a few minutes.

The `kbcli cluster volume-expand` command print the `opsname`, to check the progress of volume expanding, you can use the following command with the `opsname`.

```bash
kubectl get ops qdrant-volumeexpansion-5pbd2
>
NAME                           TYPE              CLUSTER   STATUS   PROGRESS   AGE
qdrant-volumeexpansion-5pbd2   VolumeExpansion   qdrant    Running  1/1        67s
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe qdrant
```

## Backup and restore

The backup and restore operations for Qdrant are the same as those of other clusters and you can refer to [the backup and restore documents](./../backup-and-restore/introduction.md) for details. Remember to use `--method` parameter.
