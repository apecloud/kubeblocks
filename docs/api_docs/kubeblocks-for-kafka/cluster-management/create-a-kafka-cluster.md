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

* [Install KubeBlocks](./../../installation/install-kubeblocks.md).
* View all the database types and versions available for creating a cluster.
  
  Make sure the `kafka` cluster definition is installed. If the cluster definition is not available, refer to [this doc](./../../overview/supported-addons.md#install-addons) to enable it first.

  ```bash
  kubectl get clusterdefinition kafka
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  kafka                              Available   27m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=kafka
  ```

* To keep things isolated, create a separate namespace called `demo` throughout this tutorial.

  ```bash
  kubectl create namespace demo
  ```

:::note

* KubeBlocks integrates Kafka v3.3.2, running it in KRaft mode.
* You are not recommended to use kraft cluster in combined mode in production environment.
* The controller number suggested ranges from 3 to 5, out of complexity and availability.

:::

## Create a Kafka cluster

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
        tenancy: SharedNode
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
        tenancy: SharedNode
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

:::note

If you only have one node for deploying a cluster with multiple replicas, set `spec.affinity.topologyKeys` as `null`.

:::

| Field                                 | Definition  |
|---------------------------------------|--------------------------------------|
| `metadata.annotations."kubeblocks.io/extra-env"` | It defines Kafka broker's jvm heap setting. |
| `metadata.annotations.kubeblocks.io/enabled-pod-ordinal-svc` | It defines kafka cluster annotation keys for nodeport feature gate. You can also set`kubeblocks.io/enabled-node-port-svc: broker` and `kubeblocks.io/disabled-cluster-ip-svc: broker`. |
| `spec.clusterDefinitionRef`           | It specifies the name of the ClusterDefinition for creating a specific type of cluster.  |
| `spec.clusterVersionRef`              | It is the name of the cluster version CRD that defines the cluster version.  |
| `spec.terminationPolicy`              | It is the policy of cluster termination. The default value is `Delete`. Valid values are `DoNotTerminate`, `Halt`, `Delete`, `WipeOut`.  <p> - `DoNotTerminate` blocks deletion operation. </p><p> - `Halt` deletes workload resources such as statefulset and deployment workloads but keep PVCs. </p><p> - `Delete` is based on Halt and deletes PVCs. </p><p> - `WipeOut` is based on Delete and wipe out all volume snapshots and snapshot data from a backup storage location. </p> |
| `spec.affinity`                       | It defines a set of node affinity scheduling rules for the cluster's Pods. This field helps control the placement of Pods on nodes within the cluster.  |
| `spec.affinity.podAntiAffinity`       | It specifies the anti-affinity level of Pods within a component. It determines how pods should spread across nodes to improve availability and performance. |
| `spec.affinity.topologyKeys`          | It represents the key of node labels used to define the topology domain for Pod anti-affinity and Pod spread constraints.   |
| `spec.affinity.tenacy`                | It determines the level of resource isolation between Pods. It can have the following values: `SharedNode` and `DedicatedNode`. <p> - SharedNode: It allows that multiple Pods may share the same node, which is the default behavior of K8s. </p> <p> - DedicatedNode: Each Pod runs on a dedicated node, ensuring that no two Pods share the same node.</p>|
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.services` | It defines the services to access a cluster. |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition kafka -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |