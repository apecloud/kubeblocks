---
title: KubeBlocks overview
description: KubeBlocks, kbcli, multicloud
sidebar_position: 1
---

# KubeBlocks overview

KubeBlocks is an open-source data management platform that greatly simplifies deployment and database operations. It has multiple built-in database engines and provides consistent management experience through `kbcli`, the command line tool of KubeBlocks. KubeBlocks runs on Kubernetes and supports multicloud environments. Any data product can access KubeBlocks in a declarative and configurable way to meet your needs to move to/off clouds and migrate across clouds. KubeBlocks can improve the utilization rate of your cloud resources and decrease the data computing and storage costs through shared instances and resource overcommitment.

The ApeCloud MySQL database cluster provided by KubeBlocks is fully compatible with MySQL syntax and supports single-availability zone deployment, double-availability zone deployment, and multiple-availability zone deployment. Based on the Paxos consensus protocol, the ApeCloud MySQL cluster realizes automatic leader election, log synchronization, and strict consistency. The ApeCloud MySQL cluster is the optimum choice for the production environment since it can automatically perform a high-availability switch to maintain business continuity when container exceptions, server exceptions, or availability zone exceptions occur.