---
title: 停止/启动集群
description: 如何停止/启动集群
keywords: [mysql, 停止集群, 启动集群]
sidebar_position: 5
sidebar_label: 停止/启动
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 停止/启动集群

您可以停止/启动集群以释放计算资源。停止集群后，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。您也可以重新启动该集群，使其恢复到停止集群前的状态。

## 停止集群

1. 配置集群名称，并执行以下命令来停止该集群。

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

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

    ```bash
    kubectl edit cluster mycluster -n demo
    ```

    将 replicas 设为 0，删除 Pods。

    ```yaml
    ...
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      terminationPolicy: Delete
      componentSpecs:
      - name: mysql
        componentDefRef: mysql
        disableExporter: true  
        replicas: 0 # 修改该参数值
    ...
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster stop mycluster -n demo
    ```

    </TabItem>

    </Tabs>

2. 查看集群状态，确认集群是否已停止。

    <Tabs>

    <TabItem value="kubectl" label="kubectl" default>

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>

## 启动集群
  
1. 配置集群名称，并执行以下命令来启动该集群。

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
      clusterName: mycluster
      type: Start
    EOF 
    ```

    </TabItem>
  
    <TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

    ```bash
    kubectl edit cluster mycluster -n demo
    ```

    将 replicas 数值调整为停止集群前的数量，再次启动集群。

    ```yaml
    ...
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      terminationPolicy: Delete
      componentSpecs:
      - name: mysql
        componentDefRef: mysql
        disableExporter: true
        replicas: 3 # 修改该参数值
    ...
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster start mycluster -n demo
    ```

    </TabItem>

    </Tabs>

2. 查看集群状态，确认集群是否再次启动。

    <Tabs>

    <TabItem value="kubectl" label="kubectl" default>

    ```bash
    kubectl get cluster mycluster -n demo
    ```

    </TabItem>

    <TabItem value="kbcli" label="kbcli">

    ```bash
    kbcli cluster list mycluster -n demo
    ```

    </TabItem>

    </Tabs>
