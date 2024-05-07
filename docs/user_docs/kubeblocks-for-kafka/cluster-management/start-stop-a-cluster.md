---
title: Stop/Start a Kafka cluster
description: How to start/stop a Kafka cluster
keywords: [mongodb, stop a kafka cluster, start a kafka cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

# Stop/Start a Kafka Cluster

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
  name: mycluster
  generateName: stop-
spec:
  # cluster ref
  clusterName: mycluster
  type: Stop
EOF
```

</TabItem>

<TabItem value="Cluster YAML File" label="Cluster YAML File">

Configure replicas as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  terminationPolicy: WipeOut
  componentSpecs:
  - name: kafka
    componentDefRef: kafka
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
  name: mycluster
  generateName: start-
spec:
  # cluster ref
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
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  terminationPolicy: WipeOut
  componentSpecs:
  - name: kafka
    componentDefRef: kafka
    monitor: false  
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

</TabItem>

</Tabs>
