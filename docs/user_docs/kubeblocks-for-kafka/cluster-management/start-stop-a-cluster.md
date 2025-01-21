---
title: Stop/Start a Kafka cluster
description: How to start/stop a Kafka cluster
keywords: [kafka, stop a kafka cluster, start a kafka cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start a Kafka Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

***Steps:***

1. Configure the name of your cluster and run the command below to stop this cluster.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Run the command below to stop a cluster.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name:  kafka-combine-stop
     namespace: demo
   spec:
     clusterName:  mycluster
     type: Stop
   EOF
   ```

   </TabItem>

   <TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Configure the value of `spec.componentSpecs.stop` to `true` to delete pods.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   ...
   spec:
   ...
     componentSpecs:
       - name: kafka-combine
         stop: true  # set stop `true` to stop the component
         replicas: 1
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. Check the status of the cluster to see whether it is stopped.

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   </Tabs>

## Start a cluster
  
1. Configure the name of your cluster and run the command below to start this cluster.

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   Apply an OpsRequest to start the cluster.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: kafka-combined-start
     namespace: demo
   spec:
     clusterName: mycluster
     type: Start
   EOF
   ```

   </TabItem>

   <TabItem value="Edit cluster YAML file" label="Edit cluster YAML File">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Change the value of `spec.componentSpecs.stop` to `false` to start this cluster again.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   ...
   spec:
   ...
     componentSpecs:
       - name: kafka-combine
         stop: false  # set to `false` (or remove this field) to start the component
         replicas: 1
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster start mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. Check the status of the cluster to see whether it is running again.

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   </Tabs>
