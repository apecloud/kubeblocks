---
title: 删除集群
description: 如何删除集群
keywords: [kafka, 删除集群, 删除保护]
sidebar_position: 7
sidebar_label: 删除保护
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 删除集群

## 终止策略

:::note

终止策略决定了删除集群的方式，可在创建集群时进行设置。

:::

| **terminationPolicy**  | **删除操作**                    |
|:--                     | :--                                       |
| `DoNotTerminate`       | `DoNotTerminate` 禁止删除操作。 |
| `Halt`                 | `Halt` 删除工作负载资源（如 statefulset、deployment 等），但保留 PVC。 |
| `Delete`               | `Delete` 删除工作负载资源和 PVC，但保留备份。 |
| `WipeOut`              | `WipeOut` 删除工作负载资源、PVC 和所有相关资源（包括备份）。 |

执行以下命令查看终止策略。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster list kafka
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
kubectl -n default get cluster kafka
>
NAME    CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS     AGE
kafka      kafka                                kafka-3.3.2    Delete                               Running    19m
```

</TabItem>

</Tabs>

## 步骤

执行以下命令，删除集群。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster delete kafka
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

如果想删除集群和所有相关资源，可以将终止策略修改为 `WipeOut`，然后再删除该集群。

```bash
kubectl patch -n demo cluster kafka -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -n demo cluster kafka
```

</TabItem>

</Tabs>
