---
title: Stop/Start a MySQL cluster
description: How to stop/start a MySQL cluster
keywords: [mysql, stop a cluster, start a cluster]
sidebar_position: 5
sidebar_label: Stop/Start
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Stop/Start a MySQL cluster

You can stop/start a cluster to save computing resources. When a cluster is stopped, the computing resources of this cluster are released, which means the pods of Kubernetes are released, but the storage resources are reserved. You can start this cluster again by snapshots if you want to restore the cluster resources.

## Stop a cluster

You can stop a cluster by creating an OpsRequest or changing the YAML file of the cluster.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: mycluster
  namespace: demo
spec:
  # cluster ref
  clusterName: mycluster
  type: Stop
EOF
```

</TabItem>
  
<TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

Configure replicas as 0 to delete pods.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  terminationPolicy: WipeOut
  componentSpecs:
  - name: mysql
    componentDefRef: mysql
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

You can start a cluster by creating an OpsRequest or changing the YAML file of the cluster.

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

```bash
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-start
  namespace: demo
spec:
  # cluster ref
  clusterName: mycluster
  type: Start
EOF 
```

</TabItem>
  
<TabItem value="Change the cluster YAML file" label="Change the cluster YAML file">

Change replicas back to the original amount to start this cluster again.

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
    name: mycluster
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  terminationPolicy: WipeOut
  componentSpecs:
  - name: mysql
    componentDefRef: mysql
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
