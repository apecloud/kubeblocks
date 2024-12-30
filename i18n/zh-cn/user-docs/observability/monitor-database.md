---
title: 监控
description: 如何监控数据库集群
keywords: [监控, 监控数据库集群]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 监控数据库

您可按需通过不同的监控工具监控集群状态，本教程使用 Prometheus 和 Grafana 作为监控工作，以配置 PostgreSQL 集群的监控功能为例进行说明。

## 步骤 1. 安装 Prometheus Operator 和 Grafana

安装 Prometheus Operator 和 Grafana, 监控数据库性能指标。如果您的环境中已有 Prometheus Operator，可跳过本节。

1. 为监控 Operator 创建新的命名空间。

   ```bash
   kubectl create namespace monitoring
   ```

2. 添加 Prometheus Operator Helm 仓库。

   ```bash
   helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
   ```

3. 安装 Prometheus Operator。

   ```bash
   helm install prometheus-operator prometheus-community/kube-prometheus-stack --namespace monitoring
   ```

4. 验证 Prometheus Operator 是否安装成功。

   ```bash
   kubectl get pods -n monitoring
   ```

5. 连接 Prometheus 和 Grafana 大盘。

   1. 查看 Prometheus 和 Grafana 的服务端口。

       ```bash
       kubectl get svc -n monitoring
       ```

   2. 使用 port forward 从本地连接 Prometheus 大盘。

       ```bash
       kubectl port-forward svc/prometheus-operator-kube-p-prometheus -n monitoring 9090:9090
       ```

       您也可通过在浏览器中打开 "http://localhost:9090" 地址，连接 Prometheus 大盘。

   3. 从 secret 中获取 Grafana 的连接凭证。

       ```bash
       kubectl get secrets prometheus-operator-grafana -n monitoring -oyaml
       ```  

   4. 使用 port forward 从本地连接 Grafana 大盘。

       ```bash
       kubectl port-forward svc/prometheus-operator-grafana -n monitoring 3000:80
       ```

       您也可通过在浏览器中打开 "http://localhost:3000" 地址，连接 Grafana 大盘。

6. （可选）配置 `PodMonitor` 及 `ServiceMonitor` 选择器。

   Prometheus Operator 使用 Prometheus CRD 创建实例，自定义配置 replica、PVC 等其他参数。

   如需更新 `PodMonitor` 及 `ServiceMonitor` 的配置，您可按需更新 Prometheus CR 文件。

   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: Prometheus
   metadata:
   spec:
     podMonitorNamespaceSelector: {} # 匹配 PodMonitor 的命名空间
     # 选择用于目标发现的 PodMonitors。空的标签选择器
     # 会匹配所有对象
     podMonitorSelector:
       matchLabels:
         release: prometheus # 确保您的 PodMonitor CR 的标签与此选择器匹配
     serviceMonitorNamespaceSelector: {} # 匹配 ServiceMonitor 的命名空间
     # 选择用于目标发现的 ServiceMonitors。空的标签选择器
     # 会匹配所有对象
     serviceMonitorSelector:
       matchLabels:
         release: prometheus # 确保您的 ServiceMonitor CR 的标签与此选择器匹配
   ```

## 步骤 2. 监控数据库集群

监控集群的方式多种多样，本文档中我们使用 Promethus 和 Grafana 演示如何监控集群。

### 开启集群的监控 exporter

#### 创建新集群并开启监控 exporter

使用以下命令创建集群，并开启监控 exporter。

:::Note

创建集群时，请确保 `spec.componentSpecs.disableExporter` 设为 `false`。

:::

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: demo
spec:
  clusterDefinitionRef: postgresql
  clusterVersionRef: postgresql-12.14.0
  terminationPolicy: Delete
  affinity:
    podAntiAffinity: Preferred
    topologyKeys:
    - kubernetes.io/hostname
    tenancy: SharedNode
  tolerations:
    - key: kb-data
      operator: Equal
      value: 'true'
      effect: NoSchedule
  componentSpecs:
  - name: postgresql
    componentDefRef: postgresql
    enabledLogs:
    - running
    disableExporter: true # 将参数值设为 `false`，开启 exporter
    replicas: 2
    resources:
      limits:
        cpu: '0.5'
        memory: 0.5Gi
      requests:
        cpu: '0.5'
        memory: 0.5Gi
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
EOF
```

#### 开启已有集群的监控 exporter

如果您的环境中已有集群，可执行以下命令查看监控 exporter 是否开启。

```bash
kubectl get cluster mycluster -o yaml
```

<details>

<summary>输出</summary>

```yaml
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

</details>

将 `disableExporter` 设置为 `false` 或使用隐式设置（使用默认值），即表示监控 exporter 已启用，这是监控功能正常运行的前提条件。如果输出结果显示 `disableExporter: true`，您需要将其修改为 `false`，开启监控 exporter。

请注意更新 `disableExporter` 字段会导致集群中的所有 Pod 重启。

<Tabs>

<TabItem value="kubectl patch" label="kubectl patch" default>

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

</TabItem>

<TabItem value="编辑集群 YAML 文件" label="编辑集群 YAML 文件">

您也可通过编辑 `cluster.yaml` 文件开启监控功能。

```bash
kubectl edit cluster mycluster -n demo
```

修改 `disableExporter` 的参数值。

```yaml
...
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: true # 将参数值设为 `false`，开启 exporter
```

</TabItem>

</Tabs>

集群运行时，每个 Pod 都拥有一个名为 `exporter` 的 sidecar 容器，用于运行 postgres-exporter。

### 创建 PodMonitor

1. 获取 `scrapePath` 及 `scrapePort`。

   您可从 Pod 的 exporter 容器中获取 `scrapePath` 及 `scrapePort`。

   ```bash
   kubectl get po mycluster-postgresql-0 -oyaml | yq '.spec.containers[] | select(.name=="exporter") | .ports '
   ```

   <details>

   <summary>输出</summary>

   ```bash
   - containerPort: 9187
     name: http-metrics
     protocol: TCP
   ```

   </details>

2. 创建 `PodMonitor`。

   应用 `PodMonitor` 文件，监控集群。以下展示了不同引擎的创建示例，您可按需修改相关参数值。

   <Tabs>

   <TabItem value="ApeCloud MySQL" label="ApeCloud MySQL" default>

   您也可以在 [KubeBlocks Addons 仓库](https://github.com/apecloud/kubeblocks-addons/blob/main/examples/apecloud-mysql/pod-monitor.yaml)中查看最新版本示例 YAML 文件。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: mycluster-pod-monitor
     namespace: monitoring # 说明：此处为 Prometheus operator 所在的 namespace
     labels:               # 此处对应 `prometheus.spec.podMonitorSelector` 中设置的标签。
       release: prometheus
   spec:
     jobLabel: kubeblocks-service
     # 定义了从关联的 Kubernetes `Pod` 对象
     # 传递到采集指标上的标签
     # 请按需设置标签
     podTargetLabels:
     - app.kubernetes.io/instance
     - app.kubernetes.io/managed-by
     - apps.kubeblocks.io/component-name
     - apps.kubeblocks.io/pod-name
     podMetricsEndpoints:
       - path: /metrics
         port: http-metrics
         scheme: http
     namespaceSelector:
       matchNames:
         - default
     selector:
       matchLabels:
         app.kubernetes.io/instance: mycluster
         apps.kubeblocks.io/component-name: mysql
   EOF
   ```

   </TabItem>

   <TabItem value="MySQL 社区版" label="MySQL 社区版">

   您也可以在 [KubeBlocks Addons 仓库](https://github.com/apecloud/kubeblocks-addons/blob/main/examples/mysql/pod-monitor.yaml)中查看最新版本示例 YAML 文件。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: mycluster-pod-monitor
     namespace: monitoring # 说明：此处为 Prometheus operator 所在的 namespace
     labels:               # 此处对应 `prometheus.spec.podMonitorSelector` 中设置的标签。
       release: prometheus
   spec:
     jobLabel: kubeblocks-service
     # 定义了从关联的 Kubernetes `Pod` 对象
     # 传递到采集指标上的标签
     # 请按需设置标签
     podTargetLabels:
     - app.kubernetes.io/instance
     - app.kubernetes.io/managed-by
     - apps.kubeblocks.io/component-name
     - apps.kubeblocks.io/pod-name
     podMetricsEndpoints:
       - path: /metrics
         port: http-metrics
         scheme: http
     namespaceSelector:
       matchNames:
         - default
     selector:
       matchLabels:
         app.kubernetes.io/instance: mycluster
         apps.kubeblocks.io/component-name: mysql
   EOF
   ```

   </TabItem>

   <TabItem value="PostgreSQL" label="PostgreSQL">

   您也可以在 [KubeBlocks Addons 仓库](https://github.com/apecloud/kubeblocks-addons/blob/main/examples/postgresql/pod-monitor.yml)中查看最新版本示例 YAML 文件。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: mycluster-pod-monitor
     namespace: monitoring # 说明：此处为 Prometheus operator 所在的 namespace
     labels:               # 此处对应 `prometheus.spec.podMonitorSelector` 中设置的标签。
       release: prometheus
   spec:
     jobLabel: kubeblocks-service
     # 定义了从关联的 Kubernetes `Pod` 对象
     # 传递到采集指标上的标签
     # 请按需设置标签
     podTargetLabels:
     - app.kubernetes.io/instance
     - app.kubernetes.io/managed-by
     - apps.kubeblocks.io/component-name
     - apps.kubeblocks.io/pod-name
     podMetricsEndpoints:
       - path: /metrics
         port: http-metrics
         scheme: http
     namespaceSelector:
       matchNames:
         - default
     selector:
       matchLabels:
         app.kubernetes.io/instance: mycluster
         apps.kubeblocks.io/component-name: postgresql
   EOF
   ```

   </TabItem>

   <TabItem value="Redis" label="Redis">

   您也可以在 [KubeBlocks Addons 仓库](https://github.com/apecloud/kubeblocks-addons/blob/main/examples/redis/pod-monitor.yaml)中查看最新版本示例 YAML 文件。

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: PodMonitor
   metadata:
     name: mycluster-pod-monitor
     namespace: monitoring # 说明：此处为 Prometheus operator 所在的 namespace
     labels:               # 此处对应 `prometheus.spec.podMonitorSelector` 中设置的标签。
       release: prometheus
   spec:
     jobLabel: kubeblocks-service
     # 定义了从关联的 Kubernetes `Pod` 对象
     # 传递到采集指标上的标签
     # 请按需设置标签
     podTargetLabels:
     - app.kubernetes.io/instance
     - app.kubernetes.io/managed-by
     - apps.kubeblocks.io/component-name
     - apps.kubeblocks.io/pod-name
     podMetricsEndpoints:
       - path: /metrics
         port: http-metrics
         scheme: http
     namespaceSelector:
       matchNames:
         - default
     selector:
       matchLabels:
         app.kubernetes.io/instance: mycluster
         apps.kubeblocks.io/component-name: redis
   EOF
   ```

   </TabItem>

   </Tabs>

3. 连接 Grafana 大盘.

    登录 Grafana 大盘，并导入大盘。

    Grafana 大盘的 `APPS / PostgreSQL` 文件夹下有预设的大盘模板。您也可以在 [Grafana 大盘商店](https://grafana.com/grafana/dashboards/)获取更多大盘模板。

:::note

请确保 `PodMonitor` 文件中的标签（如 endpoint 中的 path 和 port 值）设置正确，与您使用的大盘匹配。

:::
