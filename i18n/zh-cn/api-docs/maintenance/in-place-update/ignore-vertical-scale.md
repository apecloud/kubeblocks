---
title: Enable in-place update
description: Enable in-place update
keywords: [in-place update]
sidebar_position: 2
sidebar_label: Enable in-place update
---

# Enable in-place update

Resources 的原地更新一直以来有比较强的需求，在低于 1.27 版本的 Kubernetes 中，我们可以看到很多 Kubernetes 的发行版中支持了 Resources 的原地更新能力，不同的发行版可能采用了不同的方案去实现这一特性。

为了兼容这些 Kubernetes 发行版，KubeBlocks 中增加了 `IgnorePodVerticalScaling` 特性开关。当该特性打开后，KubeBlocks 在做实例更新时，会忽略 Resources 中 CPU 和 Memory 的更新，从而使得最终渲染的 Pod 的 Resources 跟当前在运行 Pod 的 Resources 配置保持一致。\
