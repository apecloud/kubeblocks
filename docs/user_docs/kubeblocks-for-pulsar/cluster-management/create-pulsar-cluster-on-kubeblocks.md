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

1. Create a Pulsar cluster.

   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
     annotations:
       "kubeblocks.io/extra-env": '{"KB_PULSAR_BROKER_NODEPORT": "false"}'
   spec:
     terminationPolicy: Delete
     services:
     - name: proxy
       serviceName: proxy
       componentSelector: pulsar-proxy
       spec:
         type: ClusterIP
         ports:
         - name: pulsar
           port: 6650
           targetPort: 6650
         - name: http
           port: 80
           targetPort: 8080
     - name: broker-bootstrap
       serviceName: broker-bootstrap
       componentSelector: pulsar-broker
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
     componentSpecs:
     - name: pulsar-broker
       componentDef: pulsar-broker
       disableExporter: true
       serviceAccountName: kb-pulsar-cluster
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
     - name: pulsar-proxy
       componentDef: pulsar-proxy
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
     - name: bookies
       componentDef: pulsar-bookkeeper
       replicas: 3
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
       volumeClaimTemplates:
       - name: journal
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
       - name: ledgers
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
     - name: bookies-recovery
       componentDef: pulsar-bkrecovery
       replicas: 1
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
     - name: zookeeper
       componentDef: pulsar-zookeeper
       replicas: 3
       resources:
         limits:
           cpu: '0.5'
           memory: 0.5Gi
         requests:
           cpu: '0.5'
           memory: 0.5Gi
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
   EOF
   ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `metadata.annotations."kubeblocks.io/extra-env"` | It specifies whether to enable NodePort services. |
   | `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-a-postgresql-cluster.md#termination-policy). |
   | `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
   | `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
   | `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
   | `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
   | `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition postgresql -o json \| jq '.spec.componentDefs[].name'`.   |
   | `spec.componentSpecs.name`            | It specifies the name of the component.     |
   | `spec.componentSpecs.disableExporter` | It defines whether the monitoring function is enabled. |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
   | `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

2. Verify the cluster created.

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    When the status is Running, the cluster is created successfully.
