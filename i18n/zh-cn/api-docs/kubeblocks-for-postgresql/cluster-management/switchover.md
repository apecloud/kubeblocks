---
title: 切换 PostgreSQL 集群
description: 如何切换 PostgreSQL 集群
keywords: [postgresql, 切换集群, switchover]
sidebar_position: 6
sidebar_label: 切换
---

# 切换 PostgreSQL 集群

数据库 switchover 是指在数据库集群中将主数据库的角色切换到备用数据库的过程，使备用数据库成为新的主数据库实例。通常在主数据库故障、维护或升级时执行 switchover 操作，以确保数据库服务的高可用性和连续性。可使用 kubectl 命令对 PostgreSQL 集群版执行切换，KubeBlocks 将切换实例角色。

## 开始之前

* 确保集群正常运行。
  
  ```bash
  kubectl get cluster mycluster -n demo
  ```

* 检查以下角色探针参数是否存在，确认是否已启用探针。

   ```bash
   kubectl get cd postgresql -o yaml
   >
    probes:
      roleProbe:
        failureThreshold: 2
        periodSeconds: 1
        timeoutSeconds: 1
   ```

## 切换集群

您可将 PostgreSQL 主备版的从节点切换为主节点，原来的主节点实例将被切换为从节点实例。

`instanceName` 字段的值定义了本次切换是否指定了新的主节点实例。

* 不指定主节点实例进行切换。

  ```yaml
  kubectl apply -f -<<EOF
  apiVersion: apps.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
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
    name: mycluster-switchover
    namespace: demo
  spec:
    clusterName: mycluster
    type: Switchover
    switchover:
    - componentName: postgresql
      instanceName: 'mycluster-postgresql-2'
  >>
  ```

## 验证集群切换

检查实例状态，验证切换是否成功。

```bash
kubectl get cluster mycluster -n demo

kubectl -n demo get po -L kubeblocks.io/role 
```

## 处理异常情况

如果报错，请参考[异常处理](./../../handle-an-exception/handle-a-cluster-exception.md)排查问题。
