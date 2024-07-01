---
title: 参数模板
description: 如何在 KubeBlocks 中配置参数模板 
keywords: [参数模板]
sidebar_position: 4
sidebar_label: 参数模板
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 参数模板

本文档以 Oracle MySQL 为例，介绍如何在 KubeBlocks 中配置参数模板（点击参考[完整 PR](https://github.com/apecloud/learn-kubeblocks-addon/tree/main/tutorial-3-config-and-reconfig/)）。

## 开始之前

1. 了解 K8s 基本概念，例如 Pod，ConfigMap 等。
2. 阅读[添加数据库引擎](./how-to-add-an-add-on.md)文档。
3. 了解 Go Template（非必须）。

## 背景知识

在创建集群时，开发者通常会根据资源情况、性能需求、环境等信息调整参数。云数据库厂商，如 AWS、阿里云等也提供了不同的参数模版（例如 RDS 有高性能模版，异步模版等）供用户选择，方便快速启动。

在 K8s 中，用户可将参数文件以 ConfigMap 的形式挂载到 Pod 的卷上。但是 K8s 只管理 ConfigMap 的更新并将其同步到 Pod 卷，如果数据库引擎不支持动态加载配置文件（例如 MySQL、PostgreSQL），那就只能登录数据库并执行更新操作。而通过直接登录数据库的方式操作，很容易导致配置漂移。

为了防止配置漂移，KubeBlocks 通过 ConfigMap 来管理参数，且秉持 `ConfigMap is the only source-of-truth` 的理念，即所有配置的变更都是先应用到 ConfigMap 上，然后根据参数的不同生效方式，再应用到集群的每个 Pod 上。

本文将介绍 Kubeblocks 中参数配置的相关内容，包括如何添加参数模版、如何修改参数、如何配置参数校验等。在之后的教程中，还会详细说明参数/配置如何更新。

## 配置参数模板

KubeBlocks 通过 ***Go Template*** 来渲染参数模版，除了常见函数，还内置了一些数据库中常用到的计算函数（例如 `callBufferSizeByResource`、`getContainerCPU`）。

KubeBlocks 具有强大的渲染能力，能让你快速定制一个 ***自适应参数模板***（Adaptive ConfigTemplate），根据上下文（例如内存、CPU 大小）来渲染合适的配置文件。

### 添加参数模板

```yaml
1 apiVersion: v1
2 kind: ConfigMap
3 metadata:
4   name: oracle-mysql-config-template
5   labels:
6     {{- include "oracle-mysql.labels" . | nindent 4 }}
7 data:
8   my.cnf: |-
9     {{`
10      [mysqld]
11      port=3306
12      {{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
13      {{- $pool_buffer_size := ( callBufferSizeByResource ( index $.podSpec.containers 0 ) ) }}
14      {{- if $pool_buffer_size }}
15      innodb_buffer_pool_size={{ $pool_buffer_size }}
16      {{- end }}
17 
18      # 如果内存小于 8Gi，禁用 performance_schema
19      {{- if lt $phy_memory 8589934592 }}
20      performance_schema=OFF
21      {{- end }}
22 
23      [client]
24      port=3306
25      socket=/var/run/mysqld/mysqld.sock
26      `
27    }}
```

上面展示了一个通过 ConfigMap 定义的 MySQL 自适应参数模板。模板中配置了几个常见的 MySQL 参数，包括 `port`，`innodb_buffer_pool_size` 等。
它根据容器启动时配置的 memory，

- 计算得到 `innodb_buffer_size` 大小（Line 11 ~ Line 15）。
- 在 memory 小于 8Gi 时，禁用 `performance_schema` 来减少对性能的影响（Line 19 ~ Line 21）。

`callBufferSizeByResource` 是 KubeBlocks 预定义的一个 bufferPool 计算规则，主要为 MySQL 服务。

此外，你也可以通过查询 memory 和 cpu 来定制你的计算公式：

- `getContainerMemory` 获取 Pod 上某个 container 的 memory 大小。
- `getContainerCPU` 获取 Pod 中某个 container 的 CPU 大小。

:::note

你可以按需定制更多的参数计算方式，比如

- 根据 memory 大小，计算出一个合适的 `max_connection` 值。
- 根据 memory 总量，计算其他组件的合理配置。

:::

### 使用参数模板

#### 修改 ClusterDefinition

可以通过 `ClusterDefinition` 中的 `configSpecs` 来指定参数模板，引用在[添加参数模板](#添加参数模板)中定义的 ConfigMap。

```yaml
  componentDefs:
    - name: mysql-compdef
      configSpecs:
        - name: mysql-config
          templateRef: oracle-mysql-config-template # 定义参数模板的 ConfigMap 名
          volumeName: configs                       # 挂载的卷名称         
          namespace: {{ .Release.Namespace }}       # 该参数模板 ConfigMap 的 Namespace
      podSpec:
        containers:
         - name: mysql-container
           volumeMounts:
             - mountPath: /var/lib/mysql
               name: data
             - mountPath: /etc/mysql/conf.d       # 挂载的配置文件路径，引擎相关  
               name: configs                      # 和 Line 6 的 volumeName 对应   
           ports:
            ...
```

如上所示，需要通过添加 `configSpecs` 字段修改 `ClusterDefinition.yaml` 文件，分别指定：

- templateRef: 模板所在的 ConfigMap 对象名称。
- volumeName: 挂载到 Pod 的卷名。
- namespace: 模板文件的名空间（ConfigMap 是 namespace scope 的，一般为 KubeBlocks 安装的命名空间）。

#### 查看配置信息

当一个新的集群创建后，KubeBlocks 会根据配置模板渲染好对应的 Configmap，并将该 ConfigMap 挂载到 `configs` 卷中。

1. 安装 Helm chart。

   ```bash
   helm install oracle-mysql path-to-your-helm-char/oracle-mysql
   ```

2. 创建集群。

   ```bash
   kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.32
   ```

3. 查看配置。

   kbcli 提供了 `describe-config` 子命令来查看集群的配置信息。

   ```bash
   kbcli cluster describe-config mycluster --components mysql-compdef
   >
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME   FILE     ENABLED   TEMPLATE                       CONSTRAINT   RENDERED                               COMPONENT       CLUSTER
   mysql-config       my.cnf   false     oracle-mysql-config-template                mycluster-mysql-compdef-mysql-config   mysql-compdef   mycluster

   History modifications:
   OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED
   ```

可以查看到：

- 配置模版名：oracle-mysql-config-template。
- 渲染后的 ConfigMap：mycluster-mysql-compdef-mysql-config。
- 加载的文件名：my.cnf。

## 总结

本文档介绍了如何通过参数模板来渲染“自适应”参数。

K8s 会将 ConfigMap 的变更定时同步到 Pod 上，但是大部分引擎并不会主动加载新的配置（比如 MySQL、PostgreSQL、Redis）。因为，用户无法仅通过修改 ConfigMap 就实现 Reconfig（参数变更）的能力。下一篇文档将介绍如何配置参数变更。

## 附录

### A.1 如何配置多个参数模板？

在生产环境中，开发者通常需要多个参数模板来满足不同需求。例如阿里云 RDS 就提供了高性能参数模板、异步模板等。

在 KubeBlocks 中，你可以通过配置多个 `ClusterVerion` 来实现这一需求。

还记得我们对 cluster 的定义吗，它可以表示为：

$$
Cluster = ClusterDefinition.yaml \Join ClusterVersion.yaml \Join Cluster.yaml
$$

其中 JoinKey 就是 Component Name。

而多个 ClusterVersion 可以和同一个 ClusterDefinition 组合。

```yaml
## 第一个 ClusterVersion，使用 ClusterDefinition 中的配置
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: oracle-mysql
spec:
  clusterDefinitionRef: oracle-mysql
  componentVersions:
  - componentDefRef: mysql-compdef
    versionsContext:
      containers:
        - name: mysql-container
          ...
---
## 第二个 ClusterDefinition，定义了自己的 configSpecs，会覆盖 ClusterDefinition 的配置
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: oracle-mysql-perf
spec:
  clusterDefinitionRef: oracle-mysql
  componentVersions:
  - componentDefRef: mysql-compdef
    versionsContext:
      containers:
        - name: mysql-container
         ...
    # 这里的 name 需要与 ClusterDefinition 中定义的 ConfigMap 的名称一致
    configSpecs:
      - name: mysql-config    
        templateRef: oracle-mysql-perf-config-template
        volumeName: configs
```

这里创建了两个 `ClusterVersion` 对象。第一个使用了默认的参数模版（没有配置任何信息）。第二个通过 `configSpecs` 指定了一个新的参数模版 `oracle-mysql-perf-config-template`。 

在创建 Cluster 时，你可以指定 `ClusterVersion` 参数来创建不同配置的集群，如：

```bash
kbcli cluster create mysqlcuster --cluster-definition oracle-mysql --cluster-version  oracle-mysql-perf
```

:::note

KubeBlocks 会通过 `configSpecs.name` 来合并 ClusterVersion 和 ClusterDefinition 中的配置。请确保在 ClusterVersion 中定义的 `configSpecs.name` 和 ClusterDefinition 中定义的名称一致。

:::
