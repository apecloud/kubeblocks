---
title: PostgreSQL 简介
description: PostgreSQL 简介
keywords: [postgresql, 简介]
sidebar_position: 1
---

# PostgreSQL 简介

PostgreSQL 是一个强大、安全、可扩展、可定制的开源关系型数据库，适用于各种应用和环境。除了 PostgreSQL 的核心功能外，KubeBlocks PostgreSQL 集群还支持多个插件，用户可以自行添加。

* PostGIS：PostGIS 是一个开源的空间数据库插件，为 PostgreSQL 数据库添加了地理信息系统（GIS）功能。它提供了一组可用于存储、查询和分析地理空间数据的空间函数和类型，允许使用 SQL 运行位置查询。
* pgvector：pgvector 是一个支持向量数据类型和向量操作的 PostgreSQL 插件。它可以高效存储和查询向量数据，支持各种向量操作，如相似性搜索、聚类、分类和推荐系统。Pgvector 可用于存储 AI 模型的嵌入向量，为 AI 添加持久性内存。
* pg_trgm：pg_trgm 提供了基于 trigram 匹配的字母数字文本相似度计算的函数和操作符，以及支持快速搜索类似字符串的索引运算符类。
* postgres_fdw：postgres_fdw 插件可以将远程数据库的表映射到 PostgreSQL 中的本地表，允许用户通过本地数据库查询和操作数据。

**参考资料**

* [PostgreSQL features](https://www.postgresql.org/about/featurematrix/)
* [PostGIS](https://postgis.net/)
* [pgvector](https://github.com/pgvector/pgvector)
