---
title: Stop/Start a PostgreSQL cluster
description: How to start/stop a PostgreSQL cluster
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start PostgreSQL Cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

### Option 1. (Recommended) Use kbcli

Configure the name of your cluster and run the command below to stop this cluster. 

```bash
kbcli cluster stop <name>
```

***Example***

```bash
kbcli cluster stop pg-cluster
```

### Option 2. Create an OpsRequest

Run the command below to stop a cluster.
```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: pg-cluster
  generateName: stop-
spec:
  # cluster ref
  clusterRef: pg-cluster
  type: Stop
EOF
```

### Option 3. Change the YAML file of the cluster

Configure replicas as 0 to delete pods.
```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: pg-cluster
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-14.7.0
  terminationPolicy: WipeOut
  componentSpecs:
  - name: pg-replication
    componentDefRef: postgresql
    monitor: false  
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

## How KubeBlocks starts a cluster
  
### Option 1. (Recommended) Use kbcli

Configure the name of your cluster and run the command below to stop this cluster. 

```bash
kbcli cluster start <name>
```

***Example***

```bash
kbcli cluster start pg-cluster
```

### Option 2. Create an OpsRequest

Run the command below to start a cluster.

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: pg-cluster
  generateName: start-
spec:
  # cluster ref
  clusterRef: pg-cluster
  type: Start
EOF 
```

### Option 3. Change the YAML file of the cluster

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: pg-cluster
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-14.7.0
  terminationPolicy: WipeOut
  componentSpecs:
  - name: pg-replication
    componentDefRef: postgresql
    monitor: false  
    replicas: 1
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
