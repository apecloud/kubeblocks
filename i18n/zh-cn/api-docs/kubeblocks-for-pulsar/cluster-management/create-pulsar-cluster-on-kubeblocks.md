---
title: 创建 Pulsar 集群
description: 如何创建 Pulsar 集群
keywords: [pulsar, 创建集群]
sidebar_position: 1
sidebar_label: 创建
---

# 创建 Pulsar 集群

## 概述

KubeBlocks 可以通过良好的抽象快速集成新引擎，并支持 Pulsar 集群的创建和删除、集群组件的垂直扩缩容和水平扩缩容、存储扩容、重启和配置更改等。

本系列文档重点展示 KubeBlocks 对 Pulsar 日常运维能力的支持，包括集群创建、删除、重启等基本生命周期操作，以及水平扩容、垂直扩容、存储扩容、配置变更、监控等高阶操作。

## 环境推荐

关于各组件的规格（如内存、CPU 和存储容量等），请参考 [Pulsar 官方文档](https://pulsar.apache.org/docs/3.1.x/)。

|      组件               |                                 所需副本数                                  |
| :--------------------  | :------------------------------------------------------------------------ |
|       zookeeper        |   测试环境 1 个，生产环境 3 个           |
|        bookies         |  测试环境至少 3 个，生产环境至少 4 个   |
|        broker          |      至少 1 个，生产环境建议 3 个       |
| recovery (可选)         | 至少 1 个；如果 bookie 未启用 autoRecovery 功能，则至少需要 3 个 |
|   proxy (可选)          |         至少 1 个；生产环境需要 3 个           |

## 开始之前

* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。

* 查看可用于创建集群的数据库类型和版本。

  确保 `pulsar` cluster definition 已安装。如果该 cluster definition 不可用，可参考[相关文档](./../../installation/install-addons.md)启用。

  ```bash
  kubectl get clusterdefinition redis
  >
  NAME    TOPOLOGIES   SERVICEREFS    STATUS      AGE
  pulsar                              Available   16m
  ```

  查看可用于创建集群的引擎版本。

  ```bash
  kubectl get clusterversions -l clusterdefinition.kubeblocks.io/name=redis
  >
  NAME           CLUSTER-DEFINITION   STATUS      AGE
  pulsar-3.0.2   pulsar               Available   16m
  ```

* 为保证资源隔离，本教程将创建一个名为 `demo` 的独立命名空间。

  ```bash
  kubectl create namespace demo
  >
  namespace/demo created
  ```

## 创建集群

1. 在本地创建 `helm` 使用的 Pulsar 集群模板文件 `values-production.yaml`。
  
   将以下信息复制到本地文件 `values-production.yaml` 中。

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

2. 创建集群。

   - **选项 1.**（**推荐**）使用 `values-production.yaml` 创建 Pulsar 集群并启用监控。
   配置:
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install mycluster kubeblocks/pulsar-cluster --version "x.x.x" -f values-production.yaml --set disable.exporter=false --namespace=demo
     ```

   - **选项 2.** 创建带 proxy 的 Pulsar 集群。
   配置:
     - proxy: 3 replicas
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install mycluster kubeblocks/pulsar-cluster --version "x.x.x" -f values-production.yaml --set proxy.enable=true  --set disable.exporter=false --namespace=demo
     ```

   - **选项 3.** 创建带 proxy 的 Pulsar 集群，并部署独立的 `bookies-recovery` 组件。
   配置:
     - proxy: 3 replicas
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas
     - bookies-recovery: 3 replicas

     ```bash
     helm install mycluster kubeblocks/pulsar-cluster --version "x.x.x" -f values-production.yaml --set proxy.enable=true --set bookiesRecovery.enable=true --set disable.exporter=false --namespace=demo
     ```

   - **选项 4.** 创建 Pulsar 集群并指定 bookies 和 zookeeper 的存储参数。
   配置:
     - broker: 3 replicas
     - bookies: 4 replicas
     - zookeeper: 3 replicas

     ```bash
     helm install mycluster kubeblocks/pulsar-cluster --namespace=demo --version "x.x.x" -f values-production.yaml --set bookies.persistence.data.storageClassName=<sc name>,bookies.persistence.log.storageClassName=<sc name>,zookeeper.persistence.data.storageClassName=<sc name>,zookeeper.persistence.log.storageClassName=<sc name> --set disable.exporter=false
     ```

   您可以指定存储名称 `<sc name>`。

3. 验证已创建的集群。

    ```bash
    kubectl get cluster mycluster --namespace=demo
    ```

    当状态显示为 `Running` 时，表示集群已成功创建。
