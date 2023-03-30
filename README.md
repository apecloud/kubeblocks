# KubeBlocks

[![Build status](https://github.com/apecloud/kubeblocks/workflows/CICD-PUSH/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/cicd-push.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/apecloud/kubeblocks)](https://goreportcard.com/report/github.com/apecloud/kubeblocks)
[![Docker Pulls](https://img.shields.io/docker/pulls/apecloud/kubeblocks)](https://hub.docker.com/r/apecloud/kubeblocks)
[![codecov](https://codecov.io/gh/apecloud/kubeblocks/branch/main/graph/badge.svg?token=GEH4I1C80Y)](https://codecov.io/gh/apecloud/kubeblocks)
[![LICENSE](https://img.shields.io/github/license/apecloud/kubeblocks.svg?style=flat-square)](/LICENSE)
[![Releases](https://img.shields.io/github/release/apecloud/kubeblocks/all.svg?style=flat-square)](https://github.com/apecloud/kubeblocks/releases)
[![TODOs](https://img.shields.io/endpoint?url=https://api.tickgit.com/badge?repo=github.com/apecloud/kubeblocks)](https://www.tickgit.com/browse?repo=github.com/apecloud/kubeblocks)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/apecloud)](https://artifacthub.io/packages/search?repo=apecloud)


![image](./docs/img/banner_website_version.png)



- [KubeBlocks](#kubeblocks)
  - [What is KubeBlocks](#what-is-kubeblocks)
    - [Goals](#goals)
    - [Key Features](#key-features)
  - [Documents](#documents)
    - [Quick start with KubeBlocks](#quick-start-with-kubeblocks)
    - [Introduction](#introduction)
    - [Installation](#installation)
    - [User documents](#user-documents)
    - [Design proposal](#design-proposal)
  - [Community](#community)
  - [Contributing to KubeBlocks](#contributing-to-kubeblocks)
  - [License](#license)


## What is KubeBlocks
KubeBlocks is an open-source tool designed to help developers and platform engineers build and manage stateful workloads, such as databases and analytics, on Kubernetes. It is cloud-neutral and supports multiple public cloud providers, providing a unified and declarative approach to increase productivity in DevOps practices.

The name KubeBlocks is derived from Kubernetes and building blocks, which indicates that standardizing databases and analytics on Kubernetes can be both productive and enjoyable, like playing with construction toys. KubeBlocks combines the large-scale production experiences of top public cloud providers with enhanced usability and stability.

### Why you need KubeBlocks

Kubernetes has become the de facto standard for container orchestration. It manages an ever-increasing number of stateless workloads with the scalability and availability provided by ReplicaSet and the rollout and rollback capabilities provided by Deployment. However, managing stateful workloads poses great challenges for Kubernetes. Although statefulSet provides stable persistent storage and unique network identifiers, these abilities are far from enough for complex stateful workloads.

To address these challenges, and solve the problem of complexity, KubeBlocks introduces ReplicationSet and ConsensusSet, with the following capabilities:

- Role-based update order reduces downtime caused by upgrading versions, scaling, and rebooting.
- Latency-based election weight reduces the possibility of related workloads or components being located in different available zones.
- Maintains the status of data replication and automatically repairs replication errors or delays.

### Goals
- Enhance stateful applications control plane manageability on Kubernetes clusters, being open sourced and cloud neutral 
- Manage data platforms without a high cognitive load of cloud computing, Kubernetes, and database knowledge 
- Be community-driven, embracing extensibility, and providing domain functions without vendor lock-in
- Reduce costs by only paying for the infrastructure and increasing the utilization of resources with flexible scheduling
- Support the most popular databases, analytical software, and their bundled tools
- Provide the most advanced user experience based on the concepts of IaC and GitOps

### Key features
- Kubernetes-native and multi-cloud supported.
- Supports multiple database engines, including MySQL, PostgreSQL, Redis, MongoDB, and more.
- Provides production-level performance, resilience, scalability, and observability.
- Simplifies day-2 operations, such as upgrading, scaling, monitoring, backup, and restore.
- Declarative configuration is made simple, and imperative commands are made powerful.
- The learning curve is flat, and you are welcome to submit new issues on GitHub.


For detailed feature information, see [Feature list](https://github.com/apecloud/kubeblocks/blob/support/rewrite_kb_introduction/docs/user_docs/Introduction/feature_list.md)

## Documents
### Quick start with KubeBlocks
[Quick Start](docs/user_docs/quick_start_guide.md) shows you the quickest way to get started with KubeBlocks.
### Introduction
[Introduction](docs/user_docs/introduction/introduction.md) is a detailed information on KubeBlocks.
### Installation
[Installation](docs/user_docs/installation) document for install KubeBlocks, playground, kbctl, and create database clusters.
### User documents
[User documents](docs/user_docs) for instruction to use KubeBlocks.
### Design proposal
[Design proposal](docs/design_docs) for design motivation and methodology.

## Community
- KubeBlocks [Slack Channel](https://kubeblocks.slack.com/ssb/redirect)
- KubeBlocks Github [Discussions](https://github.com/apecloud/kubeblocks/discussions)
- Questions tagged [#KubeBlocks](https://stackoverflow.com/questions/tagged/KubeBlocks) on StackOverflow
- Follow us on Twitter [@KubeBlocks](https://twitter.com/KubeBlocks)
## Contributing to KubeBlocks
Your contributions and suggestions are welcomed and appreciated.
- See the [Contributing Guide](docs/CONTRIBUTING.md) for details on typical contribution workflows.
- See the [Development Guide](docs/DEVELOPING.md) to get started with building and developing.
- See the [Docs Contributing Guide](docs/CONTRIBUTING_DOCS.md) to get started with contributing to the KubeBlocks docs.

## License
KubeBlocks is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
