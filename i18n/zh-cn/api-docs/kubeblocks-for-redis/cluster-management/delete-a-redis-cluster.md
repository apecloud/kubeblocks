---
title: 删除集群
description: 如何删除集群
keywords: [redis, 删除集群, 删除保护]
sidebar_position: 6
sidebar_label: 删除保护
---

# 删除集群

## 终止策略

:::note

终止策略决定了删除集群的方式。

:::

| **terminationPolicy** | **删除操作**                           |
|:----------------------|:-------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。       |
| `Halt`                | `Halt` 删除工作负载资源，但保留 PVC。 |
| `Delete`              | `Delete` 删除工作负载资源和 PVC，但保留备份。   |
| `WipeOut`             | `WipeOut` 删除工作负载资源、PVC 和所有相关资源（包括备份）。    |

执行以下命令查看终止策略。

```bash
kubectl -n demo get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS    AGE
mycluster   redis                redis-7.0.6   Delete               Running   10m
```

## 步骤

删除指定集群。

```bash
kubectl delete cluster mycluster -n demo
```

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```
