# KubeBlocks

[![Documentation status](https://github.com/apecloud/kubeblocks.io/workflows/Documentation/badge.svg)](https://kubeblocks.io)
[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/7544/badge)](https://bestpractices.coreinfrastructure.org/projects/7544)
[![Releases](https://img.shields.io/github/v/release/apecloud/kubeblocks)](https://github.com/apecloud/kubeblocks/releases/latest)
[![LICENSE](https://img.shields.io/github/license/apecloud/kubeblocks.svg?style=flat-square)](/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apecloud/kubeblocks)](https://goreportcard.com/report/github.com/apecloud/kubeblocks)
[![Docker Pulls](https://img.shields.io/docker/pulls/apecloud/kubeblocks)](https://hub.docker.com/r/apecloud/kubeblocks)
[![codecov](https://codecov.io/gh/apecloud/kubeblocks/branch/main/graph/badge.svg?token=GEH4I1C80Y)](https://codecov.io/gh/apecloud/kubeblocks)
[![Build status](https://github.com/apecloud/kubeblocks/workflows/CICD-PUSH/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/cicd-push.yml)
![maturity](https://img.shields.io/static/v1?label=maturity&message=alpha&color=red)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kubeblocks)](https://artifacthub.io/packages/search?repo=kubeblocks)

![image](./docs/img/banner-readme.png)

- [KubeBlocks](#kubeblocks)
  - [What is KubeBlocks](#what-is-kubeblocks)
    - [Why you need KubeBlocks](#why-you-need-kubeblocks)
    - [Goals](#goals)
    - [Key features](#key-features)
  - [Get started with KubeBlocks](#get-started-with-kubeblocks)
  - [Community](#community)
  - [Contributing to KubeBlocks](#contributing-to-kubeblocks)
  - [Report Vulnerability](#report-vulnerability)
  - [License](#license)

## What is KubeBlocks

KubeBlocks is an open source system software that runs and manages data infrastructure on Kubernetes. It could manage various data/database engines, including RDBMSs (MySQL, PostgreSQL), Caches(Redis), NoSQLs (MongoDB), MQs(Kafka, Pulsar), and vector databases(Milvus, Qdrant, Weaviate), and the community is actively integrating more types of data/database engines into KuberBlocks. KubeBlocks helps developers, SREs, and platform engineers deploy and maintain dedicated DBPaaS in the enterprise, and supports a variety of public clouds and on-premise environments. 

KubeBlocks is designed for production purposes, providing secure, cost-effective, high-available and extensible DBPaaS solution for enterprise. The name KubeBlocks is inspired by Kubernetes and LEGO blocks, signifying that integrating data infrastructure on K8s can be standard and productive, like playing with LEGO blocks. The core of KubeBlocks is a K8s operator, which defines a set of CRDs to abstract the common attributes of various data/database engines, thereby supporting the rapid integration of a new data/database system into KubeBlocks while keeping the same operating experience and expected behaviors.

### Why you need KubeBlocks

KubeBlocks integrates the most popular data/database engines and provides rich management functions, along with declarative APIs, on various environments. Furthermore, KubeBlocks offers the following benefits:

* Cost-effective
  
  KubeBlocks optimizes the kernel and parameters of MySQL and PostgreSQL databases for containerized and cloud environments, and its performance exceeds that of RDS with the same hardware (see our blogs for test reports).

* Reliability
  
  KubeBlocks supports high-availability cluster configurations for multiple database engines, and supports Raft replication cluster for MySQL to achieve enhanced availability with durability guarantees.

* Observability
  
  KubeBlocks collects monitoring metrics from richful data sources, integrates with the Prometheus stack, and provides insightful Grafana templates. In addition, troubleshooting tools such as slow logs are also provided.

* Extensibility
  
  KubeBlocks provides the addon mechanism for integrating new data/database engines. So it can be extended to run the databases your project needs.

### Goals
- Being open and cloud-neutral
- Promoting the containerization of database workloads
- Promoting IaC and GitOps in the field of databases
- Reducing the cost of using databases
- Smoothing the learning curve of managing databases

### Key features

- Be compatible with AWS, GCP, Azure, and more
- Supports various data infrastructure systems, including MySQL, PostgreSQL, Redis, MongoDB, Kafka, Pulsar and more
- Provides production-level performance, resilience, scalability, and observability
- Simplifies day-2 operations, such as upgrading, scaling, monitoring, backup, and restore
- Contains a powerful and intuitive command line tool
  
## Get started with KubeBlocks

[Quick Start](https://kubeblocks.io/docs/preview/user_docs/try-out-on-playground/try-kubeblocks-on-your-laptop) shows you the quickest way to get started with KubeBlocks.

## Community

- KubeBlocks [Slack Channel](https://join.slack.com/t/kubeblocks/shared_invite/zt-23vym7xpx-Xu3xcE7HmcqGKvTX4U9yTg)
- KubeBlocks Github [Discussions](https://github.com/apecloud/kubeblocks/discussions)

## Contributing to KubeBlocks

Your contributions are welcomed and appreciated.

- See the [Contributor Guide](docs/CONTRIBUTING.md) for details on typical contribution workflows.
- See the [Developer Guide](docs/DEVELOPING.md) to get started with building and developing.

## Report Vulnerability

We consider security is a top priority issue. If you come across a related issue, please create a [Report a security vulnerability](https://github.com/apecloud/kubeblocks/security/advisories/new) issue.

## License

KubeBlocks is under the GNU Affero General Public License v3.0.
See the [LICENSE](./LICENSE) file for details.
