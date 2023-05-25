---
title: PostgreSQL introduction
description: PostgreSQL introduction
keywords: [postgresql, introduction]
sidebar_position: 1
---

# PostgreSQL introduction

PostgreSQL is a powerful, scalable, secure, and customizable open-source relational database management system that is suitable for various applications and environments. In addition to the PostgreSQL core features, the KubeBlocks PostgreSQL cluster supports a number of extensions and users can also add the extensions by themselves.

* PostGIS：PostGIS is an open-source spatial database extension that adds geographic information system (GIS) functionality to the PostgreSQL database. It provides a set of spatial functions and types that can be used for storing, querying, and analyzing geospatial data, allowing you to run location queries with SQL.
* pgvector: Pgvector is a PostgreSQL extension that supports vector data types and vector operations. It provides an efficient way to store and query vector data, supporting various vector operations such as similarity search, clustering, classification, and recommendation systems. Pgvector can be used to store AI model embeddings, adding persistent memory to AI.
* pg_trgm：The pg_trgm module provides functions and operators for determining the similarity of alphanumeric text based on trigram matching, as well as index operator classes that support fast searching for similar strings.
* postgres_fdw：The postgres_fdw extension can map tables from a remote database to a local table in PostgreSQL, allowing users to query and manipulate the data through the local database.

**Reference**

* [PostgreSQL features](https://www.postgresql.org/about/featurematrix/)
* [PostGIS](https://postgis.net/)
* [pgvector](https://github.com/pgvector/pgvector)
