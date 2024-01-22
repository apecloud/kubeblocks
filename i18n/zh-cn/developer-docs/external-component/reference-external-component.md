---
title: 引用外部组件
description: KubeBlocks 支持引用外部组件，灵活管理集群。
keywords: [外部组件]
sidebar_position: 1
sidebar_label: 引用外部组件
---

# 引用外部组件

:::note

引用外部组件为 alpha 版本功能，后续可能会有较大的演进和变更。

:::

## 什么是引用外部组件？

许多数据库集群往往依赖于元数据存储进行分布式协调和动态配置。然而，随着数据库集群的不断增加，元数据存储本身可能会消耗大量资源，比如包括 Pulsar 中的 ZooKeeper。为了减少开销，开发者可以在多个数据库集群中引用相同的外部组件。

KubeBlocks 中的引用外部组件指的是，在一个 KubeBlocks 集群中，通过声明式定义的方式，引用一个外部组件或者基于 KubeBlocks 的组件。

根据其定义，引用可以分为两种类型：

* 引用外部组件

  此外部组件可以是基于 Kubernetes 或非 Kubernetes 的。在引用此组件时，首先创建一个 ServiceDescriptor CR，该 CR 定义了引用的服务和资源。

* 引用基于 KubeBlocks 的组件

  这种类型的组件基于 KubeBlocks 集群。在引用此组件时，只需填写被引用的集群对象即可，无需创建ServiceDescriptor 对象。

## 引用外部组件示例

下面以 KubeBlocks Pulsar 集群通过引用外部组件的方式引用 ZooKeeper 为例，展示两部分内容：

1. 在安装 KubeBlocks 或启用引擎时，[创建外部组件引用声明](#创建外部组件引用声明)。
2. 创建集群时，[定义外部组件关联映射](#定义外部组件关联映射)。

一个 KubeBlocks Pulsar 集群由 proxy、broker、bookies 以及 ZooKeeper 等组件构成。其中 broker 与 bookies 组件依赖 ZooKeeper 组件提供元数据存储与交互。

:::note

更多关于 Pulsar 集群的信息，请参考 [KubeBlocks 中的 Pulsar](./../../user-docs/kubeblocks-for-pulsar/cluster-management/create-pulsar-cluster-on-kb.md) 文档。

:::

### 创建外部组件引用声明

1. 在 ClusterDefinition 的 `componentDefs` 中，声明需要引用的组件。

    在本例中，Pulsar 的 broker 与 bookies 组件依赖 ZooKeeper 组件，所以需要在 broker 与 bookies 的 `componentDefs` 定义中加上对 ZooKeeper 的声明。

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: ClusterDefinition
    metadata:
      name: pulsar
      labels:
        {{- include "pulsar.labels" . | nindent 4 }}
    spec:
      type: pulsar
      # 其余定义省略
      componentDefs:
        - name: pulsar-broker
          workloadType: Stateful
          characterType: pulsar-broker
          serviceRefDeclarations:
          - name: pulsarZookeeper
            serviceRefDeclarationSpecs:
              - serviceKind: zookeeper
                serviceVersion: ^3.8.\d{1,2}$
          # 其余定义省略
        - name: bookies
          workloadType: Stateful
          characterType: bookkeeper
          statefulSpec:
            updateStrategy: BestEffortParallel
          serviceRefDeclarations:
          - name: pulsarZookeeper
            serviceRefDeclarationSpecs:
            - serviceKind: zookeeper
              serviceVersion: ^3.8.\d{1,2}$
        # 其余组件定义省略
    ```

    上述 `serviceRefDeclarations` 模块展示了需要增加的外部组件引用声明，可以看到，`pulsar-broker` 和 `bookies` 组件都声明了一个名为 `pulsarZookeeper` 的服务引用，这表示 `pulsar-broker` 和 `bookies` 都需要一个名称为 `pulsarZookeeper`、服务类型（`serviceKind`）为 zookeeper、服务版本（`serviceVersion`）符合 `^3.8.\d{1,2}$` 正则表达式的服务。

    这个名为 `pulsarZookeeper` 的引用声明，将在用户创建正式的 Pulsar 集群时，映射成为用户指定的具体的 ZooKeeper 集群。

2. 在组件 provider 中定义对 ZooKeeper 组件的使用方式。

    在 ClusterDefinition 进行外部组件引用声明后，用户就可以在 ClusterDefinition 中使用预定义的 `pulsarZookeeper`。

    例如，在启动 pulsar-broker 和 bookies 服务时，需要将 ZooKeeper 服务的地址传入到 pulsar-broker 和 bookies 的配置中，然后就可以在 pulsar-broker 和 bookies 的配置模板渲染中进行引用。下例展示在 `broker-env.tpl` 模板中生成需要用到的 zookeeperServers：

    :::note

    ClusterDefinition 只知道这是一个 ZooKeeper 的服务，但不知道具体是谁提供的哪一个 ZooKeeper。因此，在创建集群时，需要进行 ZooKeeper 组件的映射，请参考下一节的说明。

    :::

    ```yaml
    {{- $clusterName := $.cluster.metadata.name }}
    {{- $namespace := $.cluster.metadata.namespace }}
    {{- $pulsar_zk_from_service_ref := fromJson "{}" }}
    {{- $pulsar_zk_from_component := fromJson "{}" }}

    {{- if index $.component "serviceReferences" }}
      {{- range $i, $e := $.component.serviceReferences }}
        {{- if eq $i "pulsarZookeeper" }}
          {{- $pulsar_zk_from_service_ref = $e }}
          {{- break }}
        {{- end }}
      {{- end }}
    {{- end }}
    {{- range $i, $e := $.cluster.spec.componentSpecs }}
      {{- if eq $e.componentDefRef "zookeeper" }}
        {{- $pulsar_zk_from_component = $e }}
      {{- end }}
    {{- end }}

    # 首先尝试从服务引用中获取 ZooKeeper。如果 ZooKeeper 服务引用为空，则在 ClusterDefinition 中获取默认的 zookeepercomponentDef
    {{- $zk_server := "" }}
    {{- if $pulsar_zk_from_service_ref }}
      {{- if and (index $pulsar_zk_from_service_ref.spec "endpoint") (index $pulsar_zk_from_service_ref.spec "port") }}
        {{- $zk_server = printf "%s:%s" $pulsar_zk_from_service_ref.spec.endpoint.value $pulsar_zk_from_service_ref.spec.port.value }}
      {{- else }}
        {{- $zk_server = printf "%s-%s.%s.svc:2181" $clusterName $pulsar_zk_from_component.name $namespace }}
      {{- end }}
    {{- else }}
      {{- $zk_server = printf "%s-%s.%s.svc:2181" $clusterName $pulsar_zk_from_component.name $namespace }}
    {{- end }}
    zookeeperServers: {{ $zk_server }}
    configurationStoreServers: {{ $zk_server }}
    ```

    :::note

    目前仅支持在配置模板渲染中对声明的服务进行 endpoint 和 port 的引用，其他更多引用方式与引用规范将在后续的版本中支持（例如直接在 `env` 中引用声明服务的账号密码等信息）。

    :::

### 定义外部组件关联映射

基于上例，在创建 Pulsar 集群时，ZooKeeper 的映射关联可分成两种：

* 关联映射外部 ZooKeeper 服务；
* 关联映射 KubeBlocks 提供的独立集群部署的 ZooKeeper 服务。

#### 关联映射外部 ZooKeeper 服务

1. 在 K8s 集群中创建一个 ServiceDescriptor CR 对象。

   在 KubeBlocks 中，ServiceDescriptor 用于描述和存储引用信息的 API 对象。ServiceDescriptor 允许将一个基于 K8s 或者 非 K8s 的提供的服务抽离出来，将其提供给 KubeBlocks 中其他的 Cluster 对象进行引用。

   ServiceDescriptor 可以用于解决 KubeBlocks 中服务依赖、组件依赖和组件共享等问题。下面展示的是一个 CR 对象的示例。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: ServiceDescriptor
   metadata:
     name: zookeeper-service-descriptor
     namespace: default
   spec:
     serviceKind: zookeeper
     serviceVersion: 3.8.0
     endpoint: pulsar-zookeeper.default.svc // Replace the example value with the actual endpoint of the external zookeeper
     port: 2181
   ```

2. 在创建 Pulsar 集群时引用外部 ZooKeeper 服务。

   如下所示，创建一个引用外部 ZooKeeper 服务的 Pulsar 集群。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: pulsar
     namespace: default
   spec:
     clusterDefinitionRef: pulsar
     clusterVersionRef: pulsar-2.11.2
     componentSpecs:
     - componentDefRef: pulsar-broker
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         serviceDescriptor: zookeeper-service-descriptor
       # 省略其他的选项
     - componentDefRef: bookies
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         serviceDescriptor: zookeeper-service-descriptor
       # 省略其他的选项
   ```

   在创建 Pulsar Cluster 对象时，`serviceRefs` 将引用声明中的 `pulsarZookeeper` 映射到具体的 `serviceDescriptor`。其中 `name` 就是在 ClusterDefinition 的 `serviceRefs` 中定义的引用声明的名称，而 `serviceDescriptor` 的值就是步骤 1 中的外部的 ZooKeeper。

#### 关联映射 KubeBlock 提供的独立集群部署的 ZooKeeper 服务

即关联映射到一个独立的 KubeBlocks ZooKeeper Cluster。

1. 在 KubeBlocks 创建一个名为 `kb-zookeeper-for-pulsar` 的 ZooKeeper 集群。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: kb-zookeeper-for-pulsar
     namespace: default
   spec:
     clusterDefinitionRef: pulsar-zookeeper
     clusterVersionRef: pulsar-2.11.2
     componentSpecs:
     - componentDefRef: zookeeper
       monitor: false
       name: zookeeper
       noCreatePDB: false
       replicas: 3
       resources:
         limits:
           memory: 512Mi
         requests:
           cpu: 100m
           memory: 512Mi
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
           - ReadWriteOnce
           resources:
             requests:
               storage: 20Gi
   terminationPolicy: WipeOut
   ```

2. 在创建 Pulsar 集群时，引用上面的 ZooKeeper 集群。

   把 `serviceRefs` 中的 `cluster` 的值填写为步骤 1 中的 KubeBlocks ZooKeeper 集群的名称。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: pulsar
     namespace: default
   spec:
     clusterDefinitionRef: pulsar
     clusterVersionRef: pulsar-2.11.2
     componentSpecs:
     - componentDefRef: pulsar-broker
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         cluster: kb-zookeeper-for-pulsar
       # 省略其他的选项
     - componentDefRef: bookies
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         cluster: kb-zookeeper-for-pulsar
       # 省略其他的选项
   ```

## 注意事项与限制

KubeBlocks v0.7.0 版本中的外部组件引用功能为 alpha 版本，具有以下几个方面的使用限制：

* ClusterDefinition 引用声明中的 `name` 具有 Cluster 维度的语义一致性。也就是说，同一个 name 会被认为是同一个服务引用，并且不允许在创建 Cluster 时关联不同的映射。
* 如果在创建 Cluster 时同时指定了两种方式的关联映射（基于 serviceDescriptor 和 cluster），那么基于 cluster 的关联映射具有更高的优先级，serviceDescriptor 会被忽略。
* 如果在创建 Cluster 时使用基于 cluster 的关联映射，则不会对 ClusterDefinition 中定义的 `ServiceKind` 与 `ServiceVersion` 进行校验。

  而如果使用基于 `serviceDescriptor` 的关联映射，则会将 `serviceDescriptor` 中的 `ServiceKind` 与 `ServiceVersion` 与 ClusterDefinition 中定义的 `ServiceKind` 与 `ServiceVersion` 进行校验。只有两者匹配时，才会正确映射。
* v0.7.0 版本仅支持通过配置模板渲染使用 ClusterDefinition 中的引用声明。后续版本会支持更多方式。
