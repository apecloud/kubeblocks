---
title: Enable in-place update
description: Enable in-place update
keywords: [in-place update]
sidebar_position: 2
sidebar_label: Enable in-place update
---

# Enable in-place update

In Kubernetes versions below 1.27, we have seen support for in-place updates of Resources in many Kubernetes distributions. Different distributions may adopt different approaches to implement this feature.

To accommodate these Kubernetes distributions, KubeBlocks has introduced the `IgnorePodVerticalScaling` feature switch. When this feature is enabled, KubeBlocks ignores updates to CPU and Memory in Resources during instance updates, ensuring that the Resources configuration of the final rendered Pod remains consistent with the Resources configuration of the currently running Pod.
