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

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md) if you want to create a Kafka cluster by `kbcli`.
* Install KubeBlocks [by kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../../installation/install-with-helm/install-kubeblocks.md).
* Make sure Kafka Addon is enabled with `kbcli addon list`. If this Addon is not enabled, enable it first. Both [kbccli](./../../installation/install-with-kbcli/install-addons.md) and [Helm](./../../installation/install-with-helm/install-addons.md) options are available.

  <Tabs>

  <TabItem value="kbcli" label="kbcli" default>

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL  
  ...
  kafka                          Helm   Enabled                   true
  ...
  ```

  </TabItem>

  <TabItem value="kubectl" label="kubectl">

  ```bash
  kubectl get addons.extensions.kubeblocks.io kafka
  >
  NAME    TYPE   VERSION   PROVIDER   STATUS    AGE
  kafka   Helm                        Enabled   13m
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

<TabItem value="kbcli" label="kbcli" default>

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

2. Verify whether this cluster is created successfully.

   ```bash
   kbcli cluster list -n demo
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   demo        kafka                kafka-3.3.2   Delete               Running   Sep 27,2024 15:15 UTC+0800
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Create a Kafka cluster. If you only have one node for deploying a cluster with multiple replicas, set `spec.affinity.topologyKeys` as `null`. But for a production environment, it is not recommended to deploy all replicas on one node, which may decrease the cluster availability.

   * Create a Kafka cluster in combined mode.

       ```yaml
       # create kafka in combined mode 
       kubectl apply -f - <<EOF
       apiVersion: apps.kubeblocks.io/v1alpha1
       kind: Cluster
       metadata:
         name: mycluster
         namespace: demo
         annotations:
           "kubeblocks.io/extra-env": '{"KB_KAFKA_ENABLE_SASL":"false","KB_KAFKA_BROKER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_CONTROLLER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_PUBLIC_ACCESS":"false", "KB_KAFKA_BROKER_NODEPORT": "false"}'
           kubeblocks.io/enabled-pod-ordinal-svc: broker
       spec:
         clusterDefinitionRef: kafka
         clusterVersionRef: kafka-3.3.2
         terminationPolicy: Delete
         affinity:
           podAntiAffinity: Preferred
           topologyKeys:
           - kubernetes.io/hostname
         tolerations:
           - key: kb-data
             operator: Equal
             value: "true"
             effect: NoSchedule
         services:
         - name: bootstrap
           serviceName: bootstrap
           componentSelector: broker
           spec:
             type: ClusterIP
             ports:
             - name: kafka-client
               targetPort: 9092
               port: 9092
         componentSpecs:
         - name: broker
           componentDef: kafka-combine
           tls: false
           replicas: 1
           serviceAccountName: kb-kafka-cluster
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
           - name: metadata
             spec:
               accessModes:
               - ReadWriteOnce
               resources:
                 requests:
                   storage: 20Gi
         - name: metrics-exp
           componentDefRef: kafka-exporter
           componentDef: kafka-exporter
           replicas: 1
           resources:
             limits:
               cpu: '0.5'
               memory: 0.5Gi
             requests:
               cpu: '0.5'
               memory: 0.5Gi
       EOF
       ```

   * Create a Kafka cluster in separated mode.

       ```yaml
       # Create kafka cluster in separated mode
       kubectl apply -f - <<EOF
       apiVersion: apps.kubeblocks.io/v1alpha1
       kind: Cluster
       metadata:
         name: kafka-cluster
         namespace: demo
         annotations:
           "kubeblocks.io/extra-env": '{"KB_KAFKA_ENABLE_SASL":"false","KB_KAFKA_BROKER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_CONTROLLER_HEAP":"-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64","KB_KAFKA_PUBLIC_ACCESS":"false", "KB_KAFKA_BROKER_NODEPORT": "false"}'
           kubeblocks.io/enabled-pod-ordinal-svc: broker
       spec:
         clusterDefinitionRef: kafka
         clusterVersionRef: kafka-3.3.2
         terminationPolicy: Delete
         affinity:
           podAntiAffinity: Preferred
           topologyKeys:
           - kubernetes.io/hostname
           tolerations:
             - key: kb-data
               operator: Equal
               value: "true"
               effect: NoSchedule
           services:
             - name: bootstrap
               serviceName: bootstrap
               componentSelector: broker
           spec:
               type: ClusterIP
               ports:
               - name: kafka-client
                 targetPort: 9092
                 port: 9092
       componentSpecs:
       - name: broker
         componentDef: kafka-broker
         tls: false
         replicas: 1
         serviceAccountName: kb-kafka-cluster
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
         - name: metadata
           spec:
             storageClassName: null
             accessModes:
             - ReadWriteOnce
             resources:
               requests:
                 storage: 5Gi
       - name: controller
         componentDefRef: controller
         componentDef: kafka-controller
         tls: false
         replicas: 1
         serviceAccountName: kb-kafka-cluster
         resources:
           limits:
             cpu: '0.5'
             memory: 0.5Gi
           requests:
             cpu: '0.5'
             memory: 0.5Gi
         volumeClaimTemplates:
         - name: metadata
           spec:
             storageClassName: null
             accessModes:
             - ReadWriteOnce
             resources:
               requests:
                 storage: 20Gi
       - name: metrics-exp
         componentDefRef: kafka-exporter
         componentDef: kafka-exporter
         replicas: 1
         resources:
           limits:
             cpu: '0.5'
             memory: 0.5Gi
           requests:
             cpu: '0.5'
             memory: 0.5Gi
       EOF
       ```

   | Field                                 | Definition  |
   |---------------------------------------|--------------------------------------|
   | `metadata.annotations."kubeblocks.io/extra-env"` | It defines Kafka broker's jvm heap setting. |
   | `metadata.annotations.kubeblocks.io/enabled-pod-ordinal-svc` | It defines kafka cluster annotation keys for nodeport feature gate. You can also set`kubeblocks.io/enabled-node-port-svc: broker` and `kubeblocks.io/disabled-cluster-ip-svc: broker`. |
   | `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
   | `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
   | `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Delete`, `WipeOut`. For the detailed definition, you can refer to [Termination Policy](./delete-kafka-cluster.md#termination-policy). |
   | `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
   | `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
   | `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
   | `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
   | `spec.services` | It defines the services to access a cluster. |
   | `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
   | `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition kafka -o json \| jq '.spec.componentDefs[].name'`.   |
   | `spec.componentSpecs.name`            | It specifies the name of the component.     |
   | `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
   | `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

2. Verify whether this cluster is created successfully.

   ```bash
   kubectl get cluster mycluster -n demo
   >
   NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
   mycluster   kafka                kafka-3.3.2   Delete               Running   2m2s
   ```

</TabItem>

</Tabs>
