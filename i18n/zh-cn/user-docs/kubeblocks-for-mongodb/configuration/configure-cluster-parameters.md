---
title: 配置集群参数
description: 配置集群参数
keywords: [mongodb, 参数, 配置, 再配置]
sidebar_position: 1
---

# 配置集群参数

KubeBlocks 提供了一套默认的配置生成策略，适用于在 KubeBlocks 上运行的所有数据库，此外还提供了统一的参数配置接口，便于管理参数配置、搜索参数用户指南和验证参数有效性等。

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure` 可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

## 查看参数信息

查看集群的配置文件。

```bash
kbcli cluster describe-config mongodb-cluster
>
ConfigSpecs Meta:
CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                            COMPONENT    CLUSTER           
mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-replicaset-mongodb-metrics-config   replicaset   mongodb-cluster   

History modifications:
OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED 
```

从元信息中可以看到，集群 `mongodb-cluster` 有一个名为 `mongodb.conf` 的配置文件。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mongodb-cluster --show-detail
   ```

## 配置参数

### 使用 configure 命令配置参数

下面展示如何将 systemLog.verbosity 配置为 1。

1. 将 `systemLog.verbosity` 设置为 1。

   ```bash
   kbcli cluster configure mongodb-cluster --component mongodb --config-spec mongodb-config --config-file mongodb.conf --set systemLog.verbosity=1
   >
   Warning: The parameter change you modified needs to be restarted, which may cause the cluster to be unavailable for a period of time. Do you need to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
   ConfigSpec: mongodb-config      ConfigFile: mongodb.conf      ComponentName: mongodb  ClusterName: mongodb-cluster
   OpsRequest mongodb-cluster-reconfiguring-q8ndn created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-reconfiguring-q8ndn -n default
   ```

2. 检查配置历史。

   ```bash

    kbcli cluster describe-config mongodb-cluster
    >
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                         COMPONENT   CLUSTER
    mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-mongodb-mongodb-metrics-config   mongodb     mongodb-cluster

    History modifications:
    OPS-NAME                              CLUSTER           COMPONENT   CONFIG-SPEC-NAME   FILE           STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
    mongodb-cluster-reconfiguring-q8ndn   mongodb-cluster   mongodb     mongodb-config     mongodb.conf   Succeed   restart   3/3        Apr 21,2023 18:56 UTC+0800   {"mongodb.conf":"{\"systemLog\":{\"verbosity\":\"1\"}}"}```
   ```

3. 验证配置结果。

   ```bash
    root@mongodb-cluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
    verbosity: "1"
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config mongodb-cluster
   ```

      :::note

      如果集群中有多个组件，请使用 `--component` 参数指定一个组件。
      
      :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. 连接到数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect mongodb-cluster
   ```

      :::note

      1. `edit-config` 不能同时编辑静态参数和动态参数。
      2. KubeBlocks 未来将支持删除参数。

      :::
