---
title: Manage Weaviate with KubeBlocks
description: How to manage vector databases on KubeBlocks
keywords: [weaviate, vector database, control plane]
sidebar_position: 1
sidebar_label: Manage Qdrant with KubeBlocks
---
# Manage Weaviate with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. Weaviate is an open-source vector database that simplifies the development of AI applications. Built-in vector and hybrid search, easy-to-connect machine learning models, and a focus on data privacy enable developers of all levels to build, iterate, and scale AI capabilities faster.


KubeBlocks supports the management of Weaviate.

Before you start, [install kbcli](./../installation/install-with-kbcli/install-kbcli.md) and [install KubeBlocks](./../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).

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

      Name: weaviate	 Created Time: Jul 05,2024 17:42 UTC+0800
      NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
      default     weaviate             weaviate-1.18.0   Running   Delete

      Endpoints:
      COMPONENT   MODE        INTERNAL                                      EXTERNAL
      weaviate    ReadWrite   myw-weaviate.default.svc.cluster.local:8080   <none>

      Topology:
      COMPONENT   INSTANCE         ROLE     STATUS    AZ       NODE                    CREATED-TIME
      weaviate    myw-weaviate-0   <none>   Running   <none>   minikube/192.168.49.2   Jul 05,2024 17:42 UTC+0800

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

Weaviate provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Weaviate cluster.

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe weaviate` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose weaviate --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose weaviate --enable=true --type=internet` to open a public network reachable address for the database cluster.

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
   kubectl get cluster mycluster -o yaml
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
   kbcli cluster update mycluster --disableExporter=false
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
kbcli cluster hscale weaviate --replicas=5 --components=weaviate
```

Please wait a few seconds until the scaling process is over.

The `kbcli cluster hscale` command print the `opsname`, to check the progress of horizontal scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops weaviate-horizontalscaling-xpdwz
>
NAME                             TYPE                CLUSTER   STATUS    PROGRESS   AGE
weaviate-horizontalscaling-xpdwz   HorizontalScaling   weaviate    Running   0/2        16s
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
The `kbcli cluster vscale` command print the `opsname`, to check the progress of scaling, you can use the following command with the `opsname`.

```bash
kubectl get ops weaviate-verticalscaling-rpw2l
>
NAME                           TYPE              CLUSTER   STATUS    PROGRESS   AGE
weaviate-verticalscaling-rpw2l   VerticalScaling   weaviate    Running   1/5        44s
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

The `kbcli cluster volume-expand` command print the `opsname`, to check the progress of volume expanding, you can use the following command with the `opsname`.

```bash
kubectl get ops weaviate-volumeexpansion-5pbd2
>
NAME                           TYPE              CLUSTER   STATUS   PROGRESS   AGE
weaviate-volumeexpansion-5pbd2   VolumeExpansion   weaviate    Running  1/1        67s
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe weaviate
```

## Backup and restore

The backup and restore operations for Weaviate are the same as those of other clusters and you can refer to [the backup and restore documents](./../backup-and-restore/introduction.md) for details. Remember to use `--method` parameter.
