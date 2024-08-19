---
title: 在测试环境中连接数据库
description: 如何在测试环境中连接数据库
keywords: [连接数据库, 测试环境]
sidebar_position: 2
sidebar_label: 测试环境
---

# 在测试环境中连接数据库在测试环境中连接数据库

## 方案 1. 使用 kubectl exec 命令

如果不需要指定数据库集群地址，可执行以下命令，通过默认地址访问集群。

1. 获取集群的 `username` 和 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. 执行 `kubectl exec` 命令，指定连接的 Pod。

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash
   ```

3. 使用第 1 步中获取的 `username` 和 `password` 连接集群。

   ```bash
   mysql -u root -p2gvztbvz
   ```

## 方案 2. 通过 CLI 或 SDK 客户端 连接集群

如果您使用的引擎中 Pod 没有客户端，可执行以下步骤连接至集群。您也可以通过其他方式连接集群，如 CLI、SDK 客户端，Go 连接、Java 连接。

本文展示了使用 CLI 工具通过本机连接至集群。

1. 获取集群的 `username` 和 `password`。

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. 执行以下命令，连接集群。

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo

   mysql -h 127.0.0.1 -P 3306 -u root -p2gvztbvz
   ```
