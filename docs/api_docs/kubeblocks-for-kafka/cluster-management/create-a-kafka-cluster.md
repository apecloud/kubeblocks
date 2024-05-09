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
  kubectl get clusterdefinition mysql
  >
  NAME    TOPOLOGIES   SERVICEREFS   STATUS      AGE
  mysql                              Available   27m
  ```

  View all available versions for creating a cluster.

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=mysql
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

    ```bash
    # create kafka in combined mode 
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mycluster-combined
      namespace: demo
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: kafka-server
        monitor: false
        name: broker
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
      terminationPolicy: Delete
    EOF
    ```

* Create a Kafka cluster in separated mode.

    ```bash
    # Create kafka cluster in separated mode
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mycluster-separated
      namespace: demo
    spec:
      affinity:
        podAntiAffinity: Preferred
        tenancy: SharedNode
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionRef: kafka
      clusterVersionRef: kafka-3.3.2
      componentSpecs:
      - componentDefRef: controller
        monitor: false
        name: controller
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
        tls: false
      - componentDefRef: kafka-broker
        monitor: false
        name: broker
        noCreatePDB: false
        replicas: 1
        resources:
          limits:
            cpu: "0.5"
            memory: 0.5Gi
          requests:
            cpu: "0.5"
            memory: 0.5Gi
        serviceAccountName: kb-kafka-sa
        tls: false
      terminationPolicy: Delete
    EOF
    ```
