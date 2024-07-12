---
title: Introduction of Instance Template
description: Introduction of Instance Template
keywords: [instance template]
sidebar_position: 1
sidebar_label: Introduction of instance template
---

# Introduction of instance template

## What is an instance template

An *instance* serves as the fundamental unit in KubeBlocks, comprising a Pod along with several auxiliary objects. To simplify, you can initially think of it as a Pod, and henceforth, we'll consistently refer to it as an "Instance."

Starting from version 0.9, we're able to establish multiple instance templates for a particular component within a cluster. These instance templates include several fields such as Name, Replicas, Annotations, Labels, Env, Tolerations, NodeSelector, etc. These fields will ultimately override the corresponding ones in the default template (originating from ClusterDefinition and ComponentDefinition) to generate the final template for rendering the instance.

## Why do we the instance template

In KubeBlocks, a *Cluster* is composed of several *Components*, where each *Component* ultimately oversees multiple *Pods* and auxiliary objects.

Prior to version 0.9, these pods were rendered from a shared PodTemplate, as defined in either ClusterDefinition or ComponentDefinition. However, this design can’t meet the following demands:

 - For Clusters rendered from the same addon, setting separate scheduling configurations such as *NodeName*, *NodeSelector*, or *Tolerations*.
 - For Components rendered from the same addon, adding custom *Annotations*, *Labels*, or ENV to the Pods they manage.
 - For Pods managed by the same Component, configuring different *CPU*, *Memory*, and other *Resource Requests* and *Limits*.

With various similar requirements emerging, the Cluster API introduced the Instance Template feature from version 0.9 onwards to cater to these needs.
