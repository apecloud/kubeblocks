---
title: COnfigure IM alert
description: How to enable IM alert
sidebar_position: 2
---

# Configure IM alert

Alerts are mainly used for daily error response to improve system availability. Kubeblocks has a built-in set of common alert rules and integrates multiple notification channels. The alert capability of Kubeblocks can meet the operation and maintenance requirements of production-level online clusters.

## How KubeBlocks alert works

The built-in alert system of Kubeblocks adopts the mainstream open-source solution in the cloud native scenario, i.e. the combined solution of Prometheus and AlertManager. KubeBlocks also uses the AlertManager Webhook extension to integrate new notification channels, such as Feishu custom bot, Dingtalk custom bot, Wechat custom bot.

![Alert](../../../img/observability_alert.png)

## Alert rules

KubeBlocks has a set of general built-in alter rules to meet the alert needs of each data product and provides an out-of-the-box experience without further configurations. These alert rules provide the best practice for cluster operation and maintenance. These alarm rules further improve the alert accuracy and reduce the probability of false negatives and false positives through experience-based smoothing windows, alarm thresholds, alarm levels, and alarm indicators.

Taking PostgreSQL as an example, the alert rules have built-in common abnormal events, such as instance down, instance restart, slow query, connection amount, deadlock, and cache hit rate. 
The following example shows PostgreSQL alert rule (refer to [Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/) for syntax). When the amount of active connections exceeds 80% of the threshold and lasts for 2 minutes, Prometheus triggers a warning and sends it to AlertManager.

```bash
alert: PostgreSQLTooManyConnections
  expr: |
    sum by (namespace,app_kubernetes_io_instance,pod) (pg_stat_activity_count{datname!~"template.*|postgres"})
    > on(namespace,app_kubernetes_io_instance,pod)
    (pg_settings_max_connections - pg_settings_superuser_reserved_connections) * 0.8
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "PostgreSQL too many connections (> 80%)"
    description: "PostgreSQL has too many connections and the value is {{ $value }}. (instance: {{ $labels.pod }})"
```

You can view all the built-in alert rules in **Alerts Tab** of **Prometheus Dashboards**. Run the commands below to open Prometheus Dashboards.

```bash
# View dashboards list
kbcli dashboard list

# Open Prometheus Dashboards
kbcli dashboard open kubeblocks-prometheus-server # Here is an example and fill in the actual name based on the above dashboard list
```

## Configure IM alert

The alert message notification of Kubeblocks mainly adopts the AlertManager native capability. After receiving the Prometheus alarm, KubeBlocks performs multiple steps such as deduplication, grouping, silence, suppression, and routing, and finally sends it to the corresponding notification channel.
AlertManager integrates a set of notification channels, such as Email and Slack. Kubeblocks extends new IM class notification channels with AlertManger Webhook.

### Step 1. Configure alert channel

You need to configure the notification channels in advance based on your needs and obtain the necessary information for the following steps. 
Taking Feishu as an example, you can obtain the webhook address after creating a custom robot. If the signature verification in the security configuration is enabled, you can obtain the signature key in advance.

Currently, Feishu custom bot, DingTalk custom bot, WeChat Enterprise custom bot, and Slack are supported. You can refer to the following guides to configure the channel.

* [Feishu custom bot](https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN)
* [DingTalk custom bot](https://open.dingtalk.com/document/orgapp/custom-robot-access)
* [WeChat Enterprise custom bot](https://developer.work.weixin.qq.com/document/path/91770)
* [Slack](https://api.slack.com/messaging/webhooks)



### Step 2. Configure the receiver



## Troubleshooting

If you cannot receive alert notices, run the commands below to troubleshoot the logs of AlertManager and AlertManager-Webhook-Adaptor. 

```bash
# Find the corresponding Pod of AlertManager and get Pod name
kubectl get pods -l 'release=kubeblocks,app=prometheus,component=alertmanager'

# Search AlertManeger logs
kubectl logs <pod-name> -c prometheus-alertmanager

# Find the corresponding Pod of AlertManager-Webhook-Adaptor and get Pod name
kubectl get pods -l 'app.kubernetes.io/instance=kubeblocks,app.kubernetes.io/name=alertmanager-webhook-adaptor'

# Search AlertManager-Webhook-Adaptor logs
kubectl logs <pod-name> -c alertmanager-webhook-adaptor
```