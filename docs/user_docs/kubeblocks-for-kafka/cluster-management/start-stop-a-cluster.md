---
title: Stop/Start a Kafka cluster
description: How to start/stop a Kafka cluster
keywords: [mongodb, stop a kafka cluster, start a kafka cluster]
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

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-stop
     namespace: demo
   spec:
     clusterName: mycluster
     type: Stop
   EOF
   ```

   </TabItem>

   <TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   Configure the value of `spec.componentSpecs.replicas` as 0 to delete pods.

   ```yaml
   ...
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2
     terminationPolicy: Delete
     componentSpecs:
     - name: kafka
       componentDefRef: kafka
       disableExporter: true  
       replicas: 0 # Change this value
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

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-start
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

   Change the value of `spec.componentSpecs.replicas` back to the original amount to start this cluster again.

   ```yaml
   ...
   spec:
     clusterDefinitionRef: kafka
     clusterVersionRef: kafka-3.3.2
     terminationPolicy: Delete
     componentSpecs:
     - name: kafka
       componentDefRef: kafka
       disableExporter: true   
       replicas: 1 # Change this value
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
