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

| **terminationPolicy** | **删除操作**                                                                     |
|:----------------------|:-------------------------------------------------------------------------------------------|
| `DoNotTerminate`      | `DoNotTerminate` 禁止删除操作。                                                  |
| `Halt`                | `Halt` 删除工作负载资源（如 statefulset、deployment 等），但保留 PVC。 |
| `Delete`              | `Delete` 删除工作负载资源和 PVC，但保留备份。                              |
| `WipeOut`             | `WipeOut`  删除工作负载资源、PVC 和所有相关资源（包括备份）。    |

执行以下命令查看终止策略。

```bash
kbcli cluster list pg-cluster
>
NAME         NAMESPACE   CLUSTER-DEFINITION   VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
pg-cluster   default     postgresql           postgresql-14.7.0   Delete               Running   Mar 03,2023 18:49 UTC+0800
```

## 步骤

执行以下命令，删除集群。

```bash
kbcli cluster delete pg-cluster
```
