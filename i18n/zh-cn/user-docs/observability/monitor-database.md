---
title: 监控数据库
description: 如何监控数据库
keywords: [监控数据库, 监控集群, 监控]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 监控数据库

KubeBlocks 提供了强大的可观测性能力。您可以实时观察数据库的健康状态，及时跟踪数据库，并优化数据库性能。本文档将展示 KubeBlocks 中的数据库监控工具以及它们该如何使用。

## Playground/演示场景

KubeBlocks 以插件（Addon）形式集成了许多开源监控组件，如 Prometheus、AlertManager 和 Grafana，并采用定制的 `apecloud-otel-collector` 组件收集数据库和宿主机的监控指标。您可以在测试或演示环境中使用以下监控引擎。

- `prometheus`：包括 Prometheus 和 AlertManager 两个监控组件。
- `grafana`：包括 Grafana 的监控组件。
- `victoria-metrics`：从多个源收集指标并将其存储到 VictoriaMetrics 中。
- `victoria-metrics-agent`：从多个源收集指标，重新标记和过滤收集到的指标，并通过 Prometheus `remote_write` 协议或 VictoriaMetrics `remote_write` 协议将其存储到 VictoriaMetrics 或其他存储系统中。
- `alertmanager-webhook-adaptor`：包括消息通知扩展组件，用于扩展 AlertManager 的通知能力。目前已经支持飞书、钉钉和企业微信的自定义机器人。
- `apecloud-otel-collector`：用于采集数据库和宿主机的指标。

如果您使用的是 KubeBlocks Playground，上述监控引擎默认启用。如需在测试环境中使用，可按照以下步骤操作。

### 步骤

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 查看所有支持的引擎，确保监控引擎已启用。可参考[此文档](./../installation/install-addons.md)，查看安装或启用引擎的详细说明。

    ```bash
    # 查看内置支持的所有引擎
    kbcli addon list
    ...
    grafana                        Helm   Enabled                   true                                                                                    
    alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
    ...

    # 如果监控引擎未列出，则表明该引擎未安装，可按照以下步骤安装引擎
    # 1. 检索引擎
    kbcli addon search prometheus

    # 2. 安装引擎
    kbcli addon install prometheus --index kubeblocks

    # 如果列表中有该引擎但显示为 Disabled，说明引擎未启用，可按照以下步骤启用引擎
    kbcli addon enable apecloud-otel-collector
    ```

2. 查看集群监控功能是否已开启。如果输出结果显示 `disableExporter: false`，则说明监控功能已开启。

   ```bash
   kubectl get cluster mycluster -o yaml
   >
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
   ...
   spec:
     ...
     componentSpecs:
     ...
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

</TabItem>

<TabItem value="kubectl" label="kubeclt">

:::note

以下示例展示了启用 `prometheus` 引擎。您可以将 `prometheus` 替换为其他监控引擎的名称，以开启对应的引擎。

:::

### 1. 启用监控引擎

1. （可选）添加 KubeBlocks 仓库。如果您通过 Helm 安装 KubeBlocks，可直接执行 `helm repo update`。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. 查看引擎版本。

   ```bash
   helm search repo kubeblocks/prometheus --devel --versions
   ```

3. 安装引擎。

   ```bash
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   ```

4. 验证该引擎是否安装成功。

   状态显示为 deployed 表明该引擎安装成功。

   ```bash
   helm list -A
   >
   NAME         NAMESPACE   REVISION    UPDATED                                 STATUS      CHART                APP VERSION
   ...
   prometheus   kb-system   1           2024-05-31 12:01:52.872584 +0800 CST    deployed    prometheus-15.16.1   2.39.1 
   ```

### 2. 开启集群监控功能

监控功能开启后，开源或自定义 Exporter 将会注入，Prometheus 服务器将自动发现该 Exporter，并定时抓取监控指标。

如果您在创建集群是关闭了监控功能，可执行以下命令再次开启。

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

您也可通过编辑 `cluster.yaml` 文件开启/停用监控功能。

```bash
kubectl edit cluster mycluster -n demo
```

在编辑器中修改 `disableExporter` 的参数值。

```yaml
...
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: false # 修改该参数值
...
```

（可选）如果您想要在使用后关闭监控功能，可执行以下命令停用该功能。

```bash
kubectl patch cluster mycluster -n namespace --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":true}]'
```

### 3. 查看监控大盘

使用 KubeBlocks 提供的 Grafana 引擎查看监控大盘。

1. 获取 Grafana 引擎的用户名和密码。

   ```bash
   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-user}' |base64 -d

   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-password}' |base64 -d
   ```

2. 执行以下命令连接 Grafana 大盘。

   ```bash
   kubectl port-forward svc/grafana -n kb-system 3000:80
   >
   Forwarding from 127.0.0.1:3000 -> 3000
   Forwarding from [::1]:3000 -> 3000
   Handling connection for 3000
   ```

3. 打开浏览器，输入 `127.0.0.1:3000`，跳转至大盘界面。
4. 输入第 1 步中获取的用户名和密码，即可访问。

:::note

如果大盘中无数据，您可以检查界面中的 job 是否为 `kubeblocks-service`。如果不是，可在 job 框中输入 `kubeblocks-service`，回车后再次查看。

![monitoring](./../../img/api-monitoring.png)

:::

</TabItem>

</Tabs>

## 生产环境

在生产环境中，强烈建议搭建独立的监控系统或购买第三方监控服务。

### 1. 集成监控大盘和告警规则

KubeBlocks 为主流数据库引擎提供了 Grafana 监控大盘和 Prometheus 告警规则，您可通过[该仓库](https://github.com/apecloud/kubeblocks-mixin)获取，或者按需转化或定制。

具体导入方法，可参考您使用的第三方监控服务的相关文档。

### 2. 开启数据库监控功能

查看集群监控功能是否已开启。如果输出结果显示 `disableExporter: false`，则说明监控功能已开启。

```bash
kubectl get cluster mycluster -o yaml
>
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
...
spec:
   ...
   componentSpecs:
   ...
      disableExporter: false
```

如果输出结果中未显示 `disableExporter: false`，则说明集群监控功能未开启，可执行以下命令，开启监控功能。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster update mycluster --disable-exporter=false
```

</TabItem>

<TabItem value="kubectl" label="kubeclt">

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

您也可通过编辑 `cluster.yaml` 文件开启/停用监控功能。

```bash
kubectl edit cluster mycluster -n demo
```

在编辑器中修改 `disableExporter` 的参数值。

```yaml
...
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: false # 修改该参数值
...
```

（可选）如果您想要在使用后关闭监控功能，可执行以下命令停用该功能。

```bash
kubectl patch cluster mycluster -n namespace --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":true}]'
```

</TabItem>

</Tabs>

### 3. 查看大盘

您可通过 Grafana 网页控制台查看对应集群的大盘。具体信息，可查看 [Grafana 大盘文档](https://grafana.com/docs/grafana/latest/dashboards/)。

### 4. （可选）开启远程写（Remote Write）

远程写为可选操作，您可根据实际需要开启。

KubeBlocks 支持 victoria-metrics-agent 引擎，支持用户将数据远程写入 VictoriaMetrics 中，相较于 Prometheus 原生应用，[vmagent](https://docs.victoriametrics.com/vmagent.html) 更轻量。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

2. （可选）水平扩容 `victoria-metrics-agent`。

   当数据库实例不断增多时，单节点 vmagent 会成为瓶颈。此时可以选择通过扩容 vmagent 来解决这个问题。多节点 vmagent 内部会根据哈希策略自动划分数据采集的任务。

   ```bash
   kbcli addon enable victoria-metrics-agent --replicas <replica count> --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
   ```

3. （可选）关闭 `victoria-metrics-agent` 引擎。

   ```bash
   kbcli addon disable victoria-metrics-agent
   ```

</TabItem>

<TabItem value="kubectl" label="kubeclt">

KubeBlocks 支持 `victoria-metrics-agent` 引擎，使您可以将数据远程写入您的 VM。与原生 Prometheus 相比，[vmgent](https://docs.victoriametrics.com/vmagent.html) 更轻量并且支持水平扩展。

执行以下命令，安装 `victoria-metrics-agent` 引擎。

```bash
helm install vm kubeblocks/victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
```

有关详细设置，您可以参考 [Victoria Metrics 文档](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-agent)。

</TabItem>

</Tabs>
