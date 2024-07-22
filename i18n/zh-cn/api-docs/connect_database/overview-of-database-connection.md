---
title: 连接数据库
description: 如何连接数据库
keywords: [连接数据库]
sidebar_position: 1
sidebar_label: 简介
---

# 简介

部署完 KubeBlocks 并创建集群之后，数据库在 Kubernetes 上以 Pod 的形式运行。你可以通过客户端界面或 `kubectl` 连接到数据库。

如下图所示，首先你需要明确连接数据库的目的是什么。

- 如果你想试用 KubeBlocks、测试数据库功能或进行低流量基准测试，请参阅[在测试环境中连接数据库](./connect-database-in-testing-environment.md)。
- 如果你想在生产环境中连接数据库或进行高流量压力测试，请参阅[在生产环境中连接数据库](./connect-database-in-production-environment.md)。

![Connect database](./../../img/create-and-connect-databases-overview.jpg)
