---
title: Create a cluster for Kafka
description: Guide for cluster creation for kafka
keywords: [kafka, cluster, management]
sidebar_position: 1
sidebar_label: Create
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Create a Kafka cluster

This document shows how to create a Kafka cluster.

## Before you start

* [Install kbcli](./../../installation/install-kbcli.md) if you want to create a Kafka cluster by `kbcli`.
* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* Make sure Kafka Addon is enabled with `kbcli addon list`. If this Addon is not enabled, [enable it](./../../installation/install-addons.md) first.

  <Tabs>

  <TabItem value="kubectl" label="kubectl" default>

  ```bash
  kubectl get addons.extensions.kubeblocks.io kafka
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  kafka   Helm                        Enabled   13m
  ```

  </TabItem>

  <TabItem value="kbcli" label="kbcli">

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  kafka                          Helm   Enabled                   true
  ...
  ```

  </TabItem>

  </Tabs>

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

:::note

* KubeBlocks integrates Kafka v3.3.2, running it in KRaft mode.
* You are not recommended to use kraft cluster in combined mode in a production environment.
* The controller number suggested ranges from 3 to 5, out of complexity and availability.

:::

## Create a Kafka cluster

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

1. Create a Kafka cluster. If you only have one node for deploying a cluster with multiple replicas, configure the cluster affinity by setting `spec.schedulingPolicy` or `spec.componentSpecs.schedulingPolicy`. For details, you can refer to the [API docs](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster#apps.kubeblocks.io/v1.SchedulingPolicy). But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   For more cluster examples, refer to [the GitHub repository](https://github.com/apecloud/kubeblocks-addons/tree/main/examples/kafka).

   * Create a Kafka cluster in combined mode.

     ```yaml
     # Create a kafka cluster in combined mode
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1
     kind: Cluster
     metadata:
       name: mycluster
       namespace: demo
     spec:
       terminationPolicy: Delete
       clusterDef: kafka
       topology: combined_monitor
       componentSpecs:
         - name: kafka-combine
           env:
             - name: KB_KAFKA_BROKER_HEAP # use this ENV to set BROKER HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_KAFKA_CONTROLLER_HEAP # use this ENV to set CONTROLLER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_BROKER_DIRECT_POD_ACCESS # set to FALSE for node-port
               value: "true"
           replicas: 1
           resources:
             limits:
               cpu: "1"
               memory: "1Gi"
             requests:
               cpu: "0.5"
               memory: "0.5Gi"
           volumeClaimTemplates:
             - name: data
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 20Gi
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-exporter 
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "1Gi"
             requests:
               cpu: "0.1"
               memory: "0.2Gi"
     EOF
     ```

   * Create a Kafka cluster in separated mode.

     ```yaml
     # Create a Kafka cluster in separated mode
     kubectl apply -f - <<EOF
     apiVersion: apps.kubeblocks.io/v1
     kind: Cluster
     metadata:
       name: mycluster
       namespace: demo
     spec:
       terminationPolicy: Delete
       clusterDef: kafka
       topology: separated_monitor
       componentSpecs:
         - name: kafka-broker
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "0.5Gi"
             requests:
               cpu: "0.5"
               memory: "0.5Gi"
           env:
             - name: KB_KAFKA_BROKER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_KAFKA_CONTROLLER_HEAP
               value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
             - name: KB_BROKER_DIRECT_POD_ACCESS
               value: "true"
           volumeClaimTemplates:
             - name: data
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 20Gi
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-controller
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "0.5Gi"
             requests:
               cpu: "0.5"
               memory: "0.5Gi"
           volumeClaimTemplates:
             - name: metadata
               spec:
                 storageClassName: ""
                 accessModes:
                   - ReadWriteOnce
                 resources:
                   requests:
                     storage: 1Gi
         - name: kafka-exporter
           replicas: 1
           resources:
             limits:
               cpu: "0.5"
               memory: "1Gi"
             requests:
               cpu: "0.1"
               memory: "0.2Gi"
     EOF
     ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `spec.terminationPolicy`              | It is the policy of cluster termination. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-kafka-cluster.md#termination-policy). |
   | `spec.clusterDef` | It specifies the name of the ClusterDefinition to use when creating a Cluster. **Note: DO NOT UPDATE THIS FIELD**. The value must be must be `kafaka` to create a Kafka Cluster. |
   | `spec.topology` | It specifies the name of the ClusterTopology to be used when creating the Cluster. Valid options are: [combined,combined_monitor,separated,separated_monitor]. |
   | `spec.componentSpecs`                 | It is the list of ClusterComponentSpec objects that define the individual Components that make up a Cluster. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.replicas`        | It specifies the amount of replicas of the component. |
   | `spec.componentSpecs.resources`       | It specifies the resources required by the Component.  |
   | `spec.componentSpecs.volumeClaimTemplates` | It specifies a list of PersistentVolumeClaim templates that define the storage requirements for the Component. |
   | `spec.componentSpecs.volumeClaimTemplates.name` | It refers to the name of a volumeMount defined in `componentDefinition.spec.runtime.containers[*].volumeMounts`. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.storageClassName` | It is the name of the StorageClass required by the claim. If not specified, the StorageClass annotated with `storageclass.kubernetes.io/is-default-class=true` will be used by default. |
   | `spec.componentSpecs.volumeClaimTemplates.spec.resources.storage` | You can set the storage size as needed. |

   For more API fields and descriptions, refer to the [API Reference](https://kubeblocks.io/docs/preview/developer_docs/api-reference/cluster).

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
   mycluster   kafka                kafka-3.3.2   Delete               Running   2m2s
   ```

</TabItem>

<TabItem value="kbcli" label="kbcli">

1. Create a Kafka cluster.

   The cluster creation command is simply `kbcli cluster create`. Further, you can customize your cluster resources as demanded by using the `--set` flag.

   ```bash
   kbcli cluster create kafka mycluster -n demo
   ```

   kbcli provides more options for creating a Kafka cluster, such as setting cluster version, termination policy, CPU, and memory. You can view these options by adding `--help` or `-h` flag.

   ```bash
   kbcli cluster create kafka --help

   kbcli cluster create kafka -h
   ```

   If you only have one node for deploying a cluster with multiple replicas, you can configure the cluster affinity by setting `--pod-anti-afffinity`, `--tolerations`, and `--topology-keys` when creating a cluster. But you should note that for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
   ```

</TabItem>

</Tabs>
