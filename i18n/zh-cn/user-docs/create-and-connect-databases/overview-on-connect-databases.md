---
title: 连接数据库
description: 如何连接数据库
keywords: [连接数据库]
sidebar_position: 1
sidebar_label: 简介
---

# 简介

在部署完 KubeBlocks 并创建集群之后，数据库在 Kubernetes 上以 Pod 的形式运行。

在 KubeBlocks 上创建数据库只需运行以下命令（本章节以创建一个 MySQL 集群为例进行演示。创建其他数据库引擎的操作是类似的，细节上的差别请参加各引擎文档）。
    
  ```bash
  kbcli cluster create 
  ```

你可以通过客户端界面或 `kbcli` 连接到数据库。如下图所示，首先你需要明确连接数据库的目的是什么。
- 如果你想试用 KubeBlocks、测试数据库功能或进行低流量基准测试，请参阅[在测试环境中连接数据库](../create-and-connect-databases/connect-to-database-in-testing-environment.md)。
- 如果你想在生产环境中连接数据库或进行高流量压力测试，请参阅[在生产环境中连接数据库](../create-and-connect-databases/connect-to-database-in-production-environment.md)。
![overview](./../../img/create-and-connect-databases-overview.jpg)