---
title: Monitor database
description: How to monitor your database
keywords: [monitor database, monitor a cluster, monitor]
sidebar_position: 1
---

# Observability of KubeBlocks

With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows you how database monitoring tools work with KubeBlocks and how to use the function.

## For Playground

KubeBlocks integrates open-source monitoring components, such as Prometheus, AlertManager, and Grafana, by add-ons and adopts the custom `apecloud-otel-collector` to collect the monitoring indicators of databases and host machines. All monitoring add-ons are enabled when KubeBlocks Playground is deployed.

KubeBlock Playground supports the following built-in monitoring add-ons:

* `prometheus`: it includes Prometheus and AlertManager add-ons.
* `grafana`: it includes Grafana monitoring add-ons.
* `alertmanager-webhook-adaptor`: it includes the notification extension add-on and is used to extend the notification capability of AlertManager. Currently, the custom bots of Feishu, DingTalk, and Wechat Enterprise are supported.
* `apecloud-otel-collecotr`: it is used to collect the indicators of databases and the host machine.

1. View all built-in add-ons and make sure the monitoring add-ons are enabled.

   ```bash
   # View all add-ons supported
   kbcli addon list
   ...
   grafana                        Helm   Enabled                   true                                                                                    
   alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
   prometheus                     Helm   Enabled    alertmanager   true 
   ...
   ```

2. View the dashboard list.

   ```bash
   kbcli dashboard list
   >
   NAME                                 NAMESPACE   PORT    CREATED-TIME
   kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
   ```

3. Open the web console of a monitoring dashboard. For example,

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

## For production environment

In the production environment, all monitoring add-ons are disabled by default when installing KubeBlocks. You can enable these add-ons but it is highly recommended to build your monitoring system or purchase a third-party monitoring service.

### Enable monitoring function by kbcli

:::caution

It is not recommended to use this function in the production environment.

:::

1. View all built-in add-ons.

   ```bash
   kbcli addon list
   ```

2. Enable the monitoring add-ons.

   ```bash
   kbcli addon enable prometheus
   kbcli addon enable grafana
   kbcli addon enable alertmanager-webhook-adaptor
   kbcli addon enable apecloud-otel-collector
   ```

3. View the dashboard list.

   ```bash
   kbcli dashboard list
   >
   NAME                                 NAMESPACE   PORT    CREATED-TIME
   kubeblocks-grafana                   kb-system   13000   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-alertmanager   kb-system   19093   Jul 24,2023 11:38 UTC+0800
   kubeblocks-prometheus-server         kb-system   19090   Jul 24,2023 11:38 UTC+0800
   ```

4. Open the web console of a monitoring dashboard. For example,

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

### Enable monitoring function by integration

KubeBlocks provides an add-on, `victoria-metrics-agent`, to push the monitoring data to a third-party monitoring system compatible with the Prometheus Remote Write protocol. Compared with the native Prometheus, [vmgent](https://docs.victoriametrics.com/vmagent.html) is lighter and supports the horizontal extension.

***Steps:***

1. Enable data push.

   You just need to provide the endpoint which supports the Prometheus Remote Write protocol and multiple endpoints can be supported. Refer to the tutorials of your third-party monitoring system for how to get an endpoint.

   The following examples show how to enable data push by different options.

   ```bash
   # The default option. You only need to provide an endpoint with no verification.
   # Endpoint example: http://localhost:8428/api/v1/write
   kbcli addon enable victoria-metrics-agent --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
   ```

   ```bash
   # Basic Auth
   kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite\.basicAuth\.username=<your username>,remoteWrite\.basicAuth\.password=<your password>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

   ```bash
   # TLS
   kbcli addon enable victoria-metrics-agent --set "extraArgs.tls=true,extraArgs.tlsCertFile=<path to certifle>,extraArgs.tlsKeyFile=<path to keyfile>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

   ```bash
   # AWS SigV4
   kbcli addon enable victoria-metrics-agent --set "extraArgs.remoteWrite\.aws\.region=<your AMP region>,extraArgs.remoteWrite\.aws\.accessKey=<your accessKey>,extraArgs.remoteWrite\.aws\.secretKey=<your secretKey>,remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}"
   ```

2. (Optional) Horizontally scale the `victoria-metrics-agent` add-on.

   When the amount of database instances continues to increase, a single-node vmagent becomes the bottleneck. This problem can be solved by scaling vmagent. The multiple-node vmagent automatically divides the task of data collection according to the Hash strategy.

   ```bash
   kbcli addon enable victoria-metrics-agent --replicas <replica count> --set remoteWriteUrls={http://<remoteWriteUrl>:<port>/<remote write path>}
   ```

3. (Optional) Disable the `victoria-metrics-agent` add-on.

   ```bash
   kbcli addon disable victoria-metrics-agent
   ```

### View the web console by kbcli

### View the web console by Integration

Kubeblocks provides Grafana Dashboards and Prometheus AlertRules for mainstream engines, which you can obtain from [the repository](https://github.com/apecloud/kubeblocks-mixin), or convert and customize according to your needs.

For the importing method, refer to the tutorials of your third-party monitoring service.

### Enable the monitoring function for a database

The monitoring function is enabled by default when a database is created. The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by Prometheus server automatically and scrape monitoring indicators at regular intervals.

* For a new cluster, run the command below to create a database cluster.

    ```bash
    # Search the cluster definition
    kbcli clusterdefinition list 

    # Create a cluster
    kbcli cluster create mysql <clustername> 
    ```

    ***Example***

    ```bash
    # View all add-ons supported
    kbcli addon list
    ...
    grafana                        Helm   Enabled                   true                                                                                    
    alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
    prometheus                     Helm   Enabled    alertmanager   true 
    ...
    # Enable prometheus add-on
    kbcli addon enable prometheus

    # Enable granfana add-on
    kbcli addon enable grafana

    # Enable alertmanager-webhook-adaptor add-on
    kbcli addon enable alertmanager-webhook-adaptor
    ```

:::note

The setting of `monitor` is `true` by default and it is not recommended to disable it. In the cluster definition, you can choose any supported database engine, such as PostgreSQL, MongoDB, and Redis.

```bash
kbcli cluster create mysql --monitor=true mysql-cluster
```

:::

* For the existing cluster, you can update it to enable the monitor function with the `update` command.

    ```bash
    kbcli cluster update <name> --monitor=true
    ```

    ***Example***

    ```bash
    kbcli cluster update mysql-cluster --monitor=true
    ```

You can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see the [Grafana dashboard documentation](https://grafana.com/docs/grafana/latest/dashboards/).
