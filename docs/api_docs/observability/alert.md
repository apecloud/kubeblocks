---
title: Configure alert
description: How to enable alert
keywords: [alert, alert message, email alert]
sidebar_position: 2
---

# Configure alert

Alerts are mainly used for daily error response to improve system availability. KubeBlocks uses the open-source version of Prometheus to configure alert rules and multiple notification channels. The alert capability of KubeBlocks can meet the operation and maintenance requirements of production-level online clusters.

:::note

The alert function is the same for all.

:::

## Alert rules

KubeBlocks uses the open-source version of Prometheus to meet the needs of various data products. These alert rules provide the best practice for cluster operation and maintenance, which further improve alert accuracy and reduce the probability of false negatives and false positives by experience-based smoothing windows, alert thresholds, alert levels, and alert indicators.

Taking PostgreSQL as an example, the alert rules have built-in common abnormal events, such as instance down, instance restart, slow query, connection amount, deadlock, and cache hit rate.

The following example shows PostgreSQL alert rules (refer to [Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/) for syntax). When the amount of active connections exceeds 80% of the threshold and lasts for 2 minutes, Prometheus triggers a warning alert and sends it to AlertManager.

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

Configure alert rules as needed. For more details, please refer to [Prometheus alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules).

## Notifications

The alert message notification of KubeBlocks mainly adopts the AlertManager native capability. After receiving the Prometheus alert, KubeBlocks performs steps including deduplication, grouping, silence, suppression, and routing, and finally sends it to the corresponding notification channel.

Configure notification channels as needed. For more details, please refer to [Prometheus alerting configuration](https://prometheus.io/docs/alerting/latest/configuration/).