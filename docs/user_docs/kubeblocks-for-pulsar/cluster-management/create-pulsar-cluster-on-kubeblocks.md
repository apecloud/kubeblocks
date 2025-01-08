---
title: Create a Pulsar Cluster
description: How to Create Pulsar Cluster on KubeBlocks
keywords: [pulsar, create cluster]
sidebar_position: 1
sidebar_label: Create
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Introduction

KubeBlocks can quickly integrate new engines through good abstraction. The functions tested in KubeBlocks include Pulsar cluster creation and deletion, vertical and horizontal scaling of Pulsar cluster components, storage expansion, restart, and configuration changes.

KubeBlocks supports Pulsar's daily operations, including basic lifecycle operations such as cluster creation, deletion, and restart, as well as advanced operations such as horizontal and vertical scaling, storage expansion, and configuration changes.

## Environment Recommendation

Refer to the [Pulsar official document](https://pulsar.apache.org/docs/3.1.x/) for the configuration, such as memory, cpu, and storage, of each component.

|      Components        |                                 Replicas                                  |
| :--------------------  | :------------------------------------------------------------------------ |
|       zookeeper        |          1 for test environment or 3 for production environment           |
|        bookies         |  at least 3 for test environment, at lease 4 for production environment   |
|        broker          |      at least 1, for production environment, 3 replicas recommended       |
| recovery (Optional)    | 1; if autoRecovery is not enabled for bookie, at least 3 replicas needed  |
|   proxy (Optional)     |           1; and for production environment, 3 replicas needed            |

## Before you start

* [Install kbcli](./../../installation/install-kbcli.md) if you want to manage the StarRocks cluster with `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Check whether the Pulsar Addon is enabled. If this Addon is disabled, [enable it](./../../installation/install-addons.md) first.
* View all the database types and versions available for creating a cluster.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get clusterdefinition pulsar
  >
  NAME    TOPOLOGIES                                        SERVICEREFS    STATUS      AGE
  pulsar  pulsar-basic-cluster,pulsar-enhanced-cluster                     Available   16m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=pulsar
  >
  NAME            CLUSTER-DEFINITION   STATUS      AGE
  pulsar-2.11.2   pulsar               Available   16m
  pulsar-3.0.2    pulsar               Available   16m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli clusterdefinition list
  kbcli clusterversion list
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo`.

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

## Create Pulsar cluster

1. Create a Pulsar cluster in basic mode. For other cluster modes, check out the examples provided in [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/pulsar). If you only have one node for deploying a Pulsar Cluster, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     terminationPolicy: Delete
     clusterDef: pulsar
     topology: pulsar-basic-cluster
     services:
       - name: broker-bootstrap
         serviceName: broker-bootstrap
         componentSelector: broker
         spec:
           type: ClusterIP
           ports:
             - name: pulsar
               port: 6650
               targetPort: 6650
             - name: http
               port: 80
               targetPort: 8080
             - name: kafka-client
               port: 9092
               targetPort: 9092
       - name: zookeeper
         serviceName: zookeeper
         componentSelector: zookeeper
         spec:
           type: ClusterIP
           ports:
             - name: client
               port: 2181
               targetPort: 2181
     componentSpecs:
       - name: broker
         serviceVersion: 3.0.2
         replicas: 1
         env:
           - name: KB_PULSAR_BROKER_NODEPORT
             value: "false"
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "200m"
             memory: "512Mi"
       - name: bookies
         serviceVersion: 3.0.2
         replicas: 4
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "200m"
             memory: "512Mi"
         volumeClaimTemplates:
           - name: ledgers
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
           - name: journal
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
       - name: zookeeper
         serviceVersion: 3.0.2
         replicas: 1
         resources:
           limits:
             cpu: "1"
             memory: "512Mi"
           requests:
             cpu: "100m"
             memory: "512Mi"
         volumeClaimTemplates:
           - name: data
             spec:
               accessModes:
                 - ReadWriteOnce
               resources:
                 requests:
                   storage: 8Gi
   EOF
   ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-a-pulsar-cluster.md#termination-policy). |
   | `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be `pulsar` to create a Pulsar Cluster. |
   | `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. |
   | `spec.services` | It defines a list of additional Services that are exposed by a Cluster. |
   | `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.serviceVersion` | It specifies the version of the Service expected to be provisioned by this Component. Valid options are [2.11.2,3.0.2]. |
   | `spec.componentSpecs.disableExporter` | It determines whether metrics exporter information is annotated on the Component's headless Service. Valid options are [true, false]. |
   | `spec.componentSpecs.replicas`        | It specifies the amount of replicas of the component. |
   | `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
   | `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
   | `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

   For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

2. Verify the cluster created.

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    When the status is Running, the cluster is created successfully.
