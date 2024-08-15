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

2. 查看集群监控功能是否已开启。如果输出结果显示 `disableExporter: false`，则说明监控功能已开启。

   ```bash
   kubectl get cluster mycluster -o yaml
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
   ......
   spec:
     ......
     componentSpecs:
     ......
       disableExporter: false
   ```

   如果输出结果中未显示 `disableExporter: false`，则说明集群监控功能未开启，可执行以下命令，开启监控功能。

   ```bash
   kbcli cluster update mycluster --disable-exporter=false
   ```

3. 查看仪表盘列表。

    ```bash
    kbcli dashboard list
    >
    NAME                                 NAMESPACE   PORT    CREATED-TIME
    kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
    kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
    kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
    ```

4. 打开网页控制台并查看。例如：

    ```bash
    kbcli dashboard open kubeblocks-grafana
    ```

## 生产环境

在生产环境中，强烈建议搭建独立的监控系统或购买第三方监控服务。

### 集成监控大盘和告警规则

KubeBlocks 为主流数据库引擎提供了 Grafana 监控大盘和 Prometheus 告警规则，您可通过[该仓库](https://github.com/apecloud/kubeblocks-mixin)获取，或者按需转化或定制。

具体导入方法，可参考您使用的第三方监控服务的相关文档。

### 开启数据库监控功能

查看集群监控功能是否已开启。如果输出结果显示 `disableExporter: false`，则说明监控功能已开启。

```bash
kubectl get cluster mycluster -o yaml
>
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
......
spec:
   ......
   componentSpecs:
   ......
      disableExporter: false
```

如果输出结果中未显示 `disableExporter: false`，则说明集群监控功能未开启，可执行以下命令，开启监控功能。

```bash
kbcli cluster update mycluster --disable-exporter=false
```

### 查看大盘

您可通过 Grafana 网页控制台查看对应集群的大盘。具体信息，可查看 [Grafana 大盘文档](https://grafana.com/docs/grafana/latest/dashboards/)。

### （可选）开启远程写（Remote Write）

远程写为可选操作，您可根据实际需要开启。

KubeBlocks 支持 victoria-metrics-agent 引擎，支持用户将数据远程写入虚拟机中，相较于 Prometheus 原生应用，[vmagent](https://docs.victoriametrics.com/vmagent.html) 更轻量。

1. 开启数据推送。

   你只需要提供支持 Prometheus Remote Write 协议的终端节点（可以支持多个终端节点）。关于获取方式，请参考第三方监控系统的文档。

   下面展示如何通过不同的方法启用数据推送：

   ```bash
   # 默认选项：只需提供一个无需验证的终端节点。
   # 示例：http://localhost:8428/api/v1/write
   kbcli addon enable victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
   ```

   ```bash
   # Basic Auth 方式
   kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite\.basicAuth\.username=<your username>,extraArgs.remoteWrite\.basicAuth\.password=<your password>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

   ```bash
   # TLS 方式
   kbcli addon enable victoria-metrics-agent --set "extraArgs.tls=true,extraArgs.tlsCertFile=<path to certifle>,extraArgs.tlsKeyFile=<path to keyfile>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

   ```bash
   # AWS SigV4 方式
   kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite\.aws\.region=<your AMP region>,extraArgs.remoteWrite\.aws\.accessKey=<your accessKey>,extraArgs.remoteWrite\.aws\.secretKey=<your secretKey>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

2. 可选）水平扩容 `victoria-metrics-agent`。

   当数据库实例不断增多时，单节点 vmagent 会成为瓶颈。此时可以选择通过扩容 vmagent 来解决这个问题。多节点 vmagent 内部会根据哈希策略自动划分数据采集的任务。

   ```bash
   kbcli addon enable victoria-metrics-agent --replicas <replica count> --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
   ```

3. （可选）禁用 `victoria-metrics-agent`。

   ```bash
   kbcli addon disable victoria-metrics-agent
   ```
