---
title: 停止/启动集群
description: 如何停止/启动集群
keywords: [redis, 停止集群, 启动集群]
sidebar_position: 5
sidebar_label: 停止/启动
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 停止/启动集群

您可以停止/启动集群以释放计算资源。当集群停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你想恢复集群资源，可通过快照重新启动集群。

## 停止集群

您可通过创建 OpsRequest 或修改集群 YAML 文件来停止集群。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

执行以下命令，停止集群。

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

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

将副本数设置为 0，删除 Pod。

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

## 启动集群

您可通过创建 OpsRequest 或修改集群 YAML 文件来启动集群。
  
<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

执行以下命令，启动集群。

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

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

将副本数改为原始数量，重新启动该集群。

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