---
title: 删除集群
description: 如何删除集群
keywords: [mysql, 删除集群]
sidebar_position: 7
sidebar_label: 删除保护
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 删除集群

## 终止策略

:::note

终止策略决定了删除集群的方式。

:::

| **terminationPolicy** | **删除操作**                           |
|:----------------------|:-------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。     |
| `Halt`                | `Halt` 删除工作负载资源（如 statefulset、deployment 等），但保留 PVC。 |
| `Delete`              | `Delete` 删除工作负载资源和 PVC，但保留备份。 |
| `WipeOut`             | `WipeOut` 删除工作负载资源、PVC 和所有相关资源（包括备份）。 |

执行以下命令查看终止策略。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list mysql-cluster
>
NAME            NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mysql-cluster   default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Feb 06,2023 18:27 UTC+0800
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n demo get cluster mysql-cluster
>
NAME            CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    AGE
mysql-cluster   apecloud-mysql       ac-mysql-8.0.30   Delete               Running   67m
```

</TabItem>

</Tabs>

## 步骤

执行以下命令，删除集群。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete mysql-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster mysql-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster mysql-cluster
```

</TabItem>

</Tabs>
