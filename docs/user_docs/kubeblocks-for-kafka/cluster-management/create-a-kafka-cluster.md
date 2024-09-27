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

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure Kafka Addon is enabled with `kbcli addon list`. If this Addon is not enabled, [enable it](./../../installation/install-with-kbcli/install-addons.md) first.

  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  kafka                          0.9.0           community   Enabled    true
  ...
  ```

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

The cluster creation command is simply `kbcli cluster create`. Further, you can customize your cluster resources as demanded by using the `--set` flag.

```bash
kbcli cluster create mycluster --cluster-definition kafka -n demo
```

See the table below for detailed descriptions of customizable parameters, setting the `--termination-policy` is necessary, and you are strongly recommended to turn on the monitor and enable all logs.

📎 Table 1. kbcli cluster create flags description

| Option                                                                    | Description                    |
|---------------------------------------------------------------------------|---------------------------------------------------------------------------------|
| --mode='combined'                                                         | Mode for Kafka kraft cluster, 'combined' is combined Kafka controller and broker,'separated' is broker and controller running independently. Legal values [combined, separated] |
| --replicas=1                                                              | The number of Kafka broker replicas for combined mode. In combined mode, this number also refers to the number of the kraft controller. Valid value range[1,3,5] |
| --broker-replicas=1                                                       | The number of Kafka broker replicas for separated mode.  |
| --controller-replicas=1                                                   | The number of Kafka controller replicas for separated mode. In separated mode, this number refers to the number of kraft controller. Valid value range [1,3,5] |
| --termination-policy='Delete'                                             | The termination policy of cluster. Legal values [DoNotTerminate, Halt, Delete, WipeOut]. <br />- DoNotTerminate: DoNotTerminate blocks the delete operation. <br />- Halt: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br />- Delete: Delete is based on Halt and deletes PVCs. - WipeOut: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |
| --storage-enable=false                                                    | Specify whether to enable storage for Kafka.           |
| --host-network-accessible=false                                           | Specify whether the cluster can be accessed from within the VPC. |
| --publicly-accessible=false                                               | Specify whether the cluster can be accessed from the public internet.   |
| --broker-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64'     | Kafka broker's jvm heap setting.  |
| --controller-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64' | Kafka controller's jvm heap setting for separated mode.  The setting takes effect only when mode='separated'.  |
| --cpu=1                                                                   | CPU cores.      |
| --memory=1                                                                | Memory, the unit is Gi. |
| --storage=20                                                              | Data Storage size, the unit is Gi.   |
| --storage-class=''                                                        | The StorageClass for Kafka Data Storage.   |
| --meta-storage=5                                                          | Metadata Storage size, the unit is Gi.   |
| --meta-storage-class=''                                                   | The StorageClass for Kafka Metadata Storage.  |
| --monitor-enable=false                                                    | Specify whether to enable monitor for Kafka.    |
| --monitor-replicas=1                                                      | The number of Kafka monitor replicas.  |
| --sasl-enable=false                                                       | Specify whether to enable authentication using SASL/PLAIN for Kafka. <br /> -server: admin/kubeblocks <br /> -client: client/kubeblocks  <br /> built-in jaas file stores on /tools/client-ssl.properties   |

</TabItem>

<TabItem value="kubectl" label="kubectl">

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
| `spec.tolerations`                    | It is an array that specifies tolerations attached to the cluster's Pods, allowing them to be scheduled onto nodes with matching taints.  |
| `spec.services` | It defines the services to access a cluster. |
| `spec.componentSpecs`                 | It is the list of components that define the cluster components. This field allows customized configuration of each component within a cluster.   |
| `spec.componentSpecs.componentDefRef` | It is the name of the component definition that is defined in the cluster definition and you can get the component definition names with `kubectl get clusterdefinition kafka -o json \| jq '.spec.componentDefs[].name'`.   |
| `spec.componentSpecs.name`            | It specifies the name of the component.     |
| `spec.componentSpecs.replicas`        | It specifies the number of replicas of the component.  |
| `spec.componentSpecs.resources`       | It specifies the resource requirements of the component.  |

</TabItem>

</Tabs>