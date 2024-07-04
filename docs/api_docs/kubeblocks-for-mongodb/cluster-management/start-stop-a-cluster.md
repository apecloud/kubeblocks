---
title: Stop/Start a MongoDB cluster
description: How to start/stop a MongoDB cluster
keywords: [mongodb, stop a momgodb cluster, start a mongodb cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start a MongoDB Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

You can stop a cluster by OpsRequest or changing the YAML file of the cluster.

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

</Tabs>

## Start a cluster
  
You can stop a cluster by OpsRequest or changing the YAML file of the cluster.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

Run the command below to start a cluster.

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

</Tabs>
