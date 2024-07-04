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
* [Install KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md).
* Make sure kafka addon is enabled with `kbcli addon list`. If this addon is not enabled, [enable it](./../../overview/supported-addons.md#use-addons) first.

  ```bash
  kbcli addon list
  >
  NAME                           VERSION         PROVIDER    STATUS     AUTO-INSTALL
  ...
  kafka                          0.9.0           community   Enabled    true
  ...
  ```

:::note

* KubeBlocks integrates Kafka v3.3.2, running it in KRaft mode.
* You are not recommended to use kraft cluster in combined mode in a production environment.
* The controller number suggested ranges from 3 to 5, out of complexity and availability.

:::

## Create a Kafka cluster

The cluster creation command is simply `kbcli cluster create`. Further, you can customize your cluster resources as demanded by using the `--set` flag.

```bash
kbcli cluster create kafka
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
