---
title: KubeBlocks overview
description: KubeBlocks, kbcli, multicloud
keywords: [kubeblocks, overview, introduction]
sidebar_position: 1
---

# KubeBlocks overview

## Introduction

KubeBlocks is an open-source, cloud-native data infrastructure designed to help application developers and platform engineers manage database and analytical workloads on Kubernetes. It is cloud-neutral and supports multiple cloud service providers, offering a unified and declarative approach to increase productivity in DevOps practices.

The name KubeBlocks is derived from Kubernetes and LEGO blocks, which indicates that building database and analytical workloads on Kubernetes can be both productive and enjoyable, like playing with construction toys. KubeBlocks combines the large-scale production experiences of top cloud service providers with enhanced usability and stability.

## Why you need KubeBlocks

Kubernetes has become the de facto standard for container orchestration. It manages an ever-increasing number of stateless workloads with the scalability and availability provided by ReplicaSet and the rollout and rollback capabilities provided by Deployment. However, managing stateful workloads poses great challenges for Kubernetes. Although statefulSet provides stable persistent storage and unique network identifiers, these abilities are far from enough for complex stateful workloads.

To address these challenges, and solve the problem of complexity, KubeBlocks introduces ReplicationSet and ConsensusSet, with the following capabilities:

- Role-based update order reduces downtime caused by upgrading versions, scaling, and rebooting.
- Maintains the status of data replication and automatically repairs replication errors or delays.

## Key features

- Be compatible with AWS, GCP, Azure, and Alibaba Cloud.
- Supports MySQL, PostgreSQL, Redis, MongoDB, Kafka, and more.
- Provides production-level performance, resilience, scalability, and observability.
- Simplifies day-2 operations, such as upgrading, scaling, monitoring, backup, and restore.
- Contains a powerful and intuitive command line tool.
- Sets up a full-stack, production-ready data infrastructure in minutes.
