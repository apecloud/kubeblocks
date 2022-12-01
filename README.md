# KubeBlocks

[![Build status](https://github.com/apecloud/kubeblocks/workflows/CICD-PUSH/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/cicd-push.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/apecloud/kubeblocks)](https://goreportcard.com/report/github.com/apecloud/kubeblocks)
[![Docker Pulls](https://img.shields.io/docker/pulls/apecloud/kubeblocks)](https://hub.docker.com/r/apecloud/kubeblocks)
[![codecov](https://codecov.io/gh/apecloud/kubeblocks/branch/main/graph/badge.svg?token=GEH4I1C80Y)](https://codecov.io/gh/apecloud/kubeblocks)
[![LICENSE](https://img.shields.io/github/license/apecloud/kubeblocks.svg?style=flat-square)](/LICENSE)
[![Releases](https://img.shields.io/github/release/apecloud/kubeblocks/all.svg?style=flat-square)](https://github.com/apecloud/kubeblocks/releases)
[![TODOs](https://img.shields.io/endpoint?url=https://api.tickgit.com/badge?repo=github.com/apecloud/kubeblocks)](https://www.tickgit.com/browse?repo=github.com/apecloud/kubeblocks)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/apecloud)](https://artifacthub.io/packages/search?repo=apecloud)

![image](https://github.com/apecloud/kubeblocks/blob/support/rewrite_kb_introduction/docs/img/banner:-image-with:-ape-space.jpg)
## What is KubeBlocks
***
KubeBlocks, running on Kubernetes, is an open-source data manangement platform. KubeBlocks greatly simplifies the process of deploying databases on Kubernetes. It offers an universal view for multicloud and on-premises databases with a consistent developping and manangement experience.

### Goals
***
- Enable developers using one platform to manage any database engines used
- Provide consistency and easy-to-use declarative API
- Create and use database clusters within minutes without a deep knowledge of Kubernetes
- Relieve the burden of maintaining miscellaneous operators
- Be community driven, open and cloud neutral
- Embrace extensibility and provide domain functions without vendor lock-in
- Gain new contributors
### Key Features
***
- Cloud-neutral
  - Runs on a Kubernetes base and supports AWS EKS, GCP GKE, Azure AKS and other cloud environments
- Database as code
  - Defines each supported database engine through a declarative API, extending the Kubernetes statefulset to better adapt to stateful services as databases
  - Developers can use Kubernetes CLI or API to interact with KubeBlocks database clusters, integrate into DevOps tools and processes
- Multiple database engines compatible
  - Compatible wit MySQL, PostgreSQL, Redis, MongoDB and other database engines
  - You can also access and manage any new database engines or Plugins by defining CRD (Kubenetes custom resource definition)
- High availability
  - Provides a high-availability database WeSQL that is fully compatible with MySQL, and supports single-availability zone deployment and three-availability zone deployment
  - Based on the consistency X-Paxos protocol, WeSQL cluster realizes automatic master selection, log synchronization, and strong data consistency, and the cluster maintains high availability when a single availability zone failure
  - Follows MySQL Binary Log standard, compatible with commonly used Binlog incremental subscription tools
- Life cycle management
  - Pause and resume the database cluster
  - Restart the cluster
  - Vertical scaling, change the CPU and memory configuration of cluster nodes
  - Horizontal scaling to increase read replicas
  - Volume expansion
  - Database parameter modification
- Backup and restore
  - File backup and snapshot backup, realize minute-level snapshot backup through EBS
  - User-defined backup tools
  - Automatic backup and manual backup
  - Backup files are stored in object storage such as S3, GCS, etc., and you can specify the retention period and number of retention
  - Restores to new database cluster or restore to original cluster
- Monitoring and alarming
  - Built-in Prometheus, Grafana and AlertManager
  - Supports Prometheus exporter to output Metrics
  - Customized Grafana dashboard to view and monitor the dashboard
  - Support AlertManager to define alert rules and notifications
- Cost effective
  - KubeBlocks is completely free and open-source
  - Support resource overcommitment
- Safety
  - Role-based access control (RBAC)
  - Network transmission encryption
  - Data storage encryption
  - Backup encryption
- dbctl - Easy-to-use CLI command line tool
  - Install, uninstall and upgrade the system with dbctl
  - Support common operations such as database cluster, backup and recovery, monitoring, log, operation and maintenance, bench
  - Support dbctl to connect to the database cluster without repeatedly entering password
  - Support command line automatic completion

## Documents
### Quick start with KubeBlocks
待完成
### Introduction
[Introduction](https://github.com/apecloud/kubeblocks/tree/main/docs/user_docs/Introduction) is a detailed information on KubeBlocks.
### Release notes
[release_notes](https://github.com/apecloud/kubeblocks/tree/main/docs/release_notes) for the latest updates.
### Installation
[Installation](https://github.com/apecloud/kubeblocks/tree/main/docs/user_docs/installation) document for install KubeBlocks, playground, dbctl, and create clusters.
### User doc
[user_docs](https://github.com/apecloud/kubeblocks/tree/main/docs/user_docs) for instruction to use KubeBlocks.
### Design proposal
[design_docs](https://github.com/apecloud/kubeblocks/tree/main/docs/design_docs) for design motivation and methodology.

## Community


- [Slack Channel](https://kubeblocks.slack.com/ssb/redirect)
- Follow us on Twitter @ApeCloud
- Questions tagged #KubeBlocks on StackOverflow
- When need help, contact xxxx

## Contributing to KubeBlocks
Your contributions and suggestions are welcomed and appreciated.
- Guidelines for contributing -需要做how to 文档 内容包括如何report a issue 如何contribute code 如何review Pull Request 如何写文档 coding风格指南 等等
- 

## Code of Conduct






