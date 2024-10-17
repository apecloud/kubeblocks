---
title: FAQ
description: Upgrade, faq, tips and notes
keywords: [upgrade, FAQ]
sidebar_position: 3
sidebar_label: FAQ
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# FAQ

This guide addresses common questions and issues that may arise when upgrading KubeBlocks, ensuring a smooth and efficient process.

## Manually mark Addons

To prevent an Addon from being deleted during a KubeBlocks upgrade via `kbcli` or Helm, add the `"helm.sh/resource-policy": "keep"` annotation.

### View the Addon annotation

Run the command below to view the annotations of an Addon.

```bash
kubectl get addon -o json | jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}'
```

### Manually add an annotation for an Addon

Replace `-l app.kubernetes.io/name=kubeblocks` with your acutal filter and run the command below to add an annotation.

```bash
kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
```

If you want to add the annotation for a specified Addon, replace `{addonName}` with the actual Addon name and run the command below.

```bash
kubectl annotate http://addons.extensions.kubeblocks.io {addonName} helm.sh/resource-policy=keep
```

If you want to check whether the annotation for an Addon was added succeddfully, replace `{addonName}` with the actual Addon name and run the command below.

```bash
kubectl get addon {addonName} -o json | jq '{name: .metadata.name, annotations: .metadata.annotations}'
```

## Fix "cannot patch 'kubeblocks-dataprotection' with kind Deployment" error

When upgrading KubeBlocks to v0.8.x/v0.9.0, you might encounter the following error:

```bash
Error: UPGRADE FAILED: cannot patch "kubeblocks-dataprotection" with kind Deployment: Deployment.apps "kubeblocks-dataprotection" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"dataprotection", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable && cannot patch "kubeblocks" with kind Deployment: Deployment.apps "kubeblocks" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"apps", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable
```

This is due to label modifications of KubeBlocks and KubeBlocks-Dataprotection in KubeBlocks v0.9.1.

To resolve the issue, manually delete the `kubeblocks` and `kubeblocks-dataprotection` deployments, then run helm upgrade to complete the upgrade to v0.9.1.

```bash
# scale to 0 replica
kubectl -n kb-system scale deployment kubeblocks --replicas 0
kubectl -n kb-system scale deployment kubeblocks-dataprotection --replicas 0

# delete deployments
kubectl delete -n kb-system deployments.apps kubeblocks kubeblocks-dataprotection
```

## Specify an image registry during KubeBlocks upgrade

KubeBlocks v0.8.x uses `infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com` and `docker.io` as image registries and KubeBlocks v0.9.x uses `apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com` and `docker.io`.

When upgrading KubeBlocks, you can override the default image registry by specifying the following parameters.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli kb upgrade --version 0.9.1 \ 
--set admissionWebhooks.enabled=true \
--set admissionWebhooks.ignoreReplicasCheck=true \
--set image.registry=docker.io \  # Set KubeBlocks image registry
--set dataProtection.image.registry=docker.io \ # Set KubeBlocks-Dataprotection image registry
--set addonChartsImage.registry=docker.io # Set Addon Charts image registry
```

</TabItem>

<TabItem value="Helm" label="Helm">

```bash
helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.1 \
--set admissionWebhooks.enabled=true \
--set admissionWebhooks.ignoreReplicasCheck=true \
--set image.registry=docker.io \  # Set KubeBlocks image registry
--set dataProtection.image.registry=docker.io \ # Set KubeBlocks-Dataprotection image registry
--set addonChartsImage.registry=docker.io # Set Addon Charts image registry
```

</TabItem>

</Tabs>
