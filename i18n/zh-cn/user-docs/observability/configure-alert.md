---
title: 配置告警
description: 如何配置告警
keywords: [mysql, 告警, 告警信息, 邮件告警]
sidebar_position: 2
---

# 配置告警

告警主要用于日常故障响应，以提高系统可用性。KubeBlocks 使用开源版本的 Prometheus 配置警报规则和多个通知渠道。KubeBlocks 的告警功能能够满足生产级在线集群的运维需求。

:::note

所有数据库的告警功能相同。

:::

## 告警规则

KubeBlocks 使用开源版本的 Prometheus 来满足各种数据产品的需求。这些警报规则提供了集群运维的最佳实践，通过经验总结的平滑窗口、告警阈值、告警级别和告警指标，进一步提高了告警的准确性，降低了误报率和漏报率。

以 PostgreSQL 为例，告警规则中内置了常见的异常事件，例如实例宕机、实例重启、慢查询、连接数、死锁和缓存命中率等。

以下示例显示了 PostgreSQL 的告警规则（可参考 [Prometheus 的语法](https://prometheus.io/docs/prometheus/latest/querying/basics/)）。当活跃连接数超过阈值的 80% 并持续 2 分钟时，Prometheus 会触发 warning 警告，并将其发送到 AlertManager。

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

您可按需配置告警规则，详情可参考 [Prometheus 告警规则文档](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#defining-alerting-rules)。

## 通知

KubeBlocks 的告警通知主要采用 AlertManager 的原生能力。在接收到 Prometheus 告警后，KubeBlocks 执行去重、分组、静默、抑制和路由等步骤，最终将告警发送到相应的通知渠道。

可按需配置通知渠道，具体可参考 [Prometheus 告警配置](https://prometheus.io/docs/alerting/latest/configuration/)。
