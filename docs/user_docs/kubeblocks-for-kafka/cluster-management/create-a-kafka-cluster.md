---
title: Create a cluster for Kafka
description: Guide for cluster creation for kafka
keywords: [kafka, cluster, management]
sidebar_position: 1
sidebar_label: Create
---

# Create a Kafka cluster

This document shows how to create a Kafka cluster.

## Before you start

* [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md). 
* Install KubeBlocks: You can install KubeBlocks by [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
* Make sure kafka addon is enabled with `kbcli addon list`.

  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  kafka                        Helm   Enabled                   true
  ...
  ```

:::note

* KubeBlocks integrates Kafka v3.3.2, running it in KRaft mode.
* You are not recommended to use kraft cluster in combined mode in production environment.
* The controller number suggested ranges from 3 to 5, out of complexity and availability.

:::
## Create a Kafka cluster

<Tabs>
<TabItem value="using kbcli" label="Using kbcli" default>

The cluster creation command is simply `kbcli cluster create`. Further, you can customize your cluster resources as demanded by using the `--set` flag.

```bash
kbcli cluster create kafka
```

See the table below for detailed descriptions of customizable parameters, setting the `--termination-policy` is necessary, and you are strongly recommended to turn on the monitor and enable all logs.

📎 Table 1. kbcli cluster create flags description

| Option                                                                    | Description                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|---------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --mode='combined'                                                         | Mode for Kafka kraft cluster, 'combined' is combined Kafka controller and broker,'separated' is broker and controller running independently. Legal values [combined, separated]                                                                                                                                                                                                                                                                   |
| --replicas=1                                                              | The number of Kafka broker replicas for combined mode. In combined mode, this number also refers to the number of the kraft controller. Valid value range[1,3,5]                                                                                                                                                                                                                                                                                  |
| --broker-replicas=1                                                       | The number of Kafka broker replicas for separated mode.                                                                                                                                                                                                                                                                                                                                                                                           |
| --controller-replicas=1                                                   | The number of Kafka controller replicas for separated mode. In separated mode, this number refers to the number of kraft controller. Valid value range [1,3,5]                                                                                                                                                                                                                                                                                    |
| --termination-policy='Delete'                                             | The termination policy of cluster. Legal values [DoNotTerminate, Halt, Delete, WipeOut]. <br />- DoNotTerminate: DoNotTerminate blocks the delete operation. <br />- Halt: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br />- Delete: Delete is based on Halt and deletes PVCs. - WipeOut: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |
| --storage-enable=false                                                    | Specify whether to enable storage for Kafka.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --host-network-accessible=false                                           | Specify whether the cluster can be accessed from within the VPC.                                                                                                                                                                                                                                                                                                                                                                                  |
| --publicly-accessible=false                                               | Specify whether the cluster can be accessed from the public internet.                                                                                                                                                                                                                                                                                                                                                                             |
| --broker-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64'     | Kafka broker's jvm heap setting.                                                                                                                                                                                                                                                                                                                                                                                                                  |
| --controller-heap='-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64' | Kafka controller's jvm heap setting for separated mode.  The setting takes effect only when mode='separated'.                                                                                                                                                                                                                                                                                                                                     |
| --cpu=1                                                                   | CPU cores.                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --memory=1                                                                | Memory, the unit is Gi.                                                                                                                                                                                                                                                                                                                                                                                                                           |
| --storage=20                                                              | Data Storage size, the unit is Gi.                                                                                                                                                                                                                                                                                                                                                                                                                |
| --storage-class=''                                                        | The StorageClass for Kafka Data Storage.                                                                                                                                                                                                                                                                                                                                                                                                          |
| --meta-storage=5                                                          | Metadata Storage size, the unit is Gi.                                                                                                                                                                                                                                                                                                                                                                                                            |
| --meta-storage-class=''                                                   | The StorageClass for Kafka Metadata Storage.                                                                                                                                                                                                                                                                                                                                                                                                      |
| --monitor-enable=false                                                    | Specify whether to enable monitor for Kafka.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --monitor-replicas=1                                                      | The number of Kafka monitor replicas.                                                                                                                                                                                                                                                                                                                                                                                                             |
| --sasl-enable=false                                                       | Specify whether to enable authentication using SASL/PLAIN for Kafka. <br /> -server: admin/kubeblocks <br /> -client: client/kubeblocks  <br /> built-in jaas file stores on /tools/client-ssl.properties                                                                                                                                                                                                                                                                  |
</TabItem>

<TabItem value="using kubectl" label="Using kubectl" default>

* Create a Kafka cluster in combined mode.

    ```bash
    # create kafka in combined mode 
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: kafka-combined
      namespace: default
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
      name: kafka-separated
      namespace: default
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

</TabItem>

</Tabs>
