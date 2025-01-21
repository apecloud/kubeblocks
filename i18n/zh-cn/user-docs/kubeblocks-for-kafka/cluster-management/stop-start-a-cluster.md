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

您可以停止/启动集群以释放计算资源。停止集群后，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。您也可以重新启动该集群，使其恢复到停止集群前的状态。

## 停止集群

***步骤：***

1. 配置集群名称，并执行以下命令来停止该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name:  kafka-combine-stop
     namespace: demo
   spec:
     clusterName:  mycluster
     type: Stop
   EOF
   ```

   </TabItem>

   <TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   将 `spec.componentSpecs.stop` 设为 `true`，删除 Pods。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   ...
   spec:
   ...
     componentSpecs:
       - name: kafka-combine
         stop: true  # 将该值设置为 `true`，停止当前 component
         replicas: 1
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster stop mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. 检查集群的状态，查看其是否已停止。

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   </Tabs>

## 启动集群
  
1. 配置集群名称，并执行以下命令来启动该集群。

   <Tabs>

   <TabItem value="OpsRequest" label="OpsRequest" default>

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: operations.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: kafka-combined-start
     namespace: demo
   spec:
     clusterName: mycluster
     type: Start
   EOF
   ```

   </TabItem>

   <TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   将 `spec.componentSpecs.stop` 的值 设为 `false`，启动集群。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1
   kind: Cluster
   metadata:
     name: mycluster
     namespace: demo
   ...
   spec:
   ...
     componentSpecs:
       - name: kafka-combine
         stop: false  # 将该值设置为 `false` 或者删除该字段，启动当前 component
         replicas: 1
   ...
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster start mycluster -n demo
   ```

   </TabItem>

   </Tabs>

2. 查看集群状态，确认集群是否已启动。

   <Tabs>

   <TabItem value="kubectl" label="kubectl" default>

   ```bash
   kubectl get cluster mycluster -n demo
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli cluster list -n demo
   ```

   </TabItem>

   </Tabs>
