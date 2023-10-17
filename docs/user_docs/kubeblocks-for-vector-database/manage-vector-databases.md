---
title: Manage Vector Databases with KubeBlocks
description: How to manage vector databases on KubeBlocks
keywords: [qdrant, milvus,weaviate]
sidebar_position: 1
sidebar_label: Manage Vector Databases with KubeBlocks
---
# Manage Vector Databases with KubeBlocks

The popularity of generative AI (Generative AI) has aroused widespread attention and completely ignited the vector database (Vector Database) market. KubeBlocks supports the management of vector databases, such as Qdrant, Milvus, and Weaviate.
In this chapter, we take Qdrant as an example to show how to manage vector databases with KubeBlocks.
Before you start, [Install KubeBlocks](./../installation/install-with-helm/) and `kbcli`.

## Create a cluster

***Steps***

1. Execute the following command to create a Qdrant cluster. You can change the `cluster-definition` value as any other databases supported.

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
   qdrant   default     qdrant               qdrant-1.1.0   Delete               Running   Aug 15,2023 23:03 UTC+0800
   ```

3. Check the cluster information.

   ```bash
   kbcli cluster describe qdrant
   >
   Name: qdrant         Created Time: Aug 15,2023 23:03 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION        STATUS    TERMINATION-POLICY
   default     qdrant               qdrant-1.1.0   Running   Delete

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

## Connect to a vector database cluster

Qdrant provides both HTTP and gRPC protocols for client access on ports 6333 and 6334 respectively. Depending on where the client is, different connection options are offered to connect to the Qdrant cluster.

:::note

If your cluster is on AWS, install the AWS Load Balancer Controller first.

:::

- If your client is inside a K8s cluster, run `kbcli cluster describe qdrant` to get the ClusterIP address of the cluster or the corresponding K8s cluster domain name.
- If your client is outside the K8s cluster but in the same VPC as the server, run `kbcli cluster expose qdant --enable=true --type=vpc` to get a VPC load balancer address for the database cluster.
- If your client is outside the VPC, run `kbcli cluster expose qdant --enable=true --type=internet` to open a public network reachable address for the database cluster.

## Monitor the vector database

Open the grafana monitor web page.

```bash
kbcli dashboard open kubeblocks-grafana
```

When executing this command, browser is opened and you can see the dashboard.

## Scaling

Scaling function for vector databases is also supported.

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

## Volume Expanding

***Steps:***

```bash
kbcli cluster volume-expand qdrant --storage=40Gi --components=qdrant -t data
```

The volume expanding may take a few minutes.

The `kbcli cluster volume-expand` command print the `opsname`, to check the progress of volume expanding, you can use the following command with the `opsname`.

```bash
kubectl get ops qdrant-volumeexpansion-5pbd2
>
NAME                           TYPE              CLUSTER   STATUS   PROGRESS   AGE
qdrant-volumeexpansion-5pbd2   VolumeExpansion   qdrant    Running  -/-        67s
```

To check whether the expanding is done, use the following command.

```bash
kbcli cluster describe qdrant
```
