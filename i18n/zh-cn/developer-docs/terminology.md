---
title: 术语
description: KubeBlocks 中的术语
keywords: [术语]
sidebar_position: 2
sidebar_label: 术语
---
# 术语

##### Addon

Addon 是一种高效且开放的扩展机制。开发者可以使用 KubeBlocks addon 快速在 KubeBlocks 中添加新的数据库引擎，并获取该引擎的基础管理功能，比如生命周期管理、数据备份与恢复、指标和日志收集等等。

##### ActionSet

ActionSet 是一组使用特定工具执行备份恢复的命令，比如使用 xtrabackup 备份 MySQL 的命令，以及从备份中恢复数据的命令等等。

##### BackupPolicy

BackupPolicy 表示集群的备份策略，涵盖了备份存储库（BackupRepo）、备份对象和备份方法等信息。在一个备份策略中可以定义多个备份方法，每个方法引用相应的 ActionSet。在创建备份时，可以指定所需的备份策略和备份方法。

##### BackupRepo

BackupRepo 是备份数据的存储库。其原理是使用 CSI 驱动程序将备份数据上传到各种存储系统，例如对象存储系统（如 S3、GCS）和存储服务器（如 FTP、NFS）等。

##### BackupSchedule

BackupSchedule 声明了集群中自动备份的配置，包括备份频率、保留期限、备份策略和备份方法等。BackupSchedule Controller 会根据自定义资源（CR）中指定的配置创建 CronJob，自动备份集群。

##### Cluster

Cluster 即集群，集群由组件构成。

##### Component

Component 即组件，是构建数据存储和处理系统的基本单元。组件利用 StatefulSet（可以是 Kubernetes 原生 StatefulSet，也可以是客户指定的 StatefulSet，如 OpenKruise）来管理一个或多个 Pod。

##### ComponentRef

ComponentRef 用于选择需要引用的组件及其字段。

##### ComponentClassDefinition

ComponentClassDefinition 用于定义组件参数。

##### ConfigConstraint

为了方便更改配置，KubeBlocks 将引擎配置文件抽象为 ConfigConstraints。ConfigConstraints 抽象出来的信息包括：
 - 配置文件的格式；
 - 动态参数、静态参数、不可变参数；
 - 动态变化的参数；
 - 参数校验规则。

##### CRD (Custom Resource Definition)

CRD（Custom Resource Definition），即自定义资源定义，是一种强大的 Kubernetes API 扩展机制，允许开发者引入新的数据类型和对象（即自定义的资源）。

##### Operator

Operator 是一种自定义资源，可以实现对一个或多个特定应用或服务的自动化管理。Operator 通过监视和响应自定义资源对象的状态变化，帮助在 Kubernetes 中进行管理任务。

##### OpsDefinition

Ops 是 Operations（操作）的简称，表示数据库运维操作。它定义了与数据库管理相关的操作任务，指定集群和组件支持的操作类型。

##### OpsRequest

一个 OpsRequest 表示一次运维任务请求。

##### RBAC (Role-Based Access Control)

RBAC（Role-Based Access Control），即基于角色的访问控制，是一种计算机系统的安全机制，旨在根据组织内个人用户的角色授予其对资源的访问权限，控制用户访问那些仅限于授权用户访问的任务。Kubernetes 的内置 API 可以管理命名空间和集群中的角色，使它们同特定的资源与个人用户关联起来。

##### RSM （ReplicatedStateMachines）

RSM（ReplicatedStateMachines）是一种负责管理本地 Kubernetes 对象（如 StatefulSet 和 Pod）的工作负载。

##### ServiceDescriptor

ServiceDescriptor 是一个自定义资源（CR）对象，用于描述引用存储服务的 API 对象。它允许用户抽象出由 Kubernetes 或非 Kubernetes 环境提供的服务，可供 KubeBlocks 内的其他集群对象引用。ServiceDescriptor 可用于解决诸如服务依赖、组件依赖和组件共享等问题。