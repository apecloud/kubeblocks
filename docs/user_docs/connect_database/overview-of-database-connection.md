---
title: Connect database from anywhere
description: How to connect to a database
keywords: [connect to a database]
sidebar_position: 1
sidebar_label: Overview
---

# Overview of Database Connection

After deploying KubeBlocks and creating clusters, the databases run on Kubernetes, with each replica running in a Pod and managed by the InstanceSet. You can connect to the database using client tools or SDKs through the exposed database Service addresses (ClusterIP, LoadBalancer, or NodePort). See [Connect database in a production environment](connect-database-in-production-environment.md).

If you’ve created a database using KubeBlocks in a playground or test environment, you can also use `kubectl port-forward` to map the database service address to a local port on your machine. Then, you can connect to the database using a client tool or the common database clients integrated inside `kbcli`. However, please note that this is a temporary way to access services within the cluster and is intended for testing and debugging purposes only; it should not be used in a production environment. See [Connect database in a testing environment](connect-database-in-testing-environment.md).
