---
title: Stop/Start a Redis cluster
description: How to start/stop a Redis cluster
keywords: [redis, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start a Redis Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again to restore it to the state it was in before it was stopped.

## Stop a cluster

1. Configure the name of your cluster and run the command below to stop this cluster.

    <Tabs>

    <TabItem value="kbcli" label="kbcli" default>

    ```bash
    kbcli cluster stop mycluster -n demo
    ```

    </TabItem>

    <TabItem value="OpsRequest" label="OpsRequest">

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

    Configure replicas as 0 to delete pods.

    ```yaml
    kubectl edit cluster mycluster -n demo
    >
    spec:
      affinity:
        podAntiAffinity: Preferred
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionDef: redis
      componentSpecs:
      - componentDef: redis-7
        enabledLogs:
        - running
        disableExporter: true
        name: redis
        replicas: 0
        ......
      - componentDef: redis-sentinel-7
        name: redis-sentinel
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

    Change replicas back to the original amount to start this cluster again.

    ```yaml
    kubectl edit cluster mycluster -n demo
    >
    spec:
      affinity:
        podAntiAffinity: Preferred
        topologyKeys:
        - kubernetes.io/hostname
      clusterDefinitionDef: redis
      componentSpecs:
      - componentDef: redis-7
        enabledLogs:
        - running
        disableExporter: true
        name: redis
        replicas: 3
        ......
      - componentDef: redis-sentinel-7
        name: redis-sentinel
        replicas: 3
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
