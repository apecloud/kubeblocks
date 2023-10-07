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

KubeBlocks is an open-source Kubernetes operator that manages relational, NoSQL, vector, and streaming databases on the public cloud or on-premise. It is designed for production purposes, providing reliable, performant, observable, and cost-effective data infrastructure for most scenarios. The name KubeBlocks is inspired by Kubernetes and LEGO blocks, signifying that building data infrastructure on Kubernetes can be both productive and enjoyable, like playing with construction toys.

### Why you need KubeBlocks

When adopting a multi-cloud or hybrid cloud strategy, it is essential to prioritize application portability and use software or services that offer consistent functionality across different infrastructures. Kubernetes has helped in achieving this goal to some extent, and KubeBlocks can further enhance the experience. KubeBlocks integrates popular database engines and provides rich management functions, along with declarative APIs, on various infrastructures. Furthermore, KubeBlocks offers the following benefits:

* Performance
  
  You do not have to learn database tuning. KubeBlocks can leverage storage and computing resources to achieve optimal database performance.

* Reliability
  
  You do not need to worry about data loss or service outages. KubeBlocks provides fault tolerance at the node or availability zone levels, which maximizes database reliability.

* Observability
  
  You can track new metrics under one roof. KubeBlocks integrates observability platforms such as Prometheus stack, AWS AMP, Aliyun ARMS, etc., and provides beautiful templates with insights.

* Extensibility
  
  You can integrate and use new database engines with ease. KubeBlocks provides a good abstraction of the database control plane, allowing for efficient support of new database engines with a consistent user experience.

### Goals
- Being open and cloud-neutral
- Promoting the containerization of database workloads
- Promoting IaC and GitOps in the field of databases
- Reducing the cost of using databases
- Smoothing the learning curve of managing databases

### Key features

- Be compatible with AWS, GCP, Azure, and more
- Supports MySQL, PostgreSQL, Redis, MongoDB, Kafka, and more
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
