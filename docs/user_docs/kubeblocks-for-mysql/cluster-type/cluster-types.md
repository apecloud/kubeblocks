---
title: Class types
description: Cluster Class of ApeCloud MySQL
keywords: [mysql, class type]
sidebar_position: 1
sidebar_label: Class types
---

# Cluster Class of ApeCloud MySQL

ApeCloud for MySQL predefines cluster class for different CPU, memory, and storage requirements for you to choose. It is designed to offer convenience and also set a constraints on the resources applied to the cluster to avoid lowering resource utilization and system stability by unreasonable configuration.

You can apply the cluster class when creating or vertically scaling a cluster.

📎 Table 1. General-purpose class type

| COMPONENT | CLASS            | CPU  | MEMORY |
|-----------|:-----------------|------|--------|
| mysql     | general-0.5c0.5g | 500m | 512Mi  |
| mysql     | general-1c1g     | 1    | 1Gi    |
| mysql     | general-2c2g     | 2    | 2Gi    |
| mysql     | general-2c4g     | 2    | 4Gi    |
| mysql     | general-2c8g     | 2    | 8Gi    |
| mysql     | general-4c16g    | 4    | 16Gi   |
| mysql     | general-8c32g    | 8    | 32Gi   |
| mysql     | general-16c64g   | 16   | 64Gi   |
| mysql     | general-32c128g  | 32   | 128Gi  |
| mysql     | general-64c256g  | 64   | 256Gi  |
| mysql     | general-128c512g | 128  | 512Gi  |

📎 Table 2. Memory-optimized class type

| COMPONENT | CLASS       | CPU | MEMORY |
|-----------|:------------|-----|--------|
| mysql     | mo-2c16g    | 2   | 16Gi   |
| mysql     | mo-2c32g    | 2   | 32Gi   |
| mysql     | mo-4c32g    | 4   | 32Gi   |
| mysql     | mo-4c64g    | 4   | 64Gi   |
| mysql     | mo-8c64g    | 8   | 64Gi   |
| mysql     | mo-8c128g   | 8   | 128Gi  |
| mysql     | mo-12c96g   | 12  | 96Gi   |
| mysql     | mo-16c256g  | 16  | 256Gi  |
| mysql     | mo-24c192g  | 24  | 192Gi  |
| mysql     | mo-32c512g  | 32  | 512Gi  |
| mysql     | mo-48c384g  | 48  | 384Gi  |
| mysql     | mo-48c768g  | 48  | 768Gi  |
| mysql     | mo-64c1024g | 64  | 1Ti    |
