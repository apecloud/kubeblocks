---
title: KubeBlocks overview
description: KubeBlocks, kbcli, multicloud
sidebar_position: 1
---

# KubeBlocks overview

## Introduction

KubeBlocks Open-source and cloud-prem tool that helps application developers and platform engineers build and manage Kubernetes native data platforms with the most popular databases, analytical software, and their bundled tools.
 It has multiple built-in database engines and provides consistent management experience through `kbcli`, the command line tool of KubeBlocks. KubeBlocks runs on Kubernetes and supports multicloud environments. Any data product can access KubeBlocks in a declarative and configurable way to meet your needs to move to/off clouds and migrate across clouds. KubeBlocks can improve the utilization rate of your cloud resources and decrease the data computing and storage costs through shared instances and resource overcommitment.

## Feature list

* Kubernetes native and multi-cloud supported
* Provisioning a database cluster within several minutes without the knowledge of Kubernetes
* Support multiple database engines including MySQL, PostgreSQL, Redis and Apache Kafka
* Provides high-availability database clusters with single-availability zone deployment, double-availability zone deployment, or three-availability zone deployment.
* Automatic operation and maintenance include vertical scaling, horizontal scaling, volume expansion and restarting clusters
* Snapshot backup at minute-level through EBS
* Scheduled backup
* Point in time restore
* Built-in Prometheus, Grafana, and AlertManager
* Resource overcommitment to allocate more database instances on one EC2 to decrease costs efficiently
* Role-based access control (RBAC)
* `kbcli`, an easy-to-use CLI, supports common operations such as database cluster creating, connecting, backup, restore, monitoring and trouble shooting.
* Support customized robot alarms for Slack, Feishu, Wechat and DingTalks.  