---
title: 简介
description: 实例模板简介
keywords: [实例模板]
sidebar_position: 1
sidebar_label: 简介
---

# 简介

## 什么是实例模板

***实例（Instance）***是 KubeBlocks 中的基本单元，它由一个 Pod 和若干其它辅助对象组成。为了容易理解，你可以先把它简化为一个 Pod，下文中将统一使用“实例”这个名字。

从 0.9 开始，我们可以为一个 Cluster 中的某个 Component 设置若干实例模板（Instance Template），实例模板中包含 Name、Replicas、Annotations、Labels、Env、Tolerations、NodeSelector 等多个字段（Field），这些字段最终会覆盖（Override）默认模板（也就是在 ClusterDefinition 和 ComponentDefinition 中定义的 PodTemplate）中相应的字段，并生成最终的模板以便用来渲染实例。

## 为什么采用实例模板

在 KubeBlocks 中，一个 *Cluster* 由若干个 *Component* 组成，一个 *Component* 最终管理若干 *Pod* 和其它对象。

在 0.9 版本之前，这些 Pod 是从同一个 PodTemplate 渲染出来的（该 PodTemplate 在 ClusterDefinition 或 ComponentDefinition 中定义）。这样的设计不能满足如下需求：

 - 对于从同一个引擎中渲染出来的 Cluster，为其设置单独的 *NodeName*、*NodeSelector* 或 *Tolerations* 等调度相关配置。
 - 对于从同一个引擎中渲染出来的 Component，为它所管理的 Pod 添加自定义 *Annotation*、*Label* 或 *ENV*
 - 对于被同一个 Component 管理的 Pod，为它们配置不同的 *CPU*、*Memory* 等 *Resources Requests* 和 *Limits*

类似的需求还有很多，所以从 0.9 版本开始，Cluster API 中增加了实例模板特性，以满足上述需求。
