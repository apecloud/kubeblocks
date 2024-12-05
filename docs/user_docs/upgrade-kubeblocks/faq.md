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

In earlier versions, KubeBlocks pre-installed some Addons in the Helm chart, but some of these Addons have been removed in the new version. Consequently, if you upgrade directly from an older version to the latest, Helm will remove the CRs of these removed Addons, affecting the clusters created by these Addons. To prevent this, it is recommended to add the `"helm.sh/resource-policy": "keep"` annotation for Addons to ensure they remain during upgrading.

### View the Addon annotation

Run the command below to view the annotations of Addons.

```bash
kubectl get addon -o json | jq '.items[] | {name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
```

### Manually add an annotation for Addons

Replace `-l app.kubernetes.io/name=kubeblocks` with your actual filter and run the command below to add an annotation.

```bash
kubectl annotate addons.extensions.kubeblocks.io -l app.kubernetes.io/name=kubeblocks helm.sh/resource-policy=keep
```

If you want to add the annotation for a specified Addon, replace `{addonName}` with the actual Addon name and run the command below.

```bash
kubectl annotate addons.extensions.kubeblocks.io {addonName} helm.sh/resource-policy=keep
```

If you want to check whether the annotation for an Addon was added successfully, replace `{addonName}` with the actual Addon name and run the command below.

```bash
kubectl get addon {addonName} -o json | jq '{name: .metadata.name, resource_policy: .metadata.annotations["helm.sh/resource-policy"]}'
```

## Fix "cannot patch 'kubeblocks-dataprotection' with kind Deployment" error

When upgrading KubeBlocks to v0.8.x/v0.9.x, you might encounter the following error:

```bash
Error: UPGRADE FAILED: cannot patch "kubeblocks-dataprotection" with kind Deployment: Deployment.apps "kubeblocks-dataprotection" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"dataprotection", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable && cannot patch "kubeblocks" with kind Deployment: Deployment.apps "kubeblocks" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/component":"apps", "app.kubernetes.io/instance":"kubeblocks", "app.kubernetes.io/name":"kubeblocks"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable
```

This error occurs due to label modifications introduced for KubeBlocks and KubeBlocks-Dataprotection in KubeBlocks v0.9.x.

To resolve the issue, manually delete the `kubeblocks` and `kubeblocks-dataprotection` deployments, then run helm upgrade to complete the upgrade to v0.9.x.

```bash
# Scale to 0 replica
kubectl -n kb-system scale deployment kubeblocks --replicas 0
kubectl -n kb-system scale deployment kubeblocks-dataprotection --replicas 0

# Delete deployments
kubectl delete -n kb-system deployments.apps kubeblocks kubeblocks-dataprotection
```

## Specify an image registry during upgrading KubeBlocks

Starting from v0.9.0, one of KubeBlocks' image registries has changed. Specifically, the registry prefix for one of the repositories has been updated from `infracreate-registry` to `apecloud-registry`. Other image registries remain unaffected. If you installed KubeBlocks before v0.9.0, check and update your image registry configuration during the upgrade.

1. Check the image registry of KubeBlocks.

   ```bash
   helm -n kb-system get values kubeblocks -a | yq .image.registry
   ```

   If the image registry starts with `infracreate-registry` as shown below, you must specify the new image registry during the upgrade by changing the image registry prefix to `apecloud-registry`.

   <details>

   <summary>Output</summary>

   ```text
   infracreate-registry.cn-xxx.xxx.com
   ```

   </details>

2. Override the default image registry by specifying the following parameters.

   <Tabs>

   <TabItem value="Helm" label="Helm" default>

   ```bash
   helm -n kb-system upgrade kubeblocks kubeblocks/kubeblocks --version 0.9.2 \
     --set admissionWebhooks.enabled=true \
     --set admissionWebhooks.ignoreReplicasCheck=true \
     --set image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com
   ```

   </TabItem>

   <TabItem value="kbcli" label="kbcli">

   ```bash
   kbcli kb upgrade --version 0.9.2 \ 
     --set admissionWebhooks.enabled=true \
     --set admissionWebhooks.ignoreReplicasCheck=true \
     --set image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com \
     --set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com
   ```

   </TabItem>

   </Tabs>

   Here is an introduction to the flags in the above command.

   - `--set image.registry=apecloud-registry.cn-xxx.xxx.com` specifies the KubeBlocks image registry.
   - `--set dataProtection.image.registry=apecloud-registry.cn-xxx.xxx.com` specifies the KubeBlocks-Dataprotection image registry.
   - `--set addonChartsImage.registry=apecloud-registry.cn-xxx.xxx.com` specifies Addon Charts image registry.
