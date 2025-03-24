

[![Documentation status](https://github.com/apecloud/kubeblocks.io/workflows/Documentation/badge.svg)](https://kubeblocks.io)
[![OpenSSF Best Practices](https://bestpractices.coreinfrastructure.org/projects/7544/badge)](https://bestpractices.coreinfrastructure.org/projects/7544)
[![CICD Push](https://github.com/apecloud/kubeblocks/workflows/CICD-PUSH/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/cicd-push.yml)
[![CodeQL](https://github.com/apecloud/kubeblocks/workflows/CodeQL/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/codeql.yml)
[![Releases](https://github.com/apecloud/kubeblocks/actions/workflows/release-version.yml/badge.svg)](https://github.com/apecloud/kubeblocks/actions/workflows/release-version.yml)
[![Release](https://img.shields.io/github/v/release/apecloud/kubeblocks)](https://github.com/apecloud/kubeblocks/releases/latest)
[![LICENSE](https://img.shields.io/github/license/apecloud/kubeblocks.svg?style=flat-square)](/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apecloud/kubeblocks)](https://goreportcard.com/report/github.com/apecloud/kubeblocks)
[![Docker Pulls](https://img.shields.io/docker/pulls/apecloud/kubeblocks)](https://hub.docker.com/r/apecloud/kubeblocks)
[![Codecov](https://codecov.io/gh/apecloud/kubeblocks/branch/main/graph/badge.svg?token=GEH4I1C80Y)](https://codecov.io/gh/apecloud/kubeblocks)
![maturity](https://img.shields.io/static/v1?label=maturity&message=alpha&color=red)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/kubeblocks)](https://artifacthub.io/packages/search?repo=kubeblocks)

![image](./docs/img/banner-readme.jpeg)
# Welcome to the KubeBlocks project!

## Motivation

If you are a developer using multiple types of databases in your application and are considering deploying both your application and databases on K8s for cost or efficiency reasons, you need to find suitable operators for each database. Learning so many different operators and their APIs introduces a significant learning curve and time costs, not to mention the effort required to maintain them.

KubeBlocks uses a unified set of APIs (CRDs) and code to manage various databases on K8s. For example, we can use the `Cluster` resource to create a PostgreSQL cluster, a Redis cluster, or a Kafka cluster. This abstraction and unified API allow us to further use a single set of operator code to manage multiple types of databases, as well as handle day-2 operations, theoretically extending to any type of database engine.

## What is KubeBlocks

KubeBlocks is an open-source control plane software that runs and manages multiple popular database engines on K8s through a unified set of code and APIs. The core of KubeBlocks is a K8s operator, which defines a set of CRDs to abstract the common attributes of various database engines and uses these abstractions to manage the engine's lifecycle and day-2 operations.

KubeBlocks manages various types of stateful engines, including RDBMSs (MySQL, PostgreSQL), Caches(Redis), NoSQLs (MongoDB), MQs(Kafka, Pulsar), vector databases(Milvus, Qdrant, Weaviate), and data warehouses(ClickHouse, ElasticSearch, OpenSearch, Doris, StarRocks). Adding a new engine to KubeBlocks can be achieved by writing a KubeBlocks Addon. The community is actively integrating more types of engines into KubeBlocks, and it currently supports 35 types of engines.

The name KubeBlocks is inspired by Kubernetes and LEGO blocks, signifying that through the KubeBlocks API, adding, composing and managing database engines on K8s can be easy, standard and productive, like playing with LEGO blocks.

### Why you need KubeBlocks

KubeBlocks integrates the most popular database engines and provides rich management functions, along with declarative APIs, in various environments. KubeBlocks offers the following benefits:

* Production-level

  KubeBlocks has already been adopted by large internet companies, private clouds, the financial industry including banks and securities firms, telecom industry, the automotive industry, and SaaS software providers.

* Reliability

  KubeBlocks supports the integration of various mature high-availability best practices, such as Orchestrator, Patroni, and Sentinel. KubeBlocks also supports full backups, continuous backups, and point-in-time recovery (PITR).

* Ease of use

  KubeBlocks not only provides a YAML-based API but also offers an interactive `kbcli` tool to further simplify usage as a complement to `kubectl`. For example, you can install KubeBlocks and launch a playground environment on a desktop or cloud with a single command.

* Observability

  KubeBlocks collects monitoring metrics from rich data sources, integrates with the Prometheus stack, and provides insightful Grafana templates. In addition, troubleshooting tools such as slow logs are also provided.

* Extensibility

  KubeBlocks provides the addon mechanism for integrating new engines. So it can be extended to run the databases your project needs.

### Goals

- Smoothing the learning curve of managing various databases on K8s
- Exploring standard APIs for managing databases on Kubernetes
- Being open and cloud-neutral, as well as engine-neutral

### Key features

- Supports various databases, including MySQL, PostgreSQL, Redis, MongoDB, Kafka, Clickhouse, ElasticSearch and more
- Provides production-level performance, resilience, and observability
- Simplifies day-2 operations, such as upgrading, scaling, monitoring, backup, and restore
- Contains a powerful and intuitive command line tool
- Be compatible with AWS, GCP, Azure, Alibaba Cloud and more CSP

## Get started with KubeBlocks

[Quick Start](https://kubeblocks.io/docs/preview/user_docs/try-out-on-playground/try-kubeblocks-on-your-laptop) shows you the quickest way to get started with KubeBlocks.

## Resources

[API Reference](https://kubeblocks.io/docs/release-0.8/developer_docs/api-reference/cluster)

[How to write a KubeBlocks Addon?](https://kubeblocks.io/docs/release-0.8/developer_docs/integration/how-to-add-an-add-on)

[KubeBlocks: Cloud-Native Data Infrastructure for Kubernetes](https://www.youtube.com/watch?v=KNwpG51Whzg) (A Video made by Viktor Farcic)

[Dashboard Demo](https://console.kubeblocks.io/)

## KubeBlocks at KubeCon

KubeCon 2024 in HongKong from 21-23 August 2024: [How to Manage Database Clusters Without a Dedicated Operator, By Shanshan Ying, ApeCloud & Shun Ding, China Mobile Cloud](https://kccncossaidevchn2024.sched.com/event/1eYYL/how-to-manage-database-clusters-without-a-dedicated-operator-nanoxi-operatorzha-fa-lia-zhong-shi-shanshan-ying-apecloud-shun-ding-china-mobile-cloud)

KubeCon 2024 in HongKong from 21-23 August 2024: [KuaiShou's 100% Resource Utilization Boost: 100K Redis Migration from Bare Metal to Kubernetes, By XueQiang Wu, ApeCloud & YuXing Liu, Kuaishou](https://kccncossaidevchn2024.sched.com/event/1eYat/kuaishous-100-resource-utilization-boost-100k-redis-migration-from-bare-metal-to-kubernetes-zha-100pian-zhi-yi-daeplie-hui-zhe-100k-rediskubernetes-xueqiang-wu-apecloud-yuxing-liu-kuaishou)

## Community

If you have any questions, you can reach out to us through:

- KubeBlocks [Slack Channel](https://join.slack.com/t/kubeblocks/shared_invite/zt-2pjob3ezp-FzaZM7NId~Tbzp6PYNbOzQ)
- KubeBlocks Github [Discussions](https://github.com/apecloud/kubeblocks/discussions)
- KubeBlocks Wechat Account:

   <img src=".\docs\img\wechat-assistant.jpg" alt="wechat" width="100" height="100">

You can also follow us on:

- [Twitter](https://x.com/KubeBlocks)
- [LinkedIn](https://www.linkedin.com/company/apecloud-ptd-ltd/)

## Contributing to KubeBlocks

Your contributions are welcomed and appreciated.

- See the [Contributor Guide](https://github.com/apecloud/kubeblocks/blob/main/docs/CONTRIBUTING.md) for details on typical contribution workflows.
- See the [Developer Guide](https://github.com/apecloud/kubeblocks/blob/main/docs/00%20-%20index.md) to get started with building and developing.

## Report Vulnerability

We consider security as the top priority issue. If you find any security issues, please [Report a security vulnerability](https://github.com/apecloud/kubeblocks/security/advisories/new) issue.

## License

KubeBlocks is under the GNU Affero General Public License v3.0.
See the [LICENSE](https://github.com/apecloud/kubeblocks/blob/main/LICENSE) file for details.