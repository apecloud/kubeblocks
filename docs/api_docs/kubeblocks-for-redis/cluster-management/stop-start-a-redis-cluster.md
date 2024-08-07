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

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. Start this cluster again if you want to restore the cluster resources from the original storage by snapshots.

## Stop a cluster

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
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  componentSpecs:
  - componentDefRef: redis
    enabledLogs:
    - running
    disableExporter: true
    name: redis
    replicas: 0
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-redis
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
```

</TabItem>

</Tabs>

## Start a cluster
  
<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

Run the command below to start a cluster.

```yaml
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
spec:
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
  clusterDefinitionRef: redis
  clusterVersionRef: redis-7.0.6
  componentSpecs:
  - componentDefRef: redis
    enabledLogs:
    - running
    disableExporter: true
    name: redis
    replicas: 1
    resources:
      limits:
        cpu: "0.5"
        memory: 0.5Gi
      requests:
        cpu: "0.5"
        memory: 0.5Gi
    serviceAccountName: kb-redis
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
```

</TabItem>

</Tabs>
