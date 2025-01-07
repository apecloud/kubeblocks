---
title: Stop/Start a MongoDB cluster
description: How to start/stop a MongoDB cluster
keywords: [mongodb, stop a mongodb cluster, start a mongodb cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start a MongoDB Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again to restore it to the state it was in before it was stopped.

## Stop a cluster

***Steps:***

1. Configure the name of your cluster and run the command below to stop this cluster.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

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
      clusterDefinitionRef: mongodb
      clusterVersionRef: mongodb-5.0
      terminationPolicy: Delete
      componentSpecs:
      - name: mongodb
        componentDefRef: mongodb
        disableExporter: true  
        replicas: 0
        volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
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
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>

## Start a cluster
  
1. Configure the name of your cluster and run the command below to start this cluster.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

    Apply an OpsRequest to start a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name:ops-start
      namespace: demo
    spec:
      clusterName: mycluster
      type: Start
    EOF 
    ```

    </TabItem>

    <TabItem value="Edit cluster YAML file" label="Edit cluster YAML File">

    Change replicas back to the original amount to start this cluster again.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mycluster
      namespace: demo
    spec:
      clusterDefinitionRef: mongodb
      clusterVersionRef: mongodb-5.0
      terminationPolicy: Delete
      componentSpecs:
      - name: mongodb
        componentDefRef: mongodb
        disableExporter: true  
        replicas: 1
        volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: standard
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 20Gi
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
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>
