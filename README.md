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

KubeBlocks, running on Kubernetes, is an open source data manangement platforms. KubeBlocks greatly simplifies the process of deploying databases on Kubernetes. You can create and use database clusters within minutes without a deep knowledge of Kubernetes. It offers an universal view for multicloud and on-premises databases, provides a consistent experience, thus relieves the burden of maintaining miscellaneous operators.
***

### Why you use KubeBlocks
- Cloud-neutral
  - 
  - Runs on a Kubernetes base and supports AWS EKS, GCP GKE, Azure AKS and other cloud environments.
- Database as code
  - Defines each supported database engine through a declarative API, extending the Kubernetes statefulset to better adapt to stateful services as databases. 
  - Developers can use Kubernetes CLI or API to interact with KubeBlocks database clusters, integrate into DevOps tools and processes.
- Multiple database engine supported
  - Built-in MySQL, PostgreSQL, Redis, MongoDB and other database engines
  - You can also access and manage any new database engines or Plugins by defining CRD (Kubenetes custom resource definition)
- High availability
  - Officially provides a WeSQL high-availability database cluster that is fully compatible with MySQL, and supports single-availability zone deployment and three-availability zone deployment
  - Based on the consistency X-Paxos protocol, the WeSQL cluster realizes automatic master selection, log synchronization, and strong data consistency, and the cluster maintains high availability when a single availability zone failure.
  - Follows the MySQL Binary Log standard, compatible with commonly used Binlog incremental subscription tools.
- Life cycle management
  - Pause and resume the database cluster
  - Restart the cluster
  - Vertical allocation, change the CPU and memory configuration of cluster nodes
  - Horizontal scaling to increase read replicas
  - Disk expansion
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
- Cost
  - KubeBlocks is completely free and open source
  - You can independently choose VM instances with lower cost, such as Reserve instances with 1 to 3 years
  - Support over-allocation, allocate more database instances on one EC2, effectively reduce costs
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
***
## Documents

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





