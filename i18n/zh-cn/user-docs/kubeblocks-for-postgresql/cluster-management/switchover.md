---
title: 切换 PostgreSQL 集群
description: 如何切换 PostgreSQL 集群
keywords: [postgresql, 切换集群, switchover]
sidebar_position: 6
sidebar_label: 切换
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 切换 PostgreSQL 集群

你可以通过执行 kbcli 或 kubectl 命令切换 PostgreSQL 主备版。切换后，KubeBlocks 将修改实例角色。

## 开始之前

* 确保集群正常运行。
* 检查以下角色探针参数是否存在，确认是否已启用探针。

   ```bash
   kubectl get cd postgresql -o yaml
   >
   probes:
     roleProbe:
       failureThreshold: 3
       periodSeconds: 2
       timeoutSeconds: 1
   ```

## 切换集群

将 PostgreSQL 主备版的从节点切换为主节点，原来的主节点实例将被切换为从节点实例。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

* 不指定主节点实例进行切换。

    ```bash
    kbcli cluster promote mycluster
    ```

* 指定一个新的主节点实例进行切换。

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-postgresql-2'
    ```

* 如果有多个组件，可以使用 `--component` 参数指定一个组件。

    ```bash
    kbcli cluster promote mycluster --instance='mycluster-postgresql-2' --component='postgresql'
    ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

`instanceName` 的值决定了切换过程中是否指定了新的主节点实例。

* 不指定主节点实例进行切换。

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover-jhkgl
  spec:
    clusterRef: mycluster
    type: Switchover
    switchover:
    - componentName: postgresql
      instanceName: '*'
  >>
  ```

* 指定一个新的主节点实例进行切换。

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover-jhkgl
  spec:
    clusterRef: mycluster
    type: Switchover
    switchover:
    - componentName: postgresql
      instanceName: 'mycluster-postgresql-2'
  >>
  ```

</TabItem>

</Tabs>

## 验证集群切换

检查实例状态，验证切换是否成功。

```bash
kbcli cluster list-instances
```

## 处理异常情况

如果报错，请参考[异常处理](./../../handle-an-exception/handle-a-cluster-exception.md)排查问题。
