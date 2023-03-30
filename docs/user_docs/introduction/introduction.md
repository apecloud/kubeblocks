---
title: KubeBlocks overview
description: KubeBlocks, kbcli, multicloud
sidebar_position: 1
---

# KubeBlocks overview

## Introduction
KubeBlocks is an open-source tool designed to help developers and platform engineers build and manage stateful workloads, such as databases and analytics, on Kubernetes. It is cloud-neutral and supports multiple public cloud providers providing a unified and declarative approach to increase productivity in DevOps practices.

The name KubeBlocks is derived from Kubernetes and building blocks, which indicates that standardizing databases and analytics on Kubernetes can be both productive and enjoyable, like playing with construction toys. KubeBlocks combines the large-scale production experiences of top public cloud providers with enhanced usability and stability.

## Why You Need KubeBlocks

Kubernetes has become the de facto standard for container orchestration. It manages an ever-increasing number of stateless workloads with the scalability and availability provided by ReplicaSet and the rollout and rollback capabilities provided by Deployment. However, managing stateful workloads poses great challenges for Kubernetes. Although statefulSet provides stable persistent storage and unique network identifiers, these abilities are far from enough for complex stateful workloads.

To address these challenges, and solve the problem of complexity, KubeBlocks introduces ReplicationSet and ConsensusSet, with the following capabilities:

- Role-based update order reduces downtime caused by upgrading versions, scaling, and rebooting.
- Latency-based election weight reduces the possibility of related workloads or components being located in different available zones.
- Maintains the status of data replication and automatically repairs replication errors or delays.

## KubeBlocks Key Features

- Kubernetes-native and multi-cloud supported.
- Supports multiple database engines, including MySQL, PostgreSQL, Redis, MongoDB, and more.
- Provides production-level performance, resilience, scalability, and observability.
- Simplifies day-2 operations, such as upgrading, scaling, monitoring, backup, and restore.
- Declarative configuration is made simple, and imperative commands are made powerful.
- The learning curve is flat, and you are welcome to submit new issues on GitHub.