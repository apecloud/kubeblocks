---
title: 连接数据库
description: 如何连接数据库
keywords: [连接数据库]
sidebar_position: 1
sidebar_label: 简介
---

# 简介

部署 KubeBlocks 并创建集群后，对应的数据库会在 Kubernetes 上运行，每个副本都在 Pod 中运行，InstanceSet 将管理这些 Pod。您可以通过暴露的数据库服务地址（ClusterIP、LoadBalancer 或 NodePort），使用客户端工具或 SDK 连接到数据库。详见[在生产环境中连接数据库](./connect-to-database-in-production-environment.md)。

如果您在 Playground 或测试环境中使用 KubeBlocks 创建数据库集群，也可以使用 `kubectl port-forward` 将数据库服务地址映射到本地机器的端口。然后，您可以使用客户端工具或集成在 `kbcli` 中的通用数据库客户端连接数据库。不过，请注意这种方法仅适用于测试和调试，不应在生产环境中使用。详见[在测试环境中连接数据库](./connect-to-database-in-testing-environment.md)。
