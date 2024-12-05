---
title: FAQ
description: 升级, faq
keywords: [升级, FAQ, KubeBlocks, 升级指南]
sidebar_position: 4
sidebar_label: FAQ
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# FAQ

本文档罗列了 KubeBlocks 升级中常见的问题及解决方法。

## 手动标记引擎

KubeBlocks 早期版本在 Helm chart 中预装了一些引擎，但在新版本中，部分预装引擎可能被移除。此时，如果基于早期版本直接升级至最新版本，这些被移除引擎的 CR 也会被 Helm 移除，这些引擎创建的数据库集群也将受到影响。因此，升级 KubeBlocks 时，可通过为引擎添加 `"helm.sh/resource-policy": "keep"` 注解，确保相关引擎在升级过程中不会被移除。

### 查看引擎注解

执行以下命令，查看引擎注解。

```bash
kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
```

### 手动为引擎添加注解

可以将以下命令中的 `-l app.kubernetes.io/name=kubeblocks` 替换为所需的筛选项。

```bash
kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
```

如果想为某个引擎单独添加注解，可执行以下命令，替换 `{addonName}` 为需要的引擎。

```bash
kubectl annotate addons.extensions.kubeblocks.io {addonName} helm.sh/resource-policy=keep
```

如果想查看某个引擎是否成功添加了注解，可以执行以下命令，将 `{addonName}` 替换为需要的引擎。

```bash
kubectl get addon {addonName} -o json | jq '{name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
```

## 解决 "cannot patch 'kubeblocks-dataprotection' with kind Deployment" 问题

升级到 KubeBlocks v0.8.x/v0.9.0时，可能会出现以下报错：

```bash
Error: UPGRADE FAILED: cannot patch "kubeblocks-dataprotection" with kind Deployment: Deployment.apps "kubeblocks-dataprotection" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"dataprotection", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable && cannot patch "kubeblocks" with kind Deployment: Deployment.apps "kubeblocks" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"apps", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable
```

这是因为 KubeBlocks v0.9.1 修改了 KubeBlocks 和 KubeBlocks-Dataprotection 的标签。

如果出现这种错误，可以先手动删除 `kubeblocks` 和 `kubeblocks-dataprotection` 这两个 deployment，然后再执行 `helm upgrade` 升级到 KubeBlocks v0.9.1。

```bash
# 水平缩容至 0 replica
kubectl -n kb-system scale deployment kubeblocks --replicas 0
kubectl -n kb-system scale deployment kubeblocks-dataprotection --replicas 0

# 删除 deployments
kubectl delete -n kb-system deployments.apps kubeblocks kubeblocks-dataprotection
```

## 升级时如何指定镜像仓库

从 v0.9.0 开始，KubeBlocks 使用的镜像仓库中，有一个仓库地址发生了变化，该仓库的前缀从 `infracreate-registry` 更新为 `apecloud-registry`。其他镜像仓库不受影响。建议安装了 KubeBlocks v0.8.0 及之前版本的用户在升级到 v0.9.x 之前，先检查当前的镜像仓库配置，并在升级时指定新的镜像仓库地址。

1. 检查 KubeBlocks 使用的镜像仓库。

   ```bash
   helm -n kb-system get values kubeblocks -a | yq .image.registry
   ```

   如果输出的镜像仓库是以 `infracreate-registry` 开头（如下所示），您需要在升级 KubeBlocks 时指定新的镜像仓库，将前缀改为 `apecloud-registry`。

   <details>

   <summary>Output</summary>

   ```text
   infracreate-registry.cn-xxx.xxx.com
   ```

   </details>

2. 升级 KubeBlocks 时，可以通过以下参数指定修改默认镜像。

   <Tabs>

   <TabItem value="Helm" label="Helm" default>

   ```bash
   helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 \
     --set admissionWebhooks.enabled=true \
     --set admissionWebhooks.ignoreReplicasCheck=true \
     --set image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli kb upgrade --version 0.9.1 \ 
     --set admissionWebhooks.enabled=true \
     --set admissionWebhooks.ignoreReplicasCheck=true \
     --set image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com
   ```

   </TabItem>

   </Tabs>

   以下为上述命令的参数说明：

   - `--set image.registry=apecloud-registry.cn-xxx.xxx.com` 设置 KubeBlocks 镜像仓库。
   - `--set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com` 设置 KubeBlocks-Dataprotection 镜像仓库。
   - `--set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com` 设置 addon Charts 镜像仓库。
