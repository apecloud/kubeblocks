---
title: Create Pulsar Cluster on KubeBlocks
description: How to Create Pulsar Cluster on KubeBlocks
keywords: [pulsar, create cluster]
sidebar_position: 1
sidebar_label: Create
---

## Introduction

KubeBlocks can quickly integrate new engines through good abstraction. The functions tested in KubeBlocks include Pulsar cluster creation and deletion, vertical and horizontal scaling of Pulsar cluster components, storage expansion, restart, and configuration changes.

KubeBlocks supports Pulsar's daily operations, including basic lifecycle operations such as cluster creation, deletion, and restart, as well as advanced operations such as horizontal and vertical scaling, storage expansion, configuration changes, and monitoring.

## Environment Recommendation

Refer to the Pulsar official document for the configuration, such as memory, cup, and storage, of each component.

|      Components        |                                 Replicas                                  |
| :--------------------  | :------------------------------------------------------------------------ |
|       zookeeper        |          1 for test environment or 3 for production environment           |
|        bookies         |  at lease 3 for test environment, at lease 4 for production environment   |
|        broker          |      at least 1, for production environment, 3 replicas recommended       |
| recovery (Optional)    | 1; if autoRecovery is not enabled for bookie, at least 3 replicas needed  |
|   proxy (Optional)     |           1; and for production environment, 3 replicas needed            |

## Create Pulsar cluster

1. Create the Pulsar cluster template file `values-production.yaml` for `helm` locally.
  
   Copy the following information to the local file `values-production.yamlCre`.

   ```bash
   ## Bookies configuration
   bookies:
     resources:
       limits:
         memory: 8Gi
       requests:
         cpu: 2
         memory: 8Gi

     persistence:
       data:
         storageClassName: kb-default-sc
         size: 128Gi
       log:
         storageClassName: kb-default-sc
         size: 64Gi

   ## Zookeeper configuration
   zookeeper:
     resources:
       limits:
         memory: 2Gi
       requests:
         cpu: 1
         memory: 2Gi

     persistence:
       data:
         storageClassName: kb-default-sc
         size: 20Gi
       log:
         storageClassName: kb-default-sc 
         size: 20Gi
        
   broker:
     replicaCount: 3
     resources:
       limits:
         memory: 8Gi
       requests:
         cpu: 2
         memory: 8Gi
   ```

2. Create cluster.

   - **Option 1**: (**Recommended**) Create pulsar cluster by `values-production.yaml` and enable monitor.
    Configuration:
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install pulsar kubeblocks/pulsar-cluster --version "0.6.0-beta.11" -f values-production.yaml --set monitor.enabled=true
     ```

   - **Option 2**: Create pulsar cluster with proxy.
     Configuration:
     - proxy: 3 replicas
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install pulsar kubeblocks/pulsar-cluster --version "0.6.0-beta.11" -f values-production.yaml --set proxy.enable=true  --set monitor.enabled=true  
     ```

   - **Option 3**:  Create pulsar cluster with proxy and deploy `bookies-recovery` component.  
   Configuration:
     - proxy: 3 replicas
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas
     - bookies-recovery: 3 replicas

     ```bash
     helm install pulsar kubeblocks/pulsar-cluster --version "0.6.0-beta.11" -f values-production.yaml --set proxy.enable=true --set bookiesRecovery.enable=true --set monitor.enabled=true 
     ```

   - **Option 4**: Create pulsar cluster and specify bookies and zookeeper storage parameters.
   Configuration:
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install pulsar kubeblocks/pulsar-cluster --version "0.6.0-beta.11" -f values-production.yaml --set bookies.persistence.data.storageClassName=<sc name>,bookies.persistence.log.storageClassName=<sc name>,zookeeper.persistence.data.storageClassName=<sc name>,zookeeper.persistence.log.storageClassName=<sc name> --set monitor.enabled=true
     ```

   You can specify the storage name `<sc name>`.

3. Verify the cluster created.

    ```bash
    kubectl get cluster pulsar
    ```

    When the status is Running, the cluster is created successfully.
