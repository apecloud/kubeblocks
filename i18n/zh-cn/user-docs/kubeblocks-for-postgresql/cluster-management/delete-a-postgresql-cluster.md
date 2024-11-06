---
title: 删除集群
description: 如何删除集群
keywords: [postgresql, 删除集群]
sidebar_position: 7
sidebar_label: 删除保护
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 删除 PostgreSQL 集群

:::note

终止策略决定了删除集群的方式。

:::

## 终止策略

| **终止策略** | **删除操作**                                                                     |
|:----------------------|:-------------------------------------------------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。                                                  |
| `Halt`                | `Halt` 删除集群资源（如 Pods、Services 等），但保留 PVC。停止其他运维操作的同时，保留了数据。但 `Halt` 策略在 v0.9.1 中已启用，设置为 `Halt` 的效果与 `DoNotTerminate` 相同。  |
| `Delete`              | `Delete` 在 `Halt` 的基础上，删除 PVC 及所有持久数据。                              |
| `WipeOut`             | `WipeOut`  删除所有集群资源，包括外部存储中的卷快照和备份。使用该策略将会删除全部数据，特别是在非生产环境，该策略将会带来不可逆的数据丢失。请谨慎使用。   |

执行以下命令查看终止策略。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mycluster -n demo
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   demo        postgresql           postgresql-14.8.0   Delete               Running   Sep 28,2024 16:47 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    AGE
mycluster   postgresql           postgresql-14.8.0   Delete               Running   29m
```

</TabItem>

</Tabs>

## 步骤

执行以下命令，删除集群。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mycluster -n demo
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl delete cluster mycluster -n demo
```

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mycluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mycluster
```

</TabItem>

</Tabs>
