---
title: 卸载 KubeBlocks
description: 卸载 KubeBlocks
keywords: [kubeblocks, 卸载]
sidebar_position: 5
sidebar_label: 卸载 KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Uninstall KubeBlocks

卸载顺序：

1. 如果已经创建了集群，请先删除集群。

   ```bash
   kubebctl delete cluster <clustername> -n namespace
   ```

2. 卸载 KubeBlocks。

## 卸载 KubeBlocks

<Tabs>

<TabItem value="Helm" label="Helm" default>

在执行以下命令前，请删除之前创建的所有集群和资源，否则卸载可能无法成功。

```bash
helm uninstall kubeblocks --namespace kb-system
```

Helm 不会删除 CRD 对象。请使用以下命令删除 KubeBlocks 创建的对象。

```bash
kubectl get crd -o name | grep kubeblocks.io | xargs kubectl delete
```

</TabItem>

<TabItem value="YAML" label="YAML">

从 KubeBlocks chart 生成 YAML 文件，并使用 `kubectl` 进行卸载。

```bash
helm template kubeblocks kubeblocks/kubeblocks --namespace kb-system | kubectl delete -f -
```

</TabItem>

</Tabs>
