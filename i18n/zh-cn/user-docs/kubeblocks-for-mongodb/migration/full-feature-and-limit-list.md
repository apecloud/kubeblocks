---
title: 功能及限制
description: KubeBlocks MongoDB 迁移功能列表及限制
keywords: [mongodb, 迁移, 功能, 限制]
sidebar_position: 1
sidebar_label: 功能及限制
---

# 功能及限制

## 功能

* 预检查
  * 数据库连接
  * 数据库版本
  * 数据库是否支持增量迁移
  * 是否支持源库表结构
* 数据初始化
  * 支持所有主要数据类型
* 增量数据迁移
  * 支持所有主要数据类型
  * 支持最终一致的断点续传能力

## 限制

* 整体限制
  * 如果使用增量数据迁移，源数据库应该是主备版结构中的主节点。
  * 除增量数据迁移模块外，其他模块不支持断点续传能力。即，如果此模块发生异常（例如由于停机和断网引起 Pod 故障），需要重新迁移。
  * 在数据传输任务过程中，不支持对源数据库中的迁移对象进行 Drop、Rename 和 DropDatabase 等操作。
  * 数据库和集合名称不能包含中文字符和特殊字符，如单引号（'）和逗号（,）。
  * 在迁移过程中，不支持源库中的主从节点切换。因为这可能导致任务配置中指定的连接串发生变化，进而导致链路失败。
* 预检查模块：无
* 数据初始化模块
  * 不支持除 UTF-8 以外的数据库字符集。
* 增量数据迁移模块
  * 不支持除 UTF-8 以外的数据库字符集。
