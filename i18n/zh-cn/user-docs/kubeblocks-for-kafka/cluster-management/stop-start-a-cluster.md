---
title: 停止/启动集群
description: 如何停止/启动集群
keywords: [kafka, 停止集群, 启动集群]
sidebar_position: 6
sidebar_label: 停止/启动
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

## 停止集群

***步骤：***

1. 配置集群名称，并执行以下命令来停止该集群。

   <Tabs>

   <TabItem value="kbcli" label="kbcli" default>

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

   </TabItem>

   <TabItem value="OpsRequest" label="OpsRequest">

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

   将 replicas 设为 0，删除 Pods。

   ```yaml
   kubectl edit cluster mycluster -n demo
   >
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

2. 检查集群的状态，查看其是否已停止。

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

## 启动集群
  
1. 配置集群名称，并执行以下命令来启动该集群。

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

   将 replicas 数值调整为停止集群前的数量，再次启动集群。

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

2. 查看集群状态，确认集群是否已启动。

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
