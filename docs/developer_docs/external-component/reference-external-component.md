---
title: Reference external component
description: KubeBlocks supports referencing the external component to manage cluster flexibly.
keywords: [external component]
sidebar_position: 1
sidebar_label: Reference external component
---

# Reference an external component

:::note

The external component function is an alpha version and there might be major revisions and changes in the future.

:::

## What is referencing an external component?

Some database clusters rely on metadata storage for distributed coordination and dynamic configuration. However, as the number of database clusters increases, the metadata storage itself can consume a significant amount of resources. Examples of such components include ZooKeeper in Pulsar. To reduce overhead, you can reference the same external component in multiple database clusters now.

Referencing an external component in KubeBlocks means referencing an external or KubeBlocks-based component in a declarative manner in a KubeBlocks cluster.

As its definition indicates, referencing the external component can be divided into two types:

* Referencing an external component

  This external component can be Kubernetes-based or non-Kubernetes. When referencing this component, first create a ServiceDescriptor CR (custom resources) which defines both the service and resources for referencing.

* Reference a KubeBlocks-based component

  This type of component is based on KubeBlocks clusters. When referencing this component, just fill in the referenced Cluster and no ServiceDescriptor is required.

## Examples of referencing an external component

The following examples show how a Pulsar cluster created by the KubeBlocks add-on references ZooKeeper as an external component. The instructions below include two parts:

1. [Create an external component reference declaration](#create-an-external-component-reference-declaration) when installing KubeBlocks or enabling an add-on.
2. [Define the mapping relation of the external component](#define-the-mapping-relation-of-the-external-component) when creating a cluster.

A KubeBlocks Pulsar cluster is composed of components including proxy, broker, bookies, and ZooKeeper and broker and bookies rely on ZooKeeper to provide metadata storage and interaction.

:::note

For more information about the KubeBlocks Pulsar cluster, refer to [KubeBlocks for Pulsar](./../../user_docs/kubeblocks-for-pulsar/cluster-management/create-pulsar-cluster-on-kubeblocks.md).

:::

### Create an external component reference declaration

1. Declare the referenced component in `componentDefs` in the ClusterDefinition.

    In this example, the broker and bookies components of the Pulsar cluster rely on the ZooKeeper component, so you need to add the declaration of ZooKeeper in the `componentDefs` of broker and bookies.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: ClusterDefinition
    metadata:
      name: pulsar
      labels:
        {{- include "pulsar.labels" . | nindent 4 }}
    spec:
      type: pulsar
      # Here omit other definitions
      componentDefs:
        - name: pulsar-broker
          workloadType: Stateful
          characterType: pulsar-broker
          serviceRefDeclarations:
          - name: pulsarZookeeper
            serviceRefDeclarationSpecs:
              - serviceKind: zookeeper
                serviceVersion: ^3.8.\d{1,2}$
          # Here omit other definitions
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
        # Here omit other component definitions
    ```

    `serviceRefDeclarations` above describe the external component referencing declarations, in which both `pulsar-broker` and `bookies` declare a component named `pulsarZookeeper`. It means `pulsar-broker` and `bookies` require a component whose name is `pulsarZookeeper`, `serviceKind` is `zookeeper`, and `serviceVersion` matches the `^3.8.\d{1,2}$` regular expression.

    This `pulsarZookeeper` reference declaration will be mapped to your specified ZooKeeper cluster when you create a Pulsar cluster.

2. Define the usage of this ZooKeeper component in the component provider.

    After the external component is declared in the ClusterDefinition, you can use the pre-defined `pulsarZookeeper` in any definition in the ClusterDefinition.

    For example, when starting the pulsar-broker and bookies components, upload the address of the ZooKeeper component to the configuration of pulsar-broker and bookies. Then you can reference this ZooKeeper component when rendering the pulsar-broker and bookies configuration templates. Below is an example of generating zookeeperServers in the `broker-env.tpl`.

    :::note

    From the above declaration, ClusterDefinition only knows this is a ZooKeeper component but has no idea of which ZooKeeper is provided by whom. Therefore, you need to map this ZooKeeper component when creating a cluster. Follow the instructions in the next section.

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

    # Try to get zookeeper from service reference first, if zookeeper service reference is empty, get default zookeeper componentDef in ClusterDefinition
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

    Currently, KubeBlocks only supports referencing the endpoint and port of the declared components when rendering the configuration templates. Other referencing options and paradigms will be supported in the future, for example, directly referencing the account and password of the component in `env`.

    :::

### Define the mapping relation of the external component

Based on the above example, when creating a Pulsar cluster, the ZooKeeper component mapping can be divided into two types:

* Mapping the external ZooKeeper component.
* Mapping the ZooKeeper component deployed by an individual cluster provided by KubeBlocks.

#### Mapping the external ZooKeeper component

1. Create a ServiceDescriptor CR object in a Kubernetes cluster.

   In KubeBlocks, ServiceDescriptor is used to describe the API object of the component reference information. ServiceDescriptor can separate a Kubernetes-based or non-Kubernetes component and provide it to other cluster objects in KubeBlocks for referencing.

   ServiceDescriptor can be used to solve issues like service dependency, component dependency, and component sharing in KubeBlocks.

   Below is an example of CR object.

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

2. Reference the external ZooKeeper component when creating a Pulsar cluster.

   The following example shows how to create a Pulsar cluster referencing the external ZooKeeper component mentioned above.

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
       # Here omit other definitions
     - componentDefRef: bookies
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         serviceDescriptor: zookeeper-service-descriptor
       # Here omit other definitions
   ```

   When creating the Pulsar Cluster object, `serviceRefs` maps `pulsarZookeeper` in the declaration to the specific `serviceDescriptor`. `name` in `serviceRefs` corresponds to the component referencing name defined in the ClusterDefinition and the value of `serviceDescriptor` is the name of `ServiceDescriptor` in Step 1.

#### Mapping the ZooKeeper component deployed by an individual cluster provided by KubeBlocks

This mapping relation refers to mapping an external component to an individual KubeBlocks ZooKeeper cluster.

1. Create a KubeBlocks ZooKeeper cluster named `kb-zookeeper-for-pulsar`.

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

2. Reference the above ZooKeeper cluster when creating a Pulsar cluster.

   Fill in the value of `cluster` in `serviceRefs` with the name of the KubeBlocks ZooKeeper cluster in Step 1.

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
       # Here omit other definitions
     - componentDefRef: bookies
       serviceRefs:
       - name: pulsarZookeeper
         namespace: default
         cluster: kb-zookeeper-for-pulsar
       # Here omit other definitions
   ```

## Cautions and limits

KubeBlocks v0.7.0 only provides an alpha version of the external component referencing function and there are several limits.

* The `name` in the ClusterDefinition of component referencing declaration maintains the semantic consistency on the cluster level, which means the same names are identified as the same component referencing and they cannot be mapped to different clusters.
* If both serviceDescriptor-based and cluster-based mapping are specified when creating a cluster, the cluster-based one enjoys higher priority and the serviceDescriptor-based one will be ignored.
* If the cluster-based mapping is used when creating a cluster, the `serviceKind` and `serviceVersion` defined in the ClusterDefinition will not be verified.

  If the serviceDescriptor-based mapping is adopted, KubeBlocks will verify the `serviceKind` and `serviceVersion` in the `serviceDescriptor` by comparing the `serviceKind` and `serviceVersion` defined in the ClusterDefinition. Mapping then is performed only when the values match.
* For v0.7.0, the usage of component referencing in the ClusterDefinition is supported only by rendering the configuration templates. Other usage options will be supported in the future.
