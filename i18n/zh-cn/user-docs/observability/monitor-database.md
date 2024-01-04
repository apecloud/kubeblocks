---
title: 监控数据库
description: 如何监控数据库
keywords: [监控数据库, 监控集群, 监控]
sidebar_position: 1
---

# 监控数据库

KubeBlocks 提供了强大的可观测性能力。你可以实时观察数据库的健康状态，及时跟踪数据库，并优化数据库性能。本文档将展示 KubeBlocks 中的数据库监控工具以及它们该如何使用。

## Playground 演示场景

KubeBlocks 以引擎形式集成了许多开源监控组件，如 Prometheus、AlertManager 和 Grafana，并采用定制的 `apecloud-otel-collector` 组件收集数据库和宿主机的监控指标。在部署 KubeBlocks Playground 时，所有监控组件都会默认启用。

KubeBlocks Playground 内置以下几个监控组件：

- `prometheus`：包括 Prometheus 和 AlertManager 两个监控组件。
- `grafana`：包括 Grafana 的监控组件。
- `alertmanager-webhook-adaptor`：包括消息通知扩展组件，用于扩展 AlertManager 的通知能力。目前已经支持飞书、钉钉和企业微信的自定义机器人。
- `apecloud-otel-collector`：用于采集数据库和宿主机的指标。

1. 查看所有支持的引擎，确保监控引擎已启用。

    ```bash
    # 查看内置支持的所有引擎
    kbcli addon list
    ...
    grafana                        Helm   Enabled                   true                                                                                    
    alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
    prometheus                     Helm   Enabled    alertmanager   true 
    ...
    ```

2. 查看仪表盘列表。

    ```bash
    kbcli dashboard list
    >
    NAME                                 NAMESPACE   PORT    CREATED-TIME
    kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
    kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
    kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
    ```

3. 打开网页控制台并查看。例如：

    ```bash
    kbcli dashboard open kubeblocks-grafana
    ```

## 生产环境

在生产环境中，强烈建议搭建独立的监控系统或购买第三方监控服务。

### 启用监控功能

KubeBlocks 提供了一个名为 `victoria-metrics-agent` 的引擎，可以将监控数据推送到与 Prometheus Remote Write 协议兼容的第三方监控系统。与原生的 Prometheus 相比，[vmagent](https://docs.victoriametrics.com/vmagent.html) 更轻量且支持水平扩展。

1. 启用数据推送。

    你只需要提供支持 Prometheus Remote Write 协议的终端节点（可以支持多个终端节点）。关于获取方式，请参考第三方监控系统的文档。

    下面展示如何通过不同的方法启用数据推送：

    ```bash
    # 默认选项：只需提供一个无需验证的终端节点。
    # 示例：http://localhost:8428/api/v1/write
    kbcli addon enable victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
    ```

    ```bash
    # Basic Auth 方式
    kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite.basicAuth.username=<your username>,remoteWrite.basicAuth.password=<your password>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
    ```

    ```bash
    # TLS 方式
    kbcli addon enable victoria-metrics-agent --set "extraArgs.tls=true,extraArgs.tlsCertFile=<path to certifle>,extraArgs.tlsKeyFile=<path to keyfile>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
    ```

    ```bash
    # AWS SigV4 方式
    kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite.aws.region=<your AMP region>,extraArgs.remoteWrite.aws.accessKey=<your accessKey>,extraArgs.remoteWrite.aws.secretKey=<your secretKey>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
    ```

2. （可选）水平扩容 `victoria-metrics-agent`。

    当数据库实例不断增多时，单节点 vmagent 会成为瓶颈。此时可以选择通过扩容 vmagent 来解决这个问题。多节点 vmagent 内部会根据哈希策略自动划分数据采集的任务。

    ```bash
    kbcli addon enable victoria-metrics-agent --replicas <replica count> --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
    ```

3. （可选）禁用 `victoria-metrics-agent`。

    ```bash
    kbcli addon disable victoria-metrics-agent
    ```

### 集成仪表盘和告警规则

Kubeblocks 提供了主流引擎的 Grafana 仪表盘和 Prometheus 告警规则，你可以从 [Kubeblocks 仓库](https://github.com/apecloud/kubeblocks-mixin) 获取这些资源，或者根据自己的需求进行转换和定制。

关于如何导入，请参考第三方监控服务的文档。

## 启用数据库的监控功能

在创建数据库时，KubeBlocks 默认启用监控功能，启用后会注入开源或定制的采集组件。该采集组件会被 Prometheus 服务器自动发现并定期抓取监控指标。你可以将 `mysql` 更改为 `postgresql`、`mongodb` 或 `redis` 以创建其他数据库引擎的集群。

- 对于新集群，请执行以下命令创建数据库集群。

    ```bash
    # 查询集群定义
    kbcli clusterdefinition list 

    # 创建集群
    kbcli cluster create mysql <clustername> 
    ```

    :::note

    你可以将 `--monitoring-interval` 设置为 `0` 以关闭监控功能（不建议关闭）。

    ```bash
    kbcli cluster create mysql mycluster --monitoring-interval=0
    ```

    :::

- 对于已禁用监控功能的集群，可以使用 `update` 子命令启用监控功能。

    ```bash
    kbcli cluster update mycluster --monitoring-interval=15s
    ```

你可以通过 Grafana Web Console 查看相应集群的仪表盘。更多信息请参阅 [Grafana 仪表盘文档](https://grafana.com/docs/grafana/latest/dashboards/)。
