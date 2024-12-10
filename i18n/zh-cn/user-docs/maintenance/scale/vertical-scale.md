---
title: 垂直扩缩容
description: 如何对集群进行垂直扩缩容
keywords: [垂直扩缩容, 变配]
sidebar_position: 2
sidebar_label: 垂直扩缩容
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，可通过垂直扩容将资源类别从 1C2G 调整为 2C4G。

本文档以 MySQL 为示例，演示如何对集群进行垂直扩缩容。

:::note

从 v0.9.0 开始，MySQL 和 PostgreSQL 集群在进行水平扩缩容后，KubeBlocks 会根据新的规格自动匹配合适的配置模板。这因为 KubeBlocks 在 v0.9.0 中引入了动态配置功能。该功能简化了配置参数的过程，节省了时间和精力，并减少了由于配置错误引起的性能问题。有关详细说明，请参阅[配置](./../../kubeblocks-for-apecloud-mysql/configuration/configuration.md)。

:::

## 开始之前

查看集群状态是否为 `Running`，否则，后续操作可能失败。

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   mysql                mysql-8.0.33   Delete               Running   4d18h
```

## 步骤

可通过以下两种方式进行垂直扩缩容。

<Tabs>

<TabItem value="OpsRequest" label="OpsRequest" default>

1. 对指定集群应用 OpsRequest，可按需修改参数值。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
     namespace: demo
   spec:
     clusterName: mycluster
     type: VerticalScaling
     verticalScaling:
     - componentName: mysql
       requests:
         memory: "2Gi"
         cpu: "1"
       limits:
         memory: "4Gi"
         cpu: "2"
   EOF
   ```

2. 查看运维任务状态，验证该任务是否执行成功。

   ```bash
   kubectl get ops -n demo
   >
   NAMESPACE   NAME                   TYPE              CLUSTER     STATUS    PROGRESS   AGE
   demo        ops-vertical-scaling   VerticalScaling   mycluster   Succeed   3/3        6m
   ```

   如果有报错，可执行 `kubectl describe ops -n demo` 命令查看该运维操作的相关事件，协助排障。

3. 查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>
  
<TabItem value="修改集群 YAML 文件" label="修改集群 YAML 文件">

1. 修改 YAML 文件中 `spec.componentSpecs.resources` 的配置。`spec.componentSpecs.resources` 控制资源的请求值和限制值，修改参数值将触发垂直扩缩容。

   ```bash
   kubectl edit cluster mycluster -n demo
   ```

   在编辑器中修改 `spec.componentSpecs.resources` 的参数值。

   ```yaml
   ...
   spec:
     clusterDefinitionRef: apecloud-mysql
     clusterVersionRef: ac-mysql-8.0.30
     componentSpecs:
     - name: mysql
       componentDefRef: mysql
       replicas: 3
       resources: # 修改参数值
         requests:
           memory: "2Gi"
           cpu: "1"
         limits:
           memory: "4Gi"
           cpu: "2"
   ...
   ```

2. 查看相应资源是否变更。

   ```bash
   kubectl describe cluster mycluster -n demo
   ```

</TabItem>

</Tabs>
