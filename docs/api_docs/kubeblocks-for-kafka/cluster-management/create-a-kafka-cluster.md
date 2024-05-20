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

* Install KubeBlocks: You can install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* View all the database types and versions available for creating a cluster.
  
  Make sure the `mysql` cluster definition is installed with `kubectl get clusterdefinition mysql`.

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

If you only have one node for deploying a RaftGroup Cluster, set `spec.affinity.topologyKeys` as `null`.

:::