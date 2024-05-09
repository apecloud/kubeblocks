---
title: Introduction
description: Introduction to MySQL Proxy
keywords: [introduction, proxy]
sidebar_position: 1
sidebar_label: Introduction
---

# Introduction

A database proxy is an essential tool for developers and database administrators to improve the scalability, performance, security, and resilience of their applications.

MySQL Proxy is a database proxy designed to be highly compatible with MySQL. It supports the MySQL wire protocol, read-write splitting without stale reads, connection pooling, and transparent failover. This section introduces MySQL Proxy, explaining its architecture, key features, and benefits.

## Architecture

MySQL Proxy is a fork of the Vitess project and the support for sharding is removed in exchange for better SQL compatibility, for example, better support for subqueries, Common Table Expressions (CTE) and expression evaluation. The below graph displays the architecture of a proxy cluster.

**VTGate**: Client application usually connects to VTGate via standard MySQL wire protocol. VTGate is stateless, which means it can be easily and effectively scaled in terms of size and performance. It acts like a MySQL and is responsible for parsing SQL queries, as well as planning and routing queries to VTTables.

**VTTablet**: Typically, VTTablet is implemented as a sidecar for MySQL. If MySQL Proxy is deployed in Kubernetes, VTTablet should be in the same pod as MySQL. VTTablet accepts gRPC requests from VTGate and then sends those queries to be executed on MySQL. The VTTablet takes care of a few tasks such as permission checking and logging, but its most critical role is to ensure proper connection pooling.

**VTController**: The VTController component facilitates service discovery between VTGate and VTTablet, while also enabling them to store metadata of the cluster. If the role of MySQL changes, for example, from leaders to followers, the corresponding role of VTTablet should change accordingly. VTController checks the status of the MySQL cluster and sends commands to VTTablet to request that it changes roles.

![ApeCloud MySQL Proxy architecture](./../../../img/proxy-architecture.png)

## Connection pooling

Database connections consume memory, and to ensure that the buffer pool has enough memory, databases restrict the value of 'max_connections'. As applications are generally stateless and may need to rapidly scale out, a large number of connections may be created, which can easily overload your database. So, these kinds of applications are not suitable for directly connecting to the MySQL server and creating a connection pool.

Applications can create as many connections as they require with MySQL Proxy, without any concern about 'max_connection' errors. Because MySQL Proxy takes over the establishment of a connection pool to the database, allowing applications to share and reuse connections at the MySQL server side. This reduces the memory and CPU overheads associated with opening and closing connections on the MySQL server side, improving scalability and performance.

## Read-Write splitting

Using read-only nodes can significantly reduce the workload on the primary database node and improve resource utilization. However, managing multiple read-only nodes and deciding when to use each can be challenging for applications. MySQL Proxy addresses this issue by offering three essential features: read-write splitting, read-after-write consistency, and load-balancing. These features make it easier for applications to take advantage of read-only nodes effectively.

**Read-Write Split**: MySQL Proxy simplifies application logic by automatically routing read queries to read-only nodes and write queries to the primary node. This is achieved by parsing and analyzing SQL statements, which improves load balancing and ensures efficient use of available resources.

**Read-After-Write Consistency**: This feature works in conjunction with read-write splitting to maintain data consistency while still benefiting from performance improvements. When an application writes data to the primary node and subsequently reads it on a read-only node, MySQL Proxy makes sure that the data that was just written to the primary node can be accessed and read from the read-only node.

**Load-Balancing**: MySQL Proxy helps manage read-only nodes by routing queries to the appropriate node using various load balancing policies. This ensures that the workload is evenly distributed across all available nodes, optimizing performance and resource utilization.

## Transparent failover

Failover is a feature designed to ensure that if the original database instance becomes unavailable, it is replaced with another instance and remains highly available. Various factors can trigger a failover event, including issues with the database instance or scheduled maintenance procedures like database upgrades.

Without MySQL Proxy, a failover requires a short period of downtime. Existing connections to the database are disconnected and need to be reopened by your application. MySQL Proxy is capable of automatically detecting failovers and buffering application SQL in its memory while keeping application connections intact, thus enhancing application resilience in the event of database failures.

There is no way to completely solve the application-side connection error problem. However, the proxy is stateless and can be deployed with multiple nodes for high availability. Moreover, restart/recovery is usually way faster than the database, as it does not need to recover from the undo-redo log.
