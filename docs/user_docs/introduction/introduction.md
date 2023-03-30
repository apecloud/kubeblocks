---
title: KubeBlocks overview
description: KubeBlocks, kbcli, multicloud
sidebar_position: 1
---

# KubeBlocks overview

## Introduction
KubeBlocks is an open-source tool designed to help developers and platform engineers build and manage stateful workloads, such as databases and analytics, on Kubernetes. It is cloud-neutral and supports multiple public cloud providers. KubeBlocks provides a unified and declarative approach to increase productivity in DevOps practices.

The name KubeBlocks is derived from Kubernetes and building blocks, which indicates that standardizing databases and analytics on Kubernetes can be both enjoyable and productive, like playing with construction toys. KubeBlocks combines the large-scale production experiences of top public cloud providers with enhanced usability and stability.

## Why You Need KubeBlocks

Kubernetes has become the de facto standard for container orchestration. It manages an ever-increasing number of stateless workloads with the scalability and availability provided by ReplicaSet and the rollout and rollback capabilities provided by Deployment. However, managing stateful workloads poses great challenges for Kubernetes. Although statefulSet provides stable persistent storage and unique network identifiers, these abilities are far from enough for complex stateful workloads.

To address these challenges, and solve the problem of complexity, KubeBlocks introduces ReplicationSet and ConsensusSet, with the following capabilities:

- Role-based update order reduces downtime caused by upgrading versions, scaling, and rebooting.
- Latency-based election weight reduces the possibility of related workloads or components being located in different available zones.
- Maintains the status of data replication and automatically repairs replication errors or delays.

## Feature list

* Kubernetes native and multi-cloud supported
* Provisioning a database cluster within several minutes without the knowledge of Kubernetes
* Supports multiple database engines including MySQL, PostgreSQL, Redis and Apache Kafka
* Provides high-availability database clusters with single-availability zone deployment, double-availability zone deployment, or three-availability zone deployment.
* Automatic operation and maintenance, including vertical scaling, horizontal scaling, volume expansion and restarting clusters
* Snapshot backup at minute-level through EBS
* Scheduled backup
* Point in time restore
* Built-in Prometheus, Grafana, and AlertManager
* Resource overcommitment to allocate more database instances on one EC2 to decrease costs efficiently
* Role-based access control (RBAC)
* `kbcli`, an easy-to-use CLI, supports common operations such as database cluster creating, connecting, backup, restore, monitoring and trouble shooting.
* Supports custom robot alerts for Slack, Feishu, Wechat and DingTalks.  
