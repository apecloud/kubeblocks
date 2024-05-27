---
title: Connect database from anywhere
description: How to connect to a database
keywords: [connect to a database]
sidebar_position: 1
sidebar_label: Overview
---

# Overview of Database Connection

After the deployment of KubeBlocks and creating clusters, the database runs on Kubernetes as a Pod. You can connect to the database via a client interface or `kbcli`.
As the figure below indicates, you must be clear with the purpose of connecting the database.

- To take a trial of KubeBlocks, and test the database function, or benchmarking with low-flow testing, see [Connect database in a testing environment](connect-database-in-testing-environment.md).
- To connect a database in a production environment, or for a high-flow pressure test, see [Connect database in a production environment](connect-database-in-production-environment.md).

![Connect database](./../../img/connect_database.png)
