---
title: 访问日志
description: 如何访问日志
keywords: [访问日志]
sidebar_position: 3
---

# 访问日志

为简化用户排查问题的复杂度，KubeBlocks 的命令行工具 kbcli 支持查看运行在 KubeBlocks 上的数据引擎生成的各种日志，例如慢日志、错误日志、审计日志和容器运行日志（Stdout 和 Stderr）等。对于 Redis 数据库，仅支持查看运行日志。以上称为日志增强功能。

KubeBlocks 日志增强功能使用类似 `kubectl exec` 和 `kubectl logs` 的方法，确保自闭环和轻量化。

## 开始之前

- 容器镜像支持 `tail` 和 `xargs` 命令。
- 安装 KubeBlocks：你可以通过 [kbcli](../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 或 [Helm](../installation/install-with-helm/install-kubeblocks-with-helm.md) 进行安装。
- 在本指南中，我们以 MySQL 引擎为例。其他数据库引擎操作相同。

## 步骤

1. 启用日志增强功能。

   - 在创建集群时启用此功能。
     - 如果你通过执行 `kbcli cluster create` 命令创建集群，请添加 `--enable-all-logs=true` 启用日志增强功能。当此选项为 `true` 时，`ClusterDefinition` 中 `spec.componentDefs.logConfigs` 定义的所有日志类型将自动启用。

        ```bash
        kbcli cluster create mysql --enable-all-logs=true mycluster
        ```

   - 如果在创建集群时未启用该功能，请更新该集群。

      ```bash
      kbcli cluster update mycluster --enable-all-logs=true -n <namespace>
      ```

      :::note

      创建集群时的默认命名空间是 `default`。如果在创建集群时指定了命名空间，请将 `<namespace>` 填写为你自定义的命名空间。

      :::

2. 查看支持的日志。

    执行 `kbcli cluster list-logs` 命令，查看目标集群中已开启日志增强功能的日志类型和日志文件的详细信息，展示集群中的每个节点实例。

      ***示例***

      ```bash
      kbcli cluster list-logs mycluster
      >
      INSTANCE                 LOG-TYPE        FILE-PATH                                   SIZE        LAST-WRITTEN                          COMPONENT
      mycluster-mysql-0        error           /data/mysql/log/mysqld-error.log            6.4K        Feb 06, 2023 09:13 (UTC+00:00)        mysql
      mycluster-mysql-0        general         /data/mysql/log/mysqld.log                  5.9M        Feb 06, 2023 09:13 (UTC+00:00)        mysql
      mycluster-mysql-0        slow            /data/mysql/log/mysqld-slowquery.log        794         Feb 06, 2023 09:13 (UTC+00:00)        mysql       
      ```

3. 访问集群日志文件。

   执行 `kbcli cluster logs` 命令，查看目标集群上目标节点生成的日志文件的详细信息。你可以使用不同的方法来查看所需要的日志文件。还可以执行 `kbcli cluster logs -h` 查看示例和方法。

    ```bash
    kbcli cluster logs -h
    ```

    <details>

    <summary>输出</summary>

    ```bash
    Access cluster log file

    Examples:
    # 返回集群 mycluster 的快照日志，默认使用主实例（stdout）
    kbcli cluster logs mycluster

    # 仅显示集群 mycluster 的最近 20 行日志，默认使用主实例（stdout）
    kbcli cluster logs --tail=20 mycluster

    # 返回集群 mycluster 中指定实例 my-instance-0 的快照日志（stdout）
    kbcli cluster logs mycluster --instance my-instance-0

    # 返回集群 mycluster 中指定实例 my-instance-0 和指定容器 my-container 的快照日志（stdout）
    kbcli cluster logs mycluster --instance my-instance-0 -c my-container

    # 返回集群 mycluster 的慢日志，默认使用主实例
    kbcli cluster logs mycluster --file-type=slow

    # 实时流式传输集群 mycluster 的慢日志，默认使用主实例
    kbcli cluster logs -f mycluster --file-type=slow

    # 返回集群 mycluster 中指定实例 my-instance-0 的指定文件日志
    kbcli cluster logs mycluster --instance my-instance-0 --file-path=/var/log/yum.log

    # 返回集群 mycluster 中指定实例 my-instance-0 和指定容器 my-container 的指定文件日志
    kbcli cluster logs mycluster --instance my-instance-0 -c my-container --file-path=/var/log/yum.log
    ```

    </details>

2. （可选）故障排除。

    日志增强功能不会影响 KubeBlocks 的核心流程。如果发生配置异常，你将收到告警信息，方便排查问题。 `warning` 信息记录在目标数据引擎集群的 `event` 和 `status.Conditions` 中。

    查看 `warning` 信息。
      - 执行 `kbcli cluster describe <cluster-name>`，查看目标集群的状态。也可以执行 `kbcli cluster list events <cluster-name>` 直接查看目标集群的事件信息。

        ```bash
        kbcli cluster describe mycluster
        ```

        ```bash
        kbcli cluster list-events mycluster
        ```

      -  执行 `kubectl describe cluster <cluster-name>` 查看告警信息。

          ```bash
          kubectl describe cluster mycluster
          ```

         ***示例***
          ```bash
          Status:           
            Cluster Def Generation:  3         
            Components:               
                Replicasets:                 
                  Phase:  Running           
            Conditions:             
              Last Transition Time:  2022-11-11T03:57:42Z             
              Message:               EnableLogs of cluster component replicasets has invalid value [errora slowa] which isn't defined in cluster definition component replicasets             
              Reason:                EnableLogsListValidateFail             
              Status:                False             
              Type:                  ValidateEnabledLogs           
            Observed Generation:     2           
            Operations:             
              Horizontal Scalable:                 
                  Name:  replicasets             
              Restartable:               
                replicasets             
              Vertical Scalable:               
                replicasets           
              Phase:  Running         
            Events:           
              Type     Reason                      Age   From                Message           
              ----     ------                      ----  ----                -------           
              Normal   Creating                    49s   cluster-controller  Start Creating in Cluster: release-name-error           
              Warning  EnableLogsListValidateFail  49s   cluster-controller  EnableLogs of cluster component replicasets has invalid value [errora slowa] which isn't defined in cluster definition component replicasets           
              Normal   Running                     36s   cluster-controller  Cluster: release-name-error is ready, current phase is Running         
          ```

