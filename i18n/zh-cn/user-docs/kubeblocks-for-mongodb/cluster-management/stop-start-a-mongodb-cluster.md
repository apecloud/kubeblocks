---
title: 停止/启动集群
description: 停止/启动集群
keywords: [mongodb, 停止集群, 启动集群]
sidebar_position: 5
sidebar_label: 停止/启动
---

# 停止/启动集群

你可以停止/启动集群以释放计算资源。当集群被停止时，其计算资源将被释放，也就是说 Kubernetes 的 Pod 将被释放，但其存储资源仍将被保留。如果你希望通过快照从原始存储中恢复集群资源，请重新启动该集群。

## 停止集群

***步骤：***

1. 配置集群名称，并执行以下命令来停止该集群。

    ```bash
    kbcli cluster stop mongodb-cluster
    ```

2. 检查集群的状态，查看其是否已停止。

    ```bash
    kbcli cluster list
    ```

## 启动集群
  
1. 配置集群名称，并执行以下命令来启动该集群。

   ```bash
   kbcli cluster start mongodb-cluster
   ```

2. 查看集群状态，确认集群是否再次启动。

   ```bash
   kbcli cluster list
   ```
