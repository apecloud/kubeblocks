---
title: Stop/Start a Redis cluster
description: How to start/stop a Redis cluster
keywords: [redis, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start a Redis Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

### Option 1. (Recommended) Use kbcli

Configure the name of your cluster and run the command below to stop this cluster. 

```bash
kbcli cluster stop <name>
```

***Example***

```bash
kbcli cluster stop redis-cluster
```

### Option 2. Create an OpsRequest

Run the command below to stop a cluster.
```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: stop-
spec:
  # cluster ref
  clusterRef: redis-cluster
  type: Stop
EOF
```

### Option 3. Change the YAML file of the cluster

Configure replicas as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: redis-cluster
spec:
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  terminationPolicy: Delete
  componentSpecs:
  - name: redis
    componentDefRef: redis
    monitor: true  
    replicas: 0
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  - name: redis-sentinel
    componentDefRef: redis-sentinel
    monitor: true  
    replicas: 0
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

## Start a cluster
  
### Option 1. (Recommended) Use kbcli

Configure the name of your cluster and run the command below to stop this cluster. 

```bash
kbcli cluster start <name>
```

***Example***

```bash
kbcli cluster start redis-cluster
```

### Option 2. Create an OpsRequest

Run the command below to start a cluster.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generateName: start-
spec:
  # cluster ref
  clusterRef: redis-cluster
  type: Start
EOF 
```

### Option 3. Change the YAML file of the cluster

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: redis-cluster
spec:
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  terminationPolicy: Delete
  componentSpecs:
  - name: redis
    componentDefRef: redis
    monitor: true  
    replicas: 2
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  - name: redis-sentinel
    componentDefRef: redis-sentinel
    monitor: true  
    replicas: 3
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```
