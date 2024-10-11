---
title: Stop/Start a PostgreSQL cluster
description: How to start/stop a PostgreSQL cluster
keywords: [postgresql, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start PostgreSQL Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

   </TabItem>

   <TabItem value="OpsRequest" label="OpsRequest">

   Apply an OpsRequest to stop a cluster.

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

   Configure replicas as 0 to delete pods.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   spec:
     clusterDefinitionRef: postgresql
     clusterVersionRef: postgresql-14.8.0
     terminationPolicy: Delete
     componentSpecs:
     - name: postgresql
       componentDefRef: postgresql
       disableExporter: true  
       replicas: 0
   ......
   ```

   </TabItem>

   </Tabs>

2. Check the status of the cluster to see whether it is stopped.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   <TabItem value="kubectl" label="kubectl">

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   </Tabs>

## Start a cluster

1. Configure the name of your cluster and run the command below to start this cluster.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster start mycluster -n demo
   ```

   </TabItem>

   <TabItem value="OpsRequest" label="OpsRequest">

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
  
   <TabItem value="Edit cluster YAML file" label="Edit cluster YAML file">

   Change replicas back to the original amount to start this cluster again.

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: mycluster
   spec:
     clusterDefinitionRef: postgresql
     clusterVersionRef: postgresql-14.8.0
     terminationPolicy: Delete
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       disableExporter: true
       replicas: 1
   ......
   ```

   </TabItem>

   </Tabs>

2. Check the status of the cluster to see whether it is running again.

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   <TabItem value="kubectl" label="kubectl">

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   </Tabs>
