---
title: 停止/启动集群
description: 如何停止/启动集群
keywords: [mongodb, 停止集群, 启动集群]
sidebar_position: 5
sidebar_label: 停止/启动
---

# 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

## 停止集群

可通过 OpsRequest 或修改集群 YAML 文件停止集群。

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

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

将 replicas 的值修改为 0，删除 pod。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
   name: mycluster
   namespace: demo
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  terminationPolicy: Delete
  componentSpecs:
  - name: kafka
    componentDefRef: kafka
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

## 启动集群
  
可通过 OpsRequest 或修改集群 YAML 文件启动集群。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

执行以下命令启动集群。

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

<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

将 replicas 数值修改为初始值，启动集群。

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
   name: mycluster
   namespace: demo
spec:
  clusterDefinitionRef: kafka
  clusterVersionRef: kafka-3.3.2
  terminationPolicy: Delete
  componentSpecs:
  - name: kafka
    componentDefRef: kafka
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
