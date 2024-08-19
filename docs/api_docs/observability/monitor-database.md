---
title: Monitor database
description: How to monitor your database
keywords: [monitor database, monitor a cluster, monitor]
sidebar_position: 1
---

# Monitor a database

With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows how database monitoring tools work with KubeBlocks and how to use the monitoring function.

## For the test/demo environment

### Enable monitoring addons

KubeBlocks integrates open-source monitoring components, such as Prometheus, AlertManager, and Grafana, by addons and adopts the custom `apecloud-otel-collector` to collect the monitoring indicators of databases and host machines. You can use these addons for the test or demo environment.

* `prometheus`: it includes Prometheus and AlertManager addons.
* `grafana`: it includes Grafana monitoring addons.
* `victoria-metrics`: it collects metrics from various sources and stores them to VictoriaMetrics.
* `victoria-metrics-agent`: it collects metrics from various sources, relabel and filter the collected metrics and store them in VictoriaMetrics or any other storage systems via Prometheus `remote_write` protocol or via VictoriaMetrics `remote_write` protocol.
* `alertmanager-webhook-adaptor`: it includes the notification extension and is used to extend the notification capability of AlertManager. Currently, the custom bots of Feishu, DingTalk, and Wechat Enterprise are supported.
* `apecloud-otel-collector`: it is used to collect the indicators of databases and the host machine.

***Steps:***

:::note

Here is an example of enabling the `prometheus` addon. You can enable other monitoring addons by replacing `prometheus` in the example with the name of other addons.

:::

1. (Optional) Add the KubeBlocks repo. If you install KubeBlocks with Helm, just run `helm repo update`.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

2. View the addon versions.

   ```bash
   helm search repo kubeblocks/prometheus --devel --versions
   ```

3. Install the addon.

   ```bash
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   ```

4. Verify whether this addon is installed.

   The STATUS is deployed and this addon is installed successfully.

   ```bash
   helm list -A
   >
   NAME         NAMESPACE   REVISION    UPDATED                                 STATUS      CHART                APP VERSION
   ......
   prometheus   kb-system   1           2024-05-31 12:01:52.872584 +0800 CST    deployed    prometheus-15.16.1   2.39.1 
   ```

### Enable the monitoring function for a database

The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by the Prometheus server automatically and scrape monitoring indicators at regular intervals.

If you disable the monitoring function when creating a cluster, run the command below to enable it.

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

If you want to disable the monitoring function, run the command below to disable it.

```bash
kubectl patch cluster mycluster -n namespace --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":true}]'
```

You can also edit the `cluster.yaml` to enable/disable the monitoring function.

```bash
kubectl edit cluster mycluster -n demo
......
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: false # Change this value
```

### View the dashboard

Use the `grafana` addon provided by KubeBlocks to view the dashboard.

1. Get the username and password of the `grafana` addon.

   ```bash
   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-user}' |base64 -d

   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-password}' |base64 -d
   ```

2. Run the command below to connect to the Grafana dashboard.

   ```bash
   kubectl port-forward svc/grafana -n kb-system 3000:80
   >
   Forwarding from 127.0.0.1:3000 -> 3000
   Forwarding from [::1]:3000 -> 3000
   Handling connection for 3000
   ```

3. Open the web browser and enter the address `127.0.0.1:3000` to visit the dashboard.
4. Enter the username and password obtained from step 1.

:::note

If there is no data in the dashboard, you can check whether the job is `kubeblocks-service`. Enter `kubeblocks-service` in the job field and press the enter button.

![monitoring](./../../img/api-monitoring.png)

:::

### (Optional) Enable remote write

KubeBlocks supports the `victoria-metrics-agent` addon to enable you to remotely write the data to your VM. Compared with the native Prometheus, [vmagent](https://docs.victoriametrics.com/vmagent.html) is lighter.

Install the `victoria-metrics-agent` addon.

```bash
helm install  vm kubeblocks/victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
```

For detailed settings, you can refer to [Victoria Metrics docs](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-agent).

## For the production environment

### Integrate monitoring tools

For the production environment, it is highly recommended to build your monitoring system or purchase a third-party monitoring service. You can follow the docs of monitoring tools to integrate the tools with KubeBlocks.

### Enable the monitoring function for a database

The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by the Prometheus server automatically and scrape monitoring indicators at regular intervals.

If you disable the monitoring function when creating a cluster, run the command below to enable it.

```bash
kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
```

If you want to disable the monitoring function, run the command below to disable it.

```bash
kubectl patch cluster mycluster -n namespace --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":true}]'
```

You can also edit the `cluster.yaml` to enable/disable the monitoring function.

```bash
kubectl edit cluster mycluster -n demo
......
componentSpecs:
  - name: mysql
    componentDefRef: mysql
    enabledLogs:
    - error
    - general
    - slow
    disableExporter: true # Change this value
```

### View the dashboard

You can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see the [Grafana dashboard documentation](https://grafana.com/docs/grafana/latest/dashboards/).

### (Optional) Enable remote write

KubeBlocks supports the `victoria-metrics-agent` addon to enable you to remotely write the data to your VM. Compared with the native Prometheus, [vmgent](https://docs.victoriametrics.com/vmagent.html) is lighter and supports the horizontal extension.

Install the `victoria-metrics-agent` addon.

```bash
helm install vm kubeblocks/victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
```

For detailed settings, you can refer to [Victoria Metrics docs](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-agent).
